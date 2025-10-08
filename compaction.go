package srdb

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// CompactionTask 表示一个 Compaction 任务
type CompactionTask struct {
	Level       int             // 源层级
	InputFiles  []*FileMetadata // 需要合并的输入文件
	OutputLevel int             // 输出层级
}

// Picker 负责选择需要 Compaction 的文件
type Picker struct {
	// Level 大小限制 (字节)
	levelSizeLimits [NumLevels]int64

	// Level 文件数量限制
	levelFileLimits [NumLevels]int
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
	for i := 1; i < NumLevels; i++ {
		p.levelFileLimits[i] = 0 // 0 表示不限制
	}

	return p
}

// PickCompaction 选择需要 Compaction 的任务（支持多任务并发）
// 返回空切片表示当前不需要 Compaction
func (p *Picker) PickCompaction(version *Version) []*CompactionTask {
	tasks := make([]*CompactionTask, 0)

	// 1. 检查 L0 (基于文件数量)
	if task := p.pickL0Compaction(version); task != nil {
		tasks = append(tasks, task)
	}

	// 2. 检查 L1-L5 (基于大小)
	for level := 1; level < NumLevels-1; level++ {
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
func (p *Picker) sortTasksByPriority(tasks []*CompactionTask, version *Version) {
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
func (p *Picker) pickL0Compaction(version *Version) *CompactionTask {
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
func (p *Picker) pickLevelCompaction(version *Version, level int) *CompactionTask {
	if level < 1 || level >= NumLevels-1 {
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
	selectedFiles := make([]*FileMetadata, 0)
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
func (p *Picker) ShouldCompact(version *Version) bool {
	tasks := p.PickCompaction(version)
	return len(tasks) > 0
}

// GetLevelScore 获取每层的 Compaction 得分 (用于优先级排序)
// 得分越高，越需要 Compaction
func (p *Picker) GetLevelScore(version *Version, level int) float64 {
	if level < 0 || level >= NumLevels {
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

// Compactor 负责执行 Compaction
type Compactor struct {
	sstDir     string
	picker     *Picker
	versionSet *VersionSet
	schema     *Schema
	mu         sync.Mutex
}

// NewCompactor 创建新的 Compactor
func NewCompactor(sstDir string, versionSet *VersionSet) *Compactor {
	return &Compactor{
		sstDir:     sstDir,
		picker:     NewPicker(),
		versionSet: versionSet,
	}
}

// SetSchema 设置 Schema（用于读取 SST 文件）
func (c *Compactor) SetSchema(schema *Schema) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.schema = schema
}

// GetPicker 获取 Picker
func (c *Compactor) GetPicker() *Picker {
	return c.picker
}

// DoCompaction 执行一次 Compaction
// 返回: VersionEdit (记录变更), error
func (c *Compactor) DoCompaction(task *CompactionTask, version *Version) (*VersionEdit, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if task == nil {
		return nil, fmt.Errorf("compaction task is nil")
	}

	// 0. 验证输入文件是否存在（防止并发 compaction 导致的竞态）
	existingInputFiles := make([]*FileMetadata, 0, len(task.InputFiles))
	for _, file := range task.InputFiles {
		sstPath := filepath.Join(c.sstDir, fmt.Sprintf("%06d.sst", file.FileNumber))
		if _, err := os.Stat(sstPath); err == nil {
			existingInputFiles = append(existingInputFiles, file)
		} else {
			fmt.Printf("[Compaction] Warning: input file %06d.sst not found, skipping from task\n", file.FileNumber)
		}
	}

	// 如果所有输入文件都不存在，直接返回（无需 compaction）
	if len(existingInputFiles) == 0 {
		fmt.Printf("[Compaction] All input files missing, compaction skipped\n")
		return nil, nil // 返回 nil 表示不需要应用任何 VersionEdit
	}

	// 1. 读取输入文件的所有行
	inputRows, err := c.readInputFiles(existingInputFiles)
	if err != nil {
		return nil, fmt.Errorf("read input files: %w", err)
	}

	// 2. 如果输出层级有文件，需要合并重叠的文件
	outputFiles := c.getOverlappingFiles(version, task.OutputLevel, inputRows)
	var existingOutputFiles []*FileMetadata
	var missingOutputFiles []*FileMetadata
	if len(outputFiles) > 0 {
		// 验证输出文件是否存在
		existingOutputFiles = make([]*FileMetadata, 0, len(outputFiles))
		missingOutputFiles = make([]*FileMetadata, 0)
		for _, file := range outputFiles {
			sstPath := filepath.Join(c.sstDir, fmt.Sprintf("%06d.sst", file.FileNumber))
			if _, err := os.Stat(sstPath); err == nil {
				existingOutputFiles = append(existingOutputFiles, file)
			} else {
				// 输出层级的文件不存在，记录并在 VersionEdit 中删除它
				fmt.Printf("[Compaction] Warning: overlapping output file %06d.sst missing, will remove from MANIFEST\n", file.FileNumber)
				missingOutputFiles = append(missingOutputFiles, file)
			}
		}

		outputRows, err := c.readInputFiles(existingOutputFiles)
		if err != nil {
			return nil, fmt.Errorf("read output files: %w", err)
		}
		inputRows = append(inputRows, outputRows...)
	}

	// 3. 合并和去重 (保留最新的记录)
	mergedRows := c.mergeRows(inputRows)

	// 计算平均行大小（基于输入文件的 FileMetadata）
	avgRowSize := c.calculateAvgRowSize(existingInputFiles, existingOutputFiles)

	// 4. 写入新的 SST 文件到输出层级
	newFiles, err := c.writeOutputFiles(mergedRows, task.OutputLevel, avgRowSize)
	if err != nil {
		return nil, fmt.Errorf("write output files: %w", err)
	}

	// 5. 创建 VersionEdit
	edit := NewVersionEdit()

	// 删除实际存在且被处理的输入文件
	for _, file := range existingInputFiles {
		edit.DeleteFile(file.FileNumber)
	}
	// 删除实际存在且被处理的输出层级文件
	for _, file := range existingOutputFiles {
		edit.DeleteFile(file.FileNumber)
	}
	// 删除缺失的输出层级文件（清理 MANIFEST 中的过期引用）
	for _, file := range missingOutputFiles {
		edit.DeleteFile(file.FileNumber)
		fmt.Printf("[Compaction] Removing missing file %06d.sst from MANIFEST\n", file.FileNumber)
	}

	// 添加新文件，并跟踪最大文件编号
	var maxFileNumber int64
	for _, file := range newFiles {
		edit.AddFile(file)
		if file.FileNumber > maxFileNumber {
			maxFileNumber = file.FileNumber
		}
	}

	// 持久化当前的文件编号计数器（关键修复：防止重启后文件编号重用）
	// 使用最大文件编号 + 1 确保并发安全
	if maxFileNumber > 0 {
		edit.SetNextFileNumber(maxFileNumber + 1)
	} else {
		// 如果没有新文件，使用当前值
		edit.SetNextFileNumber(c.versionSet.GetNextFileNumber())
	}

	return edit, nil
}

// readInputFiles 读取输入文件的所有行
// 注意：调用者必须确保传入的文件都存在，否则会返回错误
func (c *Compactor) readInputFiles(files []*FileMetadata) ([]*SSTableRow, error) {
	var allRows []*SSTableRow

	for _, file := range files {
		sstPath := filepath.Join(c.sstDir, fmt.Sprintf("%06d.sst", file.FileNumber))

		reader, err := NewSSTableReader(sstPath)
		if err != nil {
			return nil, fmt.Errorf("open sst %d: %w", file.FileNumber, err)
		}

		// 设置 Schema（如果可用）
		if c.schema != nil {
			reader.SetSchema(c.schema)
		}

		// 获取文件中实际存在的所有 key（不能用 MinKey-MaxKey 范围遍历，因为 key 可能是稀疏的）
		keys := reader.GetAllKeys()
		for _, seq := range keys {
			row, err := reader.Get(seq)
			if err != nil {
				// 这种情况理论上不应该发生（key 来自索引），但为了安全还是处理一下
				continue
			}
			allRows = append(allRows, row)
		}

		reader.Close()
	}

	return allRows, nil
}

// getOverlappingFiles 获取输出层级中与输入行重叠的文件
func (c *Compactor) getOverlappingFiles(version *Version, level int, rows []*SSTableRow) []*FileMetadata {
	if len(rows) == 0 {
		return nil
	}

	// 找到输入行的 key range
	minKey := rows[0].Seq
	maxKey := rows[0].Seq
	for _, row := range rows {
		if row.Seq < minKey {
			minKey = row.Seq
		}
		if row.Seq > maxKey {
			maxKey = row.Seq
		}
	}

	// 找到输出层级中重叠的文件
	var overlapping []*FileMetadata
	levelFiles := version.GetLevel(level)
	for _, file := range levelFiles {
		// 检查 key range 是否重叠
		if file.MaxKey >= minKey && file.MinKey <= maxKey {
			overlapping = append(overlapping, file)
		}
	}

	return overlapping
}

// mergeRows 合并行，去重并保留最新的记录
func (c *Compactor) mergeRows(rows []*SSTableRow) []*SSTableRow {
	if len(rows) == 0 {
		return rows
	}

	// 按 Seq 排序
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Seq < rows[j].Seq
	})

	// 去重：保留相同 Seq 的最新记录 (Timestamp 最大的)
	merged := make([]*SSTableRow, 0, len(rows))
	var lastRow *SSTableRow

	for _, row := range rows {
		if lastRow == nil || lastRow.Seq != row.Seq {
			// 新的 Seq
			merged = append(merged, row)
			lastRow = row
		} else {
			// 相同 Seq，保留 Time 更大的
			if row.Time > lastRow.Time {
				merged[len(merged)-1] = row
				lastRow = row
			}
		}
	}

	return merged
}

// calculateAvgRowSize 基于输入文件的 FileMetadata 计算平均行大小
func (c *Compactor) calculateAvgRowSize(inputFiles []*FileMetadata, outputFiles []*FileMetadata) int64 {
	var totalSize int64
	var totalRows int64

	// 统计输入文件
	for _, file := range inputFiles {
		totalSize += file.FileSize
		totalRows += file.RowCount
	}

	// 统计输出文件
	for _, file := range outputFiles {
		totalSize += file.FileSize
		totalRows += file.RowCount
	}

	// 计算平均值
	if totalRows == 0 {
		return 1024 // 默认 1KB
	}
	return totalSize / totalRows
}

// writeOutputFiles 将合并后的行写入新的 SST 文件
func (c *Compactor) writeOutputFiles(rows []*SSTableRow, level int, avgRowSize int64) ([]*FileMetadata, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	// 根据层级动态调整文件大小目标
	// L0: 2MB (快速 flush，小文件)
	// L1: 10MB
	// L2: 50MB
	// L3: 100MB
	// L4+: 200MB
	targetFileSize := c.getTargetFileSize(level)

	// 应用安全系数：由于压缩率、索引开销等因素，估算值可能不准确
	// 使用 80% 的目标大小作为分割点，避免实际文件超出目标过多
	targetFileSize = targetFileSize * 80 / 100

	var newFiles []*FileMetadata
	var currentRows []*SSTableRow
	var currentSize int64

	for _, row := range rows {
		// 使用平均行大小估算（基于输入文件的统计信息）
		rowSize := avgRowSize

		// 如果当前文件大小超过目标，写入文件
		if currentSize > 0 && currentSize+rowSize > targetFileSize {
			file, err := c.writeFile(currentRows, level)
			if err != nil {
				return nil, err
			}
			newFiles = append(newFiles, file)

			// 重置
			currentRows = nil
			currentSize = 0
		}

		currentRows = append(currentRows, row)
		currentSize += rowSize
	}

	// 写入最后一个文件
	if len(currentRows) > 0 {
		file, err := c.writeFile(currentRows, level)
		if err != nil {
			return nil, err
		}
		newFiles = append(newFiles, file)
	}

	return newFiles, nil
}

// getTargetFileSize 根据层级返回目标文件大小
func (c *Compactor) getTargetFileSize(level int) int64 {
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

// writeFile 写入单个 SST 文件
func (c *Compactor) writeFile(rows []*SSTableRow, level int) (*FileMetadata, error) {
	// 从 VersionSet 分配新的文件编号
	fileNumber := c.versionSet.AllocateFileNumber()
	sstPath := filepath.Join(c.sstDir, fmt.Sprintf("%06d.sst", fileNumber))

	// 创建文件
	file, err := os.Create(sstPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 使用 Compactor 的 Schema 创建 writer
	writer := NewSSTableWriter(file, c.schema)

	// 注意：这个方法只负责创建文件，不负责注册到 SSTableManager
	// 注册工作由 CompactionManager 在 VersionEdit apply 后完成

	// 写入所有行
	for _, row := range rows {
		err = writer.Add(row)
		if err != nil {
			os.Remove(sstPath)
			return nil, err
		}
	}

	// 完成写入
	err = writer.Finish()
	if err != nil {
		os.Remove(sstPath)
		return nil, err
	}

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// 创建 FileMetadata
	metadata := &FileMetadata{
		FileNumber: fileNumber,
		Level:      level,
		FileSize:   fileInfo.Size(),
		MinKey:     rows[0].Seq,
		MaxKey:     rows[len(rows)-1].Seq,
		RowCount:   int64(len(rows)),
	}

	return metadata, nil
}

// CompactionManager 管理 Compaction 流程
type CompactionManager struct {
	compactor  *Compactor
	versionSet *VersionSet
	sstManager *SSTableManager // 添加 sstManager 引用，用于同步删除 readers
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

// NewCompactionManager 创建新的 Compaction Manager
func NewCompactionManager(sstDir string, versionSet *VersionSet, sstManager *SSTableManager) *CompactionManager {
	return &CompactionManager{
		compactor:  NewCompactor(sstDir, versionSet),
		versionSet: versionSet,
		sstManager: sstManager,
		sstDir:     sstDir,
		stopCh:     make(chan struct{}),
	}
}

// GetPicker 获取 Compaction Picker
func (m *CompactionManager) GetPicker() *Picker {
	return m.compactor.GetPicker()
}

// SetSchema 设置 Schema（用于优化 SST 文件读写）
func (m *CompactionManager) SetSchema(schema *Schema) {
	m.compactor.SetSchema(schema)
}

// Start 启动后台 Compaction 和垃圾回收
func (m *CompactionManager) Start() {
	m.wg.Add(2)
	go m.backgroundCompaction()
	go m.backgroundGarbageCollection()
}

// Stop 停止后台 Compaction
func (m *CompactionManager) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

// backgroundCompaction 后台 Compaction 循环
func (m *CompactionManager) backgroundCompaction() {
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
func (m *CompactionManager) MaybeCompact() {
	// 尝试获取锁，如果已有 compaction 在执行，直接返回
	if !m.compactionMu.TryLock() {
		return
	}
	defer m.compactionMu.Unlock()

	m.doCompact()
}

// maybeCompact 内部使用的阻塞版本（后台 goroutine 使用）
func (m *CompactionManager) maybeCompact() {
	m.compactionMu.Lock()
	defer m.compactionMu.Unlock()

	m.doCompact()
}

// doCompact 实际执行 compaction 的逻辑（必须在持有 compactionMu 时调用）
// 支持并发执行多个层级的 compaction
func (m *CompactionManager) doCompact() {
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
func (m *CompactionManager) printCompactionStats(version *Version, picker *Picker) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 限制输出频率：每 60 秒输出一次
	if time.Since(m.lastCompactionTime) < 60*time.Second {
		return
	}
	m.lastCompactionTime = time.Now()

	fmt.Println("[Compaction] Status check:")
	for level := range 7 {
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
func (m *CompactionManager) DoCompactionWithVersion(task *CompactionTask, version *Version) error {
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

	// LogAndApply 成功后，注册新创建的 SST 文件到 SSTableManager
	// 这样查询才能读取到 compaction 创建的文件
	if m.sstManager != nil {
		for _, file := range edit.AddedFiles {
			sstPath := filepath.Join(m.sstDir, fmt.Sprintf("%06d.sst", file.FileNumber))
			reader, err := NewSSTableReader(sstPath)
			if err != nil {
				fmt.Printf("[Compaction] Warning: failed to open new file %06d.sst: %v\n", file.FileNumber, err)
				continue
			}
			// 设置 Schema
			if m.compactor.schema != nil {
				reader.SetSchema(m.compactor.schema)
			}
			// 添加到 SSTableManager
			m.sstManager.AddReader(reader)
		}
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
func (m *CompactionManager) DoCompaction(task *CompactionTask) error {
	// 获取当前版本
	version := m.versionSet.GetCurrent()
	if version == nil {
		return fmt.Errorf("no current version")
	}

	return m.DoCompactionWithVersion(task, version)
}

// cleanupNewFiles 清理 LogAndApply 失败后的新文件（防止孤儿文件）
func (m *CompactionManager) cleanupNewFiles(edit *VersionEdit) {
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
func (m *CompactionManager) deleteObsoleteFiles(edit *VersionEdit) {
	if edit == nil {
		fmt.Printf("[Compaction] deleteObsoleteFiles: edit is nil\n")
		return
	}

	fmt.Printf("[Compaction] deleteObsoleteFiles: %d files to delete\n", len(edit.DeletedFiles))

	// 删除被标记为删除的文件
	for _, fileNum := range edit.DeletedFiles {
		// 1. 从 SSTableManager 移除 reader（如果 sstManager 可用）
		if m.sstManager != nil {
			err := m.sstManager.RemoveReader(fileNum)
			if err != nil {
				fmt.Printf("[Compaction] Failed to remove reader for %06d.sst: %v\n", fileNum, err)
			}
		}

		// 2. 删除物理文件
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
func (m *CompactionManager) TriggerCompaction() error {
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
func (m *CompactionManager) GetStats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]any{
		"total_compactions":    m.totalCompactions,
		"last_compaction_time": m.lastCompactionTime,
	}
}

// GetLevelStats 获取每层的统计信息
func (m *CompactionManager) GetLevelStats() []map[string]any {
	version := m.versionSet.GetCurrent()
	if version == nil {
		return nil
	}

	picker := m.compactor.GetPicker()
	stats := make([]map[string]any, NumLevels)

	for level := range NumLevels {
		files := version.GetLevel(level)
		totalSize := int64(0)
		for _, file := range files {
			totalSize += file.FileSize
		}

		stats[level] = map[string]any{
			"level":      level,
			"file_count": len(files),
			"total_size": totalSize,
			"score":      picker.GetLevelScore(version, level),
		}
	}

	return stats
}

// backgroundGarbageCollection 后台垃圾回收循环
func (m *CompactionManager) backgroundGarbageCollection() {
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
func (m *CompactionManager) collectOrphanFiles() {
	// 1. 获取当前版本中的所有活跃文件
	version := m.versionSet.GetCurrent()
	if version == nil {
		return
	}

	activeFiles := make(map[int64]bool)
	for level := range NumLevels {
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
			// 检查文件修改时间，避免删除正在 flush 的文件
			// 如果文件在最近 1 分钟内创建/修改，跳过（可能正在 LogAndApply）
			fileInfo, err := os.Stat(sstPath)
			if err != nil {
				continue
			}
			if time.Since(fileInfo.ModTime()) < 1*time.Minute {
				fmt.Printf("[GC] Skipping recently modified file %06d.sst (age: %v)\n",
					fileNum, time.Since(fileInfo.ModTime()))
				continue
			}

			// 这是孤儿文件，删除它
			err = os.Remove(sstPath)
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
func (m *CompactionManager) CleanupOrphanFiles() {
	fmt.Println("[GC] Manual cleanup triggered")
	m.collectOrphanFiles()
}
