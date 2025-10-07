package compaction

import (
	"fmt"

	"code.tczkiot.com/srdb/manifest"
)

// CompactionTask 表示一个 Compaction 任务
type CompactionTask struct {
	Level       int                      // 源层级
	InputFiles  []*manifest.FileMetadata // 需要合并的输入文件
	OutputLevel int                      // 输出层级
}

// Picker 负责选择需要 Compaction 的文件
type Picker struct {
	// Level 大小限制 (字节)
	levelSizeLimits [manifest.NumLevels]int64

	// Level 文件数量限制
	levelFileLimits [manifest.NumLevels]int
}

// NewPicker 创建新的 Compaction Picker
func NewPicker() *Picker {
	p := &Picker{}

	// 设置每层的大小限制 (指数增长)
	// L0: 10MB, L1: 100MB, L2: 1GB, L3: 10GB, L4: 100GB, L5: 1TB, L6: 无限制
	p.levelSizeLimits[0] = 10 * 1024 * 1024          // 10MB
	p.levelSizeLimits[1] = 100 * 1024 * 1024         // 100MB
	p.levelSizeLimits[2] = 1024 * 1024 * 1024        // 1GB
	p.levelSizeLimits[3] = 10 * 1024 * 1024 * 1024   // 10GB
	p.levelSizeLimits[4] = 100 * 1024 * 1024 * 1024  // 100GB
	p.levelSizeLimits[5] = 1024 * 1024 * 1024 * 1024 // 1TB
	p.levelSizeLimits[6] = 0                         // 无限制

	// 设置每层的文件数量限制
	// L0 特殊处理：文件数量限制为 4 (当有4个或更多文件时触发 compaction)
	p.levelFileLimits[0] = 4
	// L1-L6: 不限制文件数量，只限制总大小
	for i := 1; i < manifest.NumLevels; i++ {
		p.levelFileLimits[i] = 0 // 0 表示不限制
	}

	return p
}

// PickCompaction 选择需要 Compaction 的任务（支持多任务并发）
// 返回空切片表示当前不需要 Compaction
func (p *Picker) PickCompaction(version *manifest.Version) []*CompactionTask {
	tasks := make([]*CompactionTask, 0)

	// 1. 检查 L0 (基于文件数量)
	if task := p.pickL0Compaction(version); task != nil {
		tasks = append(tasks, task)
	}

	// 2. 检查 L1-L5 (基于大小)
	for level := 1; level < manifest.NumLevels-1; level++ {
		if task := p.pickLevelCompaction(version, level); task != nil {
			tasks = append(tasks, task)
		}
	}

	// 3. 按优先级排序（score 越高越优先）
	if len(tasks) > 1 {
		p.sortTasksByPriority(tasks, version)
	}

	return tasks
}

// sortTasksByPriority 按优先级对任务排序（score 从高到低）
func (p *Picker) sortTasksByPriority(tasks []*CompactionTask, version *manifest.Version) {
	// 简单的冒泡排序（任务数量通常很少，< 7）
	for i := 0; i < len(tasks)-1; i++ {
		for j := i + 1; j < len(tasks); j++ {
			scoreI := p.GetLevelScore(version, tasks[i].Level)
			scoreJ := p.GetLevelScore(version, tasks[j].Level)
			if scoreJ > scoreI {
				tasks[i], tasks[j] = tasks[j], tasks[i]
			}
		}
	}
}

// pickL0Compaction 选择 L0 的 Compaction 任务
// L0 特殊：文件可能有重叠的 key range，需要全部合并
func (p *Picker) pickL0Compaction(version *manifest.Version) *CompactionTask {
	l0Files := version.GetLevel(0)
	if len(l0Files) == 0 {
		return nil
	}

	// 计算 L0 总大小
	totalSize := int64(0)
	for _, file := range l0Files {
		totalSize += file.FileSize
	}

	// 检查是否需要 Compaction（同时考虑文件数量和总大小）
	// 1. 文件数量超过限制（避免读放大：每次读取需要检查太多文件）
	// 2. 总大小超过限制（避免 L0 占用过多空间）
	needCompaction := false
	if p.levelFileLimits[0] > 0 && len(l0Files) >= p.levelFileLimits[0] {
		needCompaction = true
	}
	if p.levelSizeLimits[0] > 0 && totalSize >= p.levelSizeLimits[0] {
		needCompaction = true
	}

	if !needCompaction {
		return nil
	}

	// L0 → L1 Compaction
	// 选择所有 L0 文件（因为 key range 可能重叠）
	return &CompactionTask{
		Level:       0,
		InputFiles:  l0Files,
		OutputLevel: 1,
	}
}

// pickLevelCompaction 选择 L1-L5 的 Compaction 任务
// L1+ 的文件 key range 不重叠，可以选择多个不重叠的文件
func (p *Picker) pickLevelCompaction(version *manifest.Version, level int) *CompactionTask {
	if level < 1 || level >= manifest.NumLevels-1 {
		return nil
	}

	files := version.GetLevel(level)
	if len(files) == 0 {
		return nil
	}

	// 计算当前层级的总大小
	totalSize := int64(0)
	for _, file := range files {
		totalSize += file.FileSize
	}

	// 检查是否超过大小限制
	if totalSize < p.levelSizeLimits[level] {
		return nil
	}

	// 改进策略：根据层级压力动态调整选择策略
	// 1. 计算当前层级的压力（超过限制的倍数）
	pressure := float64(totalSize) / float64(p.levelSizeLimits[level])

	// 2. 根据压力确定目标大小和文件数量限制
	targetSize := p.getTargetCompactionSize(level + 1)
	maxFiles := 10 // 默认最多 10 个文件

	if pressure >= 10.0 {
		// 压力极高（超过 10 倍）：选择更多文件，增大目标
		maxFiles = 100
		targetSize *= 5
		fmt.Printf("[Compaction] L%d pressure: %.1fx (CRITICAL) - selecting up to %d files, target: %s\n",
			level, pressure, maxFiles, formatBytes(targetSize))
	} else if pressure >= 5.0 {
		// 压力很高（超过 5 倍）
		maxFiles = 50
		targetSize *= 3
		fmt.Printf("[Compaction] L%d pressure: %.1fx (HIGH) - selecting up to %d files, target: %s\n",
			level, pressure, maxFiles, formatBytes(targetSize))
	} else if pressure >= 2.0 {
		// 压力较高（超过 2 倍）
		maxFiles = 20
		targetSize *= 2
		fmt.Printf("[Compaction] L%d pressure: %.1fx (ELEVATED) - selecting up to %d files, target: %s\n",
			level, pressure, maxFiles, formatBytes(targetSize))
	}

	// 选择文件，直到累计大小接近目标
	selectedFiles := make([]*manifest.FileMetadata, 0)
	currentSize := int64(0)

	for _, file := range files {
		selectedFiles = append(selectedFiles, file)
		currentSize += file.FileSize

		// 如果已经达到目标大小，停止选择
		if currentSize >= targetSize {
			break
		}

		// 达到文件数量限制
		if len(selectedFiles) >= maxFiles {
			break
		}
	}

	return &CompactionTask{
		Level:       level,
		InputFiles:  selectedFiles,
		OutputLevel: level + 1,
	}
}

// getTargetCompactionSize 根据层级返回建议的 compaction 大小
func (p *Picker) getTargetCompactionSize(level int) int64 {
	switch level {
	case 0:
		return 2 * 1024 * 1024 // 2MB
	case 1:
		return 10 * 1024 * 1024 // 10MB
	case 2:
		return 50 * 1024 * 1024 // 50MB
	case 3:
		return 100 * 1024 * 1024 // 100MB
	default: // L4+
		return 200 * 1024 * 1024 // 200MB
	}
}

// ShouldCompact 判断是否需要 Compaction
func (p *Picker) ShouldCompact(version *manifest.Version) bool {
	tasks := p.PickCompaction(version)
	return len(tasks) > 0
}

// GetLevelScore 获取每层的 Compaction 得分 (用于优先级排序)
// 得分越高，越需要 Compaction
func (p *Picker) GetLevelScore(version *manifest.Version, level int) float64 {
	if level < 0 || level >= manifest.NumLevels {
		return 0
	}

	files := version.GetLevel(level)

	// L0 同时考虑文件数量和总大小，取较大值作为得分
	if level == 0 {
		scoreByCount := float64(0)
		scoreBySize := float64(0)

		if p.levelFileLimits[0] > 0 {
			scoreByCount = float64(len(files)) / float64(p.levelFileLimits[0])
		}

		if p.levelSizeLimits[0] > 0 {
			totalSize := int64(0)
			for _, file := range files {
				totalSize += file.FileSize
			}
			scoreBySize = float64(totalSize) / float64(p.levelSizeLimits[0])
		}

		// 返回两者中的较大值（哪个维度更紧迫）
		if scoreByCount > scoreBySize {
			return scoreByCount
		}
		return scoreBySize
	}

	// L1+ 基于总大小
	if p.levelSizeLimits[level] == 0 {
		return 0
	}

	totalSize := int64(0)
	for _, file := range files {
		totalSize += file.FileSize
	}

	return float64(totalSize) / float64(p.levelSizeLimits[level])
}

// formatBytes 格式化字节大小显示
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), units[exp])
}
