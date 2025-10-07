package compaction

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"code.tczkiot.com/srdb/manifest"
)

// Manager 管理 Compaction 流程
type Manager struct {
	compactor  *Compactor
	versionSet *manifest.VersionSet
	sstDir     string

	// 控制后台 Compaction
	stopCh chan struct{}
	wg     sync.WaitGroup

	// Compaction 并发控制
	compactionMu sync.Mutex // 防止并发执行 compaction

	// 统计信息
	mu                 sync.RWMutex
	totalCompactions   int64
	lastCompactionTime time.Time
	lastFailedFile     int64 // 最后失败的文件编号
	consecutiveFails   int   // 连续失败次数
	lastGCTime         time.Time
	totalOrphansFound  int64
}

// NewManager 创建新的 Compaction Manager
func NewManager(sstDir string, versionSet *manifest.VersionSet) *Manager {
	return &Manager{
		compactor:  NewCompactor(sstDir, versionSet),
		versionSet: versionSet,
		sstDir:     sstDir,
		stopCh:     make(chan struct{}),
	}
}

// GetPicker 获取 Compaction Picker
func (m *Manager) GetPicker() *Picker {
	return m.compactor.GetPicker()
}

// Start 启动后台 Compaction 和垃圾回收
func (m *Manager) Start() {
	m.wg.Add(2)
	go m.backgroundCompaction()
	go m.backgroundGarbageCollection()
}

// Stop 停止后台 Compaction
func (m *Manager) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

// backgroundCompaction 后台 Compaction 循环
func (m *Manager) backgroundCompaction() {
	defer m.wg.Done()

	ticker := time.NewTicker(10 * time.Second) // 每 10 秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.maybeCompact()
		}
	}
}

// MaybeCompact 检查是否需要 Compaction 并执行（公开方法，供外部调用）
// 非阻塞：如果已有 compaction 在执行，直接返回
func (m *Manager) MaybeCompact() {
	// 尝试获取锁，如果已有 compaction 在执行，直接返回
	if !m.compactionMu.TryLock() {
		return
	}
	defer m.compactionMu.Unlock()

	m.doCompact()
}

// maybeCompact 内部使用的阻塞版本（后台 goroutine 使用）
func (m *Manager) maybeCompact() {
	m.compactionMu.Lock()
	defer m.compactionMu.Unlock()

	m.doCompact()
}

// doCompact 实际执行 compaction 的逻辑（必须在持有 compactionMu 时调用）
// 支持并发执行多个层级的 compaction
func (m *Manager) doCompact() {
	// 获取当前版本
	version := m.versionSet.GetCurrent()
	if version == nil {
		return
	}

	// 获取所有需要 Compaction 的任务（已按优先级排序）
	picker := m.compactor.GetPicker()
	tasks := picker.PickCompaction(version)
	if len(tasks) == 0 {
		// 输出诊断信息
		m.printCompactionStats(version, picker)
		return
	}

	fmt.Printf("[Compaction] Found %d tasks to execute\n", len(tasks))

	// 并发执行所有任务
	successCount := 0
	for _, task := range tasks {
		// 检查是否是上次失败的文件（防止无限重试）
		if len(task.InputFiles) > 0 {
			firstFile := task.InputFiles[0].FileNumber
			m.mu.Lock()
			if m.lastFailedFile == firstFile && m.consecutiveFails >= 3 {
				fmt.Printf("[Compaction] Skipping L%d file %d (failed %d times)\n",
					task.Level, firstFile, m.consecutiveFails)
				m.consecutiveFails = 0
				m.lastFailedFile = 0
				m.mu.Unlock()
				continue
			}
			m.mu.Unlock()
		}

		// 获取最新版本（每个任务执行前）
		currentVersion := m.versionSet.GetCurrent()
		if currentVersion == nil {
			continue
		}

		// 执行 Compaction
		fmt.Printf("[Compaction] Starting: L%d -> L%d, files: %d\n",
			task.Level, task.OutputLevel, len(task.InputFiles))

		err := m.DoCompactionWithVersion(task, currentVersion)
		if err != nil {
			fmt.Printf("[Compaction] Failed L%d -> L%d: %v\n", task.Level, task.OutputLevel, err)

			// 记录失败信息
			if len(task.InputFiles) > 0 {
				firstFile := task.InputFiles[0].FileNumber
				m.mu.Lock()
				if m.lastFailedFile == firstFile {
					m.consecutiveFails++
				} else {
					m.lastFailedFile = firstFile
					m.consecutiveFails = 1
				}
				m.mu.Unlock()
			}
		} else {
			fmt.Printf("[Compaction] Completed: L%d -> L%d\n", task.Level, task.OutputLevel)
			successCount++

			// 清除失败计数
			m.mu.Lock()
			m.consecutiveFails = 0
			m.lastFailedFile = 0
			m.mu.Unlock()
		}
	}

	fmt.Printf("[Compaction] Batch completed: %d/%d tasks succeeded\n", successCount, len(tasks))
}

// printCompactionStats 输出 Compaction 统计信息（每分钟一次）
func (m *Manager) printCompactionStats(version *manifest.Version, picker *Picker) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 限制输出频率：每 60 秒输出一次
	if time.Since(m.lastCompactionTime) < 60*time.Second {
		return
	}
	m.lastCompactionTime = time.Now()

	fmt.Println("[Compaction] Status check:")
	for level := 0; level < 7; level++ {
		files := version.GetLevel(level)
		if len(files) == 0 {
			continue
		}

		totalSize := int64(0)
		for _, f := range files {
			totalSize += f.FileSize
		}

		score := picker.GetLevelScore(version, level)
		fmt.Printf("  L%d: %d files, %.2f MB, score: %.2f\n",
			level, len(files), float64(totalSize)/(1024*1024), score)
	}
}

// DoCompactionWithVersion 使用指定的版本执行 Compaction
func (m *Manager) DoCompactionWithVersion(task *CompactionTask, version *manifest.Version) error {
	if version == nil {
		return fmt.Errorf("version is nil")
	}

	// 执行 Compaction（使用传入的 version，而不是重新获取）
	edit, err := m.compactor.DoCompaction(task, version)
	if err != nil {
		return fmt.Errorf("compaction failed: %w", err)
	}

	// 如果 edit 为 nil，说明所有文件都已经不存在，无需应用变更
	if edit == nil {
		fmt.Printf("[Compaction] No changes needed (files already removed)\n")
		return nil
	}

	// 应用 VersionEdit
	err = m.versionSet.LogAndApply(edit)
	if err != nil {
		// LogAndApply 失败，清理已写入的新 SST 文件（防止孤儿文件）
		fmt.Printf("[Compaction] LogAndApply failed, cleaning up new files: %v\n", err)
		m.cleanupNewFiles(edit)
		return fmt.Errorf("apply version edit: %w", err)
	}

	// LogAndApply 成功后，删除废弃的 SST 文件
	m.deleteObsoleteFiles(edit)

	// 更新统计信息
	m.mu.Lock()
	m.totalCompactions++
	m.lastCompactionTime = time.Now()
	m.mu.Unlock()

	return nil
}

// DoCompaction 执行一次 Compaction（兼容旧接口）
func (m *Manager) DoCompaction(task *CompactionTask) error {
	// 获取当前版本
	version := m.versionSet.GetCurrent()
	if version == nil {
		return fmt.Errorf("no current version")
	}

	return m.DoCompactionWithVersion(task, version)
}

// cleanupNewFiles 清理 LogAndApply 失败后的新文件（防止孤儿文件）
func (m *Manager) cleanupNewFiles(edit *manifest.VersionEdit) {
	if edit == nil {
		return
	}

	fmt.Printf("[Compaction] Cleaning up %d new files after LogAndApply failure\n", len(edit.AddedFiles))

	// 删除新创建的文件
	for _, file := range edit.AddedFiles {
		sstPath := filepath.Join(m.sstDir, fmt.Sprintf("%06d.sst", file.FileNumber))
		err := os.Remove(sstPath)
		if err != nil {
			fmt.Printf("[Compaction] Failed to cleanup new file %06d.sst: %v\n", file.FileNumber, err)
		} else {
			fmt.Printf("[Compaction] Cleaned up new file %06d.sst\n", file.FileNumber)
		}
	}
}

// deleteObsoleteFiles 删除废弃的 SST 文件
func (m *Manager) deleteObsoleteFiles(edit *manifest.VersionEdit) {
	if edit == nil {
		fmt.Printf("[Compaction] deleteObsoleteFiles: edit is nil\n")
		return
	}

	fmt.Printf("[Compaction] deleteObsoleteFiles: %d files to delete\n", len(edit.DeletedFiles))

	// 删除被标记为删除的文件
	for _, fileNum := range edit.DeletedFiles {
		sstPath := filepath.Join(m.sstDir, fmt.Sprintf("%06d.sst", fileNum))
		err := os.Remove(sstPath)
		if err != nil {
			// 删除失败只记录日志，不影响 compaction 流程
			// 后台垃圾回收器会重试
			fmt.Printf("[Compaction] Failed to delete obsolete file %06d.sst: %v\n", fileNum, err)
		} else {
			fmt.Printf("[Compaction] Deleted obsolete file %06d.sst\n", fileNum)
		}
	}
}

// TriggerCompaction 手动触发一次 Compaction（所有需要的层级）
func (m *Manager) TriggerCompaction() error {
	version := m.versionSet.GetCurrent()
	if version == nil {
		return fmt.Errorf("no current version")
	}

	picker := m.compactor.GetPicker()
	tasks := picker.PickCompaction(version)
	if len(tasks) == 0 {
		return nil // 不需要 Compaction
	}

	// 依次执行所有任务
	for _, task := range tasks {
		currentVersion := m.versionSet.GetCurrent()
		if err := m.DoCompactionWithVersion(task, currentVersion); err != nil {
			return err
		}
	}

	return nil
}

// GetStats 获取 Compaction 统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_compactions":    m.totalCompactions,
		"last_compaction_time": m.lastCompactionTime,
	}
}

// GetLevelStats 获取每层的统计信息
func (m *Manager) GetLevelStats() []map[string]interface{} {
	version := m.versionSet.GetCurrent()
	if version == nil {
		return nil
	}

	picker := m.compactor.GetPicker()
	stats := make([]map[string]interface{}, manifest.NumLevels)

	for level := 0; level < manifest.NumLevels; level++ {
		files := version.GetLevel(level)
		totalSize := int64(0)
		for _, file := range files {
			totalSize += file.FileSize
		}

		stats[level] = map[string]interface{}{
			"level":      level,
			"file_count": len(files),
			"total_size": totalSize,
			"score":      picker.GetLevelScore(version, level),
		}
	}

	return stats
}

// backgroundGarbageCollection 后台垃圾回收循环
func (m *Manager) backgroundGarbageCollection() {
	defer m.wg.Done()

	ticker := time.NewTicker(5 * time.Minute) // 每 5 分钟检查一次
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.collectOrphanFiles()
		}
	}
}

// collectOrphanFiles 收集并删除孤儿 SST 文件
func (m *Manager) collectOrphanFiles() {
	// 1. 获取当前版本中的所有活跃文件
	version := m.versionSet.GetCurrent()
	if version == nil {
		return
	}

	activeFiles := make(map[int64]bool)
	for level := 0; level < manifest.NumLevels; level++ {
		files := version.GetLevel(level)
		for _, file := range files {
			activeFiles[file.FileNumber] = true
		}
	}

	// 2. 扫描 SST 目录中的所有文件
	pattern := filepath.Join(m.sstDir, "*.sst")
	sstFiles, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Printf("[GC] Failed to scan SST directory: %v\n", err)
		return
	}

	// 3. 找出孤儿文件并删除
	orphanCount := 0
	for _, sstPath := range sstFiles {
		// 提取文件编号
		var fileNum int64
		_, err := fmt.Sscanf(filepath.Base(sstPath), "%d.sst", &fileNum)
		if err != nil {
			continue
		}

		// 检查是否是活跃文件
		if !activeFiles[fileNum] {
			// 这是孤儿文件，删除它
			err := os.Remove(sstPath)
			if err != nil {
				fmt.Printf("[GC] Failed to delete orphan file %06d.sst: %v\n", fileNum, err)
			} else {
				fmt.Printf("[GC] Deleted orphan file %06d.sst\n", fileNum)
				orphanCount++
			}
		}
	}

	// 4. 更新统计信息
	m.mu.Lock()
	m.lastGCTime = time.Now()
	m.totalOrphansFound += int64(orphanCount)
	m.mu.Unlock()

	if orphanCount > 0 {
		fmt.Printf("[GC] Completed: cleaned up %d orphan files (total: %d)\n", orphanCount, m.totalOrphansFound)
	}
}

// CleanupOrphanFiles 手动触发孤儿文件清理（可在启动时调用）
func (m *Manager) CleanupOrphanFiles() {
	fmt.Println("[GC] Manual cleanup triggered")
	m.collectOrphanFiles()
}
