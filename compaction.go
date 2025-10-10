package srdb

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Compaction 层级大小限制（Append-Only 优化设计）
//
// 设计理念：
// - 触发阈值 = 目标文件大小（不 split）
// - 每个层级累积到阈值后，直接合并成一个对应大小的文件
// - 适用于 Append-Only 场景：没有更新/删除，不需要增量合并
//
// 触发规则：
// - L0 累积到 64MB  → 合并成 1 个 64MB  文件，升级到 L1
// - L1 累积到 256MB → 合并成 1 个 256MB 文件，升级到 L2
// - L2 累积到 512MB → 合并成 1 个 512MB 文件，升级到 L3
// - L3 累积到 1GB   → 保持在 L3（最后一层）
//
// 层级设计（Append-Only 优化）：
// - L0：64MB   （MemTable flush，小文件合并）
// - L1：256MB  （L0 升级，减少文件数）
// - L2：512MB  （L1 升级，温数据）
// - L3：1GB    （L2 升级，冷数据，最后一层）
const (
	level0SizeLimit = 64 * 1024 * 1024   // 64MB
	level1SizeLimit = 256 * 1024 * 1024  // 256MB
	level2SizeLimit = 512 * 1024 * 1024  // 512MB
	level3SizeLimit = 1024 * 1024 * 1024 // 1GB
)

// getLevelSizeLimit 获取层级大小限制（私有函数，供 Picker 和 Compactor 共用）
func getLevelSizeLimit(level int) int64 {
	switch level {
	case 0:
		return level0SizeLimit
	case 1:
		return level1SizeLimit
	case 2:
		return level2SizeLimit
	case 3:
		return level3SizeLimit
	default:
		return level3SizeLimit
	}
}

// CompactionTask 表示一个 Compaction 任务
type CompactionTask struct {
	Level       int             // 源层级
	InputFiles  []*FileMetadata // 需要合并的输入文件
	OutputLevel int             // 输出层级
}

// Picker 负责选择需要 Compaction 的文件
type Picker struct {
	mu           sync.Mutex
	currentStage int // 当前阶段：0=L0合并, 1=L0升级, 2=L1升级, 3=L2升级

	// 层级大小限制（可配置）
	level0SizeLimit int64
	level1SizeLimit int64
	level2SizeLimit int64
	level3SizeLimit int64
}

// NewPicker 创建新的 Compaction Picker（使用默认值）
func NewPicker() *Picker {
	return &Picker{
		currentStage:    0, // 从 L0 合并开始
		level0SizeLimit: level0SizeLimit,
		level1SizeLimit: level1SizeLimit,
		level2SizeLimit: level2SizeLimit,
		level3SizeLimit: level3SizeLimit,
	}
}

// UpdateLevelLimits 更新层级大小限制（由 CompactionManager 调用）
func (p *Picker) UpdateLevelLimits(l0, l1, l2, l3 int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.level0SizeLimit = l0
	p.level1SizeLimit = l1
	p.level2SizeLimit = l2
	p.level3SizeLimit = l3
}

// getLevelSizeLimit 获取层级大小限制（从配置读取）
func (p *Picker) getLevelSizeLimit(level int) int64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch level {
	case 0:
		return p.level0SizeLimit
	case 1:
		return p.level1SizeLimit
	case 2:
		return p.level2SizeLimit
	case 3:
		return p.level3SizeLimit
	default:
		return p.level3SizeLimit
	}
}

// PickCompaction 选择需要 Compaction 的任务（按阶段返回，阶段内并发执行）
// 返回空切片表示当前阶段不需要 Compaction
//
// 执行策略（4 阶段串行 + 阶段内并发）：
// 1. Stage 0 - L0 合并：小文件合并，减少 L0 文件数（可能保持在 L0）
// 2. Stage 1 - L0 升级：大文件或已合并文件升级到 L1
// 3. Stage 2 - L1 升级：L1 文件升级到 L2
// 4. Stage 3 - L2 升级：L2 文件升级到 L3
//
// 阶段控制：
// - 使用 currentStage 跟踪当前应该执行哪个阶段（0-3）
// - 每次调用返回当前阶段的任务，然后自动推进到下一阶段
// - 调用者应该循环调用此方法以确保所有阶段都被尝试
//
// 为什么阶段内可以并发？
// - 同一阶段的任务处理不同的文件批次（按 seq 连续划分）
// - 这些批次的文件不重叠，可以安全并发
//
// 为什么阶段间要串行？
// - L0 执行后可能产生新文件，影响下一阶段的任务计算
// - 必须基于最新的 version 重新计算下一阶段的任务
func (p *Picker) PickCompaction(version *Version) []*CompactionTask {
	p.mu.Lock()
	defer p.mu.Unlock()

	var tasks []*CompactionTask

	// 根据当前阶段选择任务
	switch p.currentStage {
	case 0:
		// Stage 0: L0 合并任务
		tasks = p.pickL0MergeTasks(version)
	case 1:
		// Stage 1: L0 升级任务
		tasks = p.pickL0UpgradeTasks(version)
	case 2:
		// Stage 2: L1 升级任务
		tasks = p.pickLevelCompaction(version, 1)
	case 3:
		// Stage 3: L2 升级任务
		tasks = p.pickLevelCompaction(version, 2)
	}

	// 推进到下一阶段（无论是否有任务），这里巧妙地
	// 使用了取模运算来保证阶段递增与阶段重置。
	p.currentStage = (p.currentStage + 1) % 4

	return tasks
}

// pickL0MergeTasks 选择 L0 的合并任务（Stage 0）
//
// 策略：
// - 按 seq 顺序遍历，只合并连续的小文件块
// - 遇到大文件（≥ 32MB）时停止当前批次，创建任务
// - 累积到 64MB 时创建一个合并任务
// - OutputLevel=0，让 determineLevel 决定是否保持在 L0
//
// 为什么必须连续？
// - 不能跳过中间文件，否则会导致 seq 范围不连续
// - 例如：[文件1: seq 1-100, 29MB] [文件2: seq 101-200, 36MB] [文件3: seq 201-300, 8MB]
// - 不能合并文件1和文件3，否则新文件 seq 范围是 [1-100, 201-300]，缺失 101-200
//
// 目的：
// - 减少 L0 文件数（防止读放大）
// - 小文件在 L0 内部合并，大文件留给 Stage 1 处理
func (p *Picker) pickL0MergeTasks(version *Version) []*CompactionTask {
	files := version.GetLevel(0)
	if len(files) == 0 {
		return nil
	}

	// 按 MinKey 排序，确保处理连续的 seq
	sort.Slice(files, func(i, j int) bool {
		return files[i].MinKey < files[j].MinKey
	})

	const smallFileThreshold = 32 * 1024 * 1024 // 32MB

	tasks := make([]*CompactionTask, 0)
	var currentBatch []*FileMetadata
	var currentSize int64

	for _, file := range files {
		// 如果是大文件，停止当前批次
		if file.FileSize >= smallFileThreshold {
			// 如果当前批次有多个小文件（> 1），创建合并任务
			if len(currentBatch) > 1 {
				tasks = append(tasks, &CompactionTask{
					Level:       0,
					InputFiles:  currentBatch,
					OutputLevel: 0,
				})
			}
			// 重置批次（单个小文件不合并，留给升级阶段）
			currentBatch = nil
			currentSize = 0
			// 跳过大文件，留给 Stage 1 处理
			continue
		}

		// 小文件：加入当前批次
		currentBatch = append(currentBatch, file)
		currentSize += file.FileSize

		// 累积到 64MB 时创建合并任务
		if currentSize >= level0SizeLimit {
			tasks = append(tasks, &CompactionTask{
				Level:       0,
				InputFiles:  currentBatch,
				OutputLevel: 0, // 建议 L0，determineLevel 会根据大小决定
			})

			currentBatch = nil
			currentSize = 0
		}
	}

	// 剩余的小文件：只有 > 1 个才创建合并任务
	if len(currentBatch) > 1 {
		tasks = append(tasks, &CompactionTask{
			Level:       0,
			InputFiles:  currentBatch,
			OutputLevel: 0,
		})
	}

	return tasks
}

// pickL0UpgradeTasks 选择 L0 的升级任务（Stage 1）
//
// 策略：
// - 以大文件（≥ 32MB）为中心，搭配前后的文件一起升级
// - 找到大文件后，向左右扩展收集文件，直到累积到 256MB（L1 限制）
// - 这样可以把大文件周围的小文件也一起带走，更高效地清理 L0
// - OutputLevel=1，强制升级到 L1+
//
// 为什么要搭配周围文件？
// - 大文件是升级的主体，周围的小文件（包括单个小文件）可以顺便带走
// - 避免小文件留在 L0 成为孤立文件
// - 例如：[小20MB] [大40MB] [小15MB] → 一起升级 → L1
//
// 目的：
// - 将成熟的大文件推到 L1
// - 顺便清理周围的小文件，为 L0 腾出空间
func (p *Picker) pickL0UpgradeTasks(version *Version) []*CompactionTask {
	files := version.GetLevel(0)
	if len(files) == 0 {
		return nil
	}

	// 按 MinKey 排序，确保处理连续的 seq
	sort.Slice(files, func(i, j int) bool {
		return files[i].MinKey < files[j].MinKey
	})

	const largeFileThreshold = 32 * 1024 * 1024 // 32MB

	tasks := make([]*CompactionTask, 0)
	processed := make(map[int64]bool) // 跟踪已处理的文件

	// 遍历文件，找到大文件作为起点
	for i, file := range files {
		// 跳过已处理的文件
		if processed[file.FileNumber] {
			continue
		}

		// 如果不是大文件，跳过（小文件等待被大文件带走）
		if file.FileSize < largeFileThreshold {
			continue
		}

		// 找到大文件，以它为中心，向左右扩展收集文件
		var batch []*FileMetadata
		var batchSize int64

		// 向左收集：找到连续的未处理文件
		left := i - 1
		var leftFiles []*FileMetadata
		for left >= 0 && !processed[files[left].FileNumber] {
			leftFiles = append([]*FileMetadata{files[left]}, leftFiles...) // 前插
			left--
		}

		// 加入左边的文件
		for _, f := range leftFiles {
			batch = append(batch, f)
			batchSize += f.FileSize
			processed[f.FileNumber] = true
		}

		// 加入中心的大文件
		batch = append(batch, file)
		batchSize += file.FileSize
		processed[file.FileNumber] = true

		// 向右收集：继续收集直到达到 256MB 或遇到已处理文件
		right := i + 1
		for right < len(files) && !processed[files[right].FileNumber] {
			// 检查是否超过限制
			if batchSize+files[right].FileSize > level1SizeLimit {
				break
			}
			batch = append(batch, files[right])
			batchSize += files[right].FileSize
			processed[files[right].FileNumber] = true
			right++
		}

		// 创建升级任务
		if len(batch) > 0 {
			tasks = append(tasks, &CompactionTask{
				Level:       0,
				InputFiles:  batch,
				OutputLevel: 1, // 升级到 L1+
			})
		}
	}

	return tasks
}

// pickLevelCompaction 选择指定层级的 Compaction 任务（用于 L1、L2，返回所有任务可并发执行）
//
// 触发规则：
// - 按 seq 顺序遍历当前层级的文件
// - 累积连续文件的大小
// - 当累积大小 >= 当前层级的大小限制时，创建一个 compaction 任务
// - 重置累积，继续处理剩余文件
// - 返回所有任务（可并发执行）
//
// 示例：L1 有 5 个文件 [50MB, 60MB, 70MB, 80MB, 90MB]
// - L1 的限制是 256MB
// - 文件1+2+3+4 = 260MB >= 256MB → 创建任务（4个文件 → L2）
// - 文件5 = 90MB < 256MB → 不创建任务（未达到升级条件）
// - 返回 1 个任务
func (p *Picker) pickLevelCompaction(version *Version, level int) []*CompactionTask {
	if level < 0 || level >= NumLevels-1 {
		return nil
	}

	files := version.GetLevel(level)
	if len(files) == 0 {
		return nil
	}

	// 按 MinKey 排序，确保处理连续的 seq
	sort.Slice(files, func(i, j int) bool {
		return files[i].MinKey < files[j].MinKey
	})

	tasks := make([]*CompactionTask, 0)
	currentLevelLimit := p.getLevelSizeLimit(level)

	// 遍历文件，累积大小，当达到当前层级的大小限制时创建任务
	var currentBatch []*FileMetadata
	var currentSize int64

	for _, file := range files {
		currentBatch = append(currentBatch, file)
		currentSize += file.FileSize

		// 如果当前批次的大小达到当前层级的限制，创建 compaction 任务
		if currentSize >= currentLevelLimit {
			tasks = append(tasks, &CompactionTask{
				Level:       level,
				InputFiles:  currentBatch,
				OutputLevel: level + 1,
			})

			// 重置批次
			currentBatch = nil
			currentSize = 0
		}
	}

	// 不处理剩余文件（未达到大小限制的文件不升级）
	// 等待更多文件累积后再升级

	return tasks
}

// GetCurrentStage 获取当前阶段（用于测试和调试）
func (p *Picker) GetCurrentStage() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.currentStage
}

// ShouldCompact 判断是否需要 Compaction（只检查，不推进阶段）
func (p *Picker) ShouldCompact(version *Version) bool {
	// 检查所有阶段，不推进 currentStage
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查 Stage 0: L0 合并
	if len(p.pickL0MergeTasks(version)) > 0 {
		return true
	}

	// 检查 Stage 1: L0 升级
	if len(p.pickL0UpgradeTasks(version)) > 0 {
		return true
	}

	// 检查 Stage 2: L1 升级
	if len(p.pickLevelCompaction(version, 1)) > 0 {
		return true
	}

	// 检查 Stage 3: L2 升级
	if len(p.pickLevelCompaction(version, 2)) > 0 {
		return true
	}

	return false
}

// GetLevelScore 获取每层的 Compaction 得分 (用于优先级排序)
// 得分越高，越需要 Compaction
func (p *Picker) GetLevelScore(version *Version, level int) float64 {
	if level < 0 || level >= NumLevels {
		return 0
	}

	// L3 是最后一层，不需要 compaction
	if level == NumLevels-1 {
		return 0
	}

	files := version.GetLevel(level)
	if len(files) == 0 {
		return 0
	}

	// 计算总大小
	totalSize := int64(0)
	for _, file := range files {
		totalSize += file.FileSize
	}

	// 使用下一级的大小限制来计算得分
	// 这样可以反映出该层级需要向上合并的紧迫程度
	nextLevelLimit := p.getLevelSizeLimit(level + 1)
	if nextLevelLimit == 0 {
		return 0
	}

	return float64(totalSize) / float64(nextLevelLimit)
}

// Compactor 负责执行 Compaction
type Compactor struct {
	sstDir     string
	picker     *Picker
	versionSet *VersionSet
	schema     *Schema
	logger     *slog.Logger
	mu         sync.RWMutex // 只保护 schema 和 logger 字段的读写
}

// NewCompactor 创建新的 Compactor
func NewCompactor(sstDir string, versionSet *VersionSet) *Compactor {
	return &Compactor{
		sstDir:     sstDir,
		picker:     NewPicker(),
		versionSet: versionSet,
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)), // 默认丢弃日志
	}
}

// SetSchema 设置 Schema（用于读取 SST 文件）
func (c *Compactor) SetSchema(schema *Schema) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.schema = schema
}

// SetLogger 设置 Logger
func (c *Compactor) SetLogger(logger *slog.Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logger = logger
}

// GetPicker 获取 Picker
func (c *Compactor) GetPicker() *Picker {
	return c.picker
}

// DoCompaction 执行一次 Compaction
// 返回: VersionEdit (记录变更), error
func (c *Compactor) DoCompaction(task *CompactionTask, version *Version) (*VersionEdit, error) {
	if task == nil {
		return nil, fmt.Errorf("compaction task is nil")
	}

	// 获取 logger
	c.mu.RLock()
	logger := c.logger
	c.mu.RUnlock()

	// 0. 验证输入文件是否存在（防止并发 compaction 导致的竞态）
	existingInputFiles := make([]*FileMetadata, 0, len(task.InputFiles))
	for _, file := range task.InputFiles {
		sstPath := filepath.Join(c.sstDir, fmt.Sprintf("%06d.sst", file.FileNumber))
		if _, err := os.Stat(sstPath); err == nil {
			existingInputFiles = append(existingInputFiles, file)
		} else {
			logger.Warn("[Compaction] Input file not found, skipping",
				"file_number", file.FileNumber)
		}
	}

	// 如果所有输入文件都不存在，直接返回（无需 compaction）
	if len(existingInputFiles) == 0 {
		logger.Warn("[Compaction] All input files missing, compaction skipped")
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
				logger.Warn("[Compaction] Overlapping output file missing, will remove from MANIFEST",
					"file_number", file.FileNumber)
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

	// 4. 写入新的 SST 文件
	// 传入输出层级，L0合并时根据文件大小动态决定，升级任务强制使用OutputLevel
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
		logger.Info("[Compaction] Removing missing file from MANIFEST",
			"file_number", file.FileNumber)
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
		c.mu.RLock()
		schema := c.schema
		c.mu.RUnlock()
		if schema != nil {
			reader.SetSchema(schema)
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

// writeOutputFiles 将合并后的行写入新的 SST 文件（Append-Only 优化：不 split）
//
// 设计理念：
// - Append-Only 场景：没有重叠数据，不需要增量合并
// - 直接将所有行写入一个文件，不进行分割
// - 简化逻辑，提高性能，减少文件数量
//
// 为什么不 split？
// - 触发阈值已经控制了文件大小（64MB/256MB/512MB/1GB）
// - 没有必要累积到大阈值后再分割成小文件
// - mmap 可以高效处理大文件（按需加载 4KB 页面）
func (c *Compactor) writeOutputFiles(rows []*SSTableRow, level int, avgRowSize int64) ([]*FileMetadata, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	// Append-Only 优化：不分割，直接写成一个文件
	file, err := c.writeFile(rows, level)
	if err != nil {
		return nil, err
	}

	return []*FileMetadata{file}, nil
}

// getTargetFileSize 根据层级返回目标文件大小（Append-Only 优化）
//
// 设计理念：
// - 目标文件大小 = 层级大小限制（不 split）
// - 每个层级合并后产生 1 个对应大小的文件
// - 适用于 Append-Only 场景：没有重叠数据，不需要增量合并
//
// 层级文件大小：
// - L0: 64MB   （MemTable flush 后的小文件合并）
// - L1: 256MB  （L0 升级）
// - L2: 512MB  （L1 升级）
// - L3: 1GB    （L2 升级，最后一层）
func (c *Compactor) getTargetFileSize(level int) int64 {
	return getLevelSizeLimit(level)
}

// determineLevel 根据文件大小和源层级决定文件应该放在哪一层
//
// 判断逻辑：
// - 如果文件大小 <= 源层级的目标文件大小 × 1.2，保持在源层级
// - 否则，向上查找合适的层级，直到文件大小 <= 该层级的目标文件大小 × 1.2
// - 如果都不适合，放在最后一层（L3）
//
// 1.2 倍容差的考量：
// - 目标文件大小是理想值（L0: 8MB, L1: 32MB, L2: 128MB, L3: 512MB）
// - 实际写入时可能略微超出（写入最后几行后超出）
// - 20% 容差避免文件因稍微超出而被强制升级
//
// 示例：
//   - 10MB 的文件，源层级 L0（目标 8MB × 1.2 = 9.6MB）
//     → 10MB > 9.6MB，升级到 L1
//   - 30MB 的文件，源层级 L1（目标 32MB × 1.2 = 38.4MB）
//     → 30MB <= 38.4MB，保持在 L1
func (c *Compactor) determineLevel(fileSize int64, sourceLevel int) int {
	// 检查文件是否适合源层级
	// 使用目标文件大小的 1.2 倍作为阈值（允许一定的溢出）
	sourceTargetSize := c.getTargetFileSize(sourceLevel)
	threshold := sourceTargetSize * 120 / 100

	if fileSize <= threshold {
		// 文件大小适合源层级，保持在源层级
		return sourceLevel
	}

	// 否则，找到合适的更高层级
	// 从源层级的下一层开始查找
	for level := sourceLevel + 1; level < NumLevels; level++ {
		targetSize := c.getTargetFileSize(level)
		threshold := targetSize * 120 / 100

		if fileSize <= threshold {
			return level
		}
	}

	// 如果都不适合，放在最后一层
	return NumLevels - 1
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
	c.mu.RLock()
	schema := c.schema
	c.mu.RUnlock()
	writer := NewSSTableWriter(file, schema)

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

	// 根据实际文件大小决定最终层级
	// level 参数是建议的输出层级（通常是 sourceLevel + 1）
	// 但我们根据文件大小重新决定，如果文件足够小可能保持在源层级
	actualLevel := c.determineLevel(fileInfo.Size(), level)

	// 创建 FileMetadata
	metadata := &FileMetadata{
		FileNumber: fileNumber,
		Level:      actualLevel,
		FileSize:   fileInfo.Size(),
		MinKey:     rows[0].Seq,
		MaxKey:     rows[len(rows)-1].Seq,
		RowCount:   int64(len(rows)),
	}

	return metadata, nil
}

// CompactionStats Compaction 统计信息
type CompactionStats struct {
	TotalCompactions   int64     `json:"total_compactions"`    // 总 compaction 次数
	LastCompactionTime time.Time `json:"last_compaction_time"` // 最后一次 compaction 时间
}

// LevelStats 层级统计信息
type LevelStats struct {
	Level     int     `json:"level"`      // 层级编号 (0-3)
	FileCount int     `json:"file_count"` // 文件数量
	TotalSize int64   `json:"total_size"` // 总大小（字节）
	Score     float64 `json:"score"`      // Compaction 得分
}

// CompactionManager 管理 Compaction 流程
type CompactionManager struct {
	compactor  *Compactor
	versionSet *VersionSet
	sstManager *SSTableManager // 添加 sstManager 引用，用于同步删除 readers
	sstDir     string

	// 配置（从 Database Options 传递）
	configMu           sync.RWMutex
	logger             *slog.Logger
	level0SizeLimit    int64
	level1SizeLimit    int64
	level2SizeLimit    int64
	level3SizeLimit    int64
	compactionInterval time.Duration
	gcInterval         time.Duration
	gcFileMinAge       time.Duration
	disableCompaction  bool
	disableGC          bool

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

// NewCompactionManager 创建新的 Compaction Manager（使用默认配置）
func NewCompactionManager(sstDir string, versionSet *VersionSet, sstManager *SSTableManager) *CompactionManager {
	return &CompactionManager{
		compactor:          NewCompactor(sstDir, versionSet),
		versionSet:         versionSet,
		sstManager:         sstManager,
		sstDir:             sstDir,
		stopCh:             make(chan struct{}),
		// 默认 logger：丢弃日志（将在 ApplyConfig 中设置为 Database.options.Logger）
		logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		// 使用硬编码常量作为默认值（向后兼容）
		level0SizeLimit:    level0SizeLimit,
		level1SizeLimit:    level1SizeLimit,
		level2SizeLimit:    level2SizeLimit,
		level3SizeLimit:    level3SizeLimit,
		compactionInterval: 10 * time.Second,
		gcInterval:         5 * time.Minute,
		gcFileMinAge:       1 * time.Minute,
		disableCompaction:  false,
		disableGC:          false,
	}
}

// ApplyConfig 应用数据库级配置（从 Database Options）
func (m *CompactionManager) ApplyConfig(opts *Options) {
	m.configMu.Lock()
	defer m.configMu.Unlock()

	m.logger = opts.Logger
	m.level0SizeLimit = opts.Level0SizeLimit
	m.level1SizeLimit = opts.Level1SizeLimit
	m.level2SizeLimit = opts.Level2SizeLimit
	m.level3SizeLimit = opts.Level3SizeLimit
	m.compactionInterval = opts.CompactionInterval
	m.gcInterval = opts.GCInterval
	m.gcFileMinAge = opts.GCFileMinAge
	m.disableCompaction = opts.DisableAutoCompaction
	m.disableGC = opts.DisableGC

	// 同时更新 compactor 的 picker 和 logger
	m.compactor.picker.UpdateLevelLimits(
		m.level0SizeLimit,
		m.level1SizeLimit,
		m.level2SizeLimit,
		m.level3SizeLimit,
	)
	m.compactor.SetLogger(opts.Logger)
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

	// 使用配置的间隔时间
	m.configMu.RLock()
	interval := m.compactionInterval
	disabled := m.disableCompaction
	m.configMu.RUnlock()

	if disabled {
		return // 禁用自动 Compaction，直接退出
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			// 检查配置是否被更新
			m.configMu.RLock()
			newInterval := m.compactionInterval
			disabled := m.disableCompaction
			m.configMu.RUnlock()

			if disabled {
				return // 运行中被禁用，退出
			}

			// 如果间隔时间改变，重新创建 ticker
			if newInterval != interval {
				interval = newInterval
				ticker.Reset(interval)
			}

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
// 阶段串行 + 阶段内并发：
// - 循环执行 4 个阶段（Stage 0 → 1 → 2 → 3）
// - 同一阶段的任务并发执行（L0 的多个批次、L1 的多个批次等）
// - 不同阶段串行执行（执行完一个阶段后，基于新 version 再执行下一阶段）
func (m *CompactionManager) doCompact() {
	picker := m.compactor.GetPicker()
	totalStagesExecuted := 0

	// 循环执行 4 个阶段
	for stage := 0; stage < 4; stage++ {
		// 获取当前版本（每个阶段都重新获取，因为上一阶段可能修改了文件结构）
		version := m.versionSet.GetCurrent()
		if version == nil {
			return
		}

		// 获取当前阶段的任务
		tasks := picker.PickCompaction(version)
		if len(tasks) == 0 {
			// 当前阶段没有任务，继续下一阶段
			continue
		}

		totalStagesExecuted++
		m.logger.Info("[Compaction] Found tasks to execute concurrently",
			"stage", stage,
			"task_count", len(tasks))

		// 并发执行同一阶段的所有任务
		var wg sync.WaitGroup
		var successCount atomic.Int64

		for _, task := range tasks {
			// 检查是否是上次失败的文件（防止无限重试）
			if len(task.InputFiles) > 0 {
				firstFile := task.InputFiles[0].FileNumber
				m.mu.Lock()
				if m.lastFailedFile == firstFile && m.consecutiveFails >= 3 {
					m.logger.Warn("[Compaction] Skipping file (failed multiple times)",
						"level", task.Level,
						"file_number", firstFile,
						"consecutive_fails", m.consecutiveFails)
					m.consecutiveFails = 0
					m.lastFailedFile = 0
					m.mu.Unlock()
					continue
				}
				m.mu.Unlock()
			}

			wg.Add(1)
			go func(task *CompactionTask) {
				defer wg.Done()

				// 获取最新版本（每个任务执行前）
				currentVersion := m.versionSet.GetCurrent()
				if currentVersion == nil {
					return
				}

				// 执行 Compaction
				m.logger.Info("[Compaction] Starting",
					"source_level", task.Level,
					"target_level", task.OutputLevel,
					"file_count", len(task.InputFiles))

				err := m.DoCompactionWithVersion(task, currentVersion)
				if err != nil {
					m.logger.Error("[Compaction] Failed",
						"source_level", task.Level,
						"target_level", task.OutputLevel,
						"error", err)

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
					m.logger.Info("[Compaction] Completed",
						"source_level", task.Level,
						"target_level", task.OutputLevel)

					// 清除失败计数
					m.mu.Lock()
					m.consecutiveFails = 0
					m.lastFailedFile = 0
					m.mu.Unlock()

					// 更新成功计数（使用原子操作）
					successCount.Add(1)
				}
			}(task)
		}

		// 等待当前阶段的所有任务完成
		wg.Wait()

		m.logger.Info("[Compaction] Stage completed",
			"stage", stage,
			"succeeded", successCount.Load(),
			"total", len(tasks))
	}

	// 如果所有阶段都没有任务，输出诊断信息
	if totalStagesExecuted == 0 {
		version := m.versionSet.GetCurrent()
		if version != nil {
			m.printCompactionStats(version, picker)
		}
	}
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

	m.logger.Info("[Compaction] Status check")
	for level := range NumLevels {
		files := version.GetLevel(level)
		if len(files) == 0 {
			continue
		}

		totalSize := int64(0)
		for _, f := range files {
			totalSize += f.FileSize
		}

		score := picker.GetLevelScore(version, level)
		m.logger.Info("[Compaction] Level status",
			"level", level,
			"file_count", len(files),
			"size_mb", float64(totalSize)/(1024*1024),
			"score", score)
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
		m.logger.Info("[Compaction] No changes needed (files already removed)")
		return nil
	}

	// 应用 VersionEdit
	err = m.versionSet.LogAndApply(edit)
	if err != nil {
		// LogAndApply 失败，清理已写入的新 SST 文件（防止孤儿文件）
		m.logger.Error("[Compaction] LogAndApply failed, cleaning up new files",
			"error", err)
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
				m.logger.Warn("[Compaction] Failed to open new file",
					"file_number", file.FileNumber,
					"error", err)
				continue
			}
			// 设置 Schema
			m.compactor.mu.RLock()
			schema := m.compactor.schema
			m.compactor.mu.RUnlock()
			if schema != nil {
				reader.SetSchema(schema)
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

	m.logger.Warn("[Compaction] Cleaning up new files after LogAndApply failure",
		"file_count", len(edit.AddedFiles))

	// 删除新创建的文件
	for _, file := range edit.AddedFiles {
		sstPath := filepath.Join(m.sstDir, fmt.Sprintf("%06d.sst", file.FileNumber))
		err := os.Remove(sstPath)
		if err != nil {
			m.logger.Warn("[Compaction] Failed to cleanup new file",
				"file_number", file.FileNumber,
				"error", err)
		} else {
			m.logger.Info("[Compaction] Cleaned up new file",
				"file_number", file.FileNumber)
		}
	}
}

// deleteObsoleteFiles 删除废弃的 SST 文件
func (m *CompactionManager) deleteObsoleteFiles(edit *VersionEdit) {
	if edit == nil {
		m.logger.Warn("[Compaction] deleteObsoleteFiles: edit is nil")
		return
	}

	m.logger.Info("[Compaction] Deleting obsolete files",
		"file_count", len(edit.DeletedFiles))

	// 删除被标记为删除的文件
	for _, fileNum := range edit.DeletedFiles {
		// 1. 从 SSTableManager 移除 reader（如果 sstManager 可用）
		if m.sstManager != nil {
			err := m.sstManager.RemoveReader(fileNum)
			if err != nil {
				m.logger.Warn("[Compaction] Failed to remove reader",
					"file_number", fileNum,
					"error", err)
			}
		}

		// 2. 删除物理文件
		sstPath := filepath.Join(m.sstDir, fmt.Sprintf("%06d.sst", fileNum))
		err := os.Remove(sstPath)
		if err != nil {
			// 删除失败只记录日志，不影响 compaction 流程
			// 后台垃圾回收器会重试
			m.logger.Warn("[Compaction] Failed to delete obsolete file",
				"file_number", fileNum,
				"error", err)
		} else {
			m.logger.Info("[Compaction] Deleted obsolete file",
				"file_number", fileNum)
		}
	}
}

// TriggerCompaction 手动触发一次 Compaction（遍历所有阶段）
func (m *CompactionManager) TriggerCompaction() error {
	picker := m.compactor.GetPicker()

	// 循环执行 4 个阶段
	for range 4 {
		version := m.versionSet.GetCurrent()
		if version == nil {
			return fmt.Errorf("no current version")
		}

		// 获取当前阶段的任务
		tasks := picker.PickCompaction(version)
		if len(tasks) == 0 {
			// 当前阶段没有任务，继续下一阶段
			continue
		}

		// 串行执行当前阶段的所有任务
		for _, task := range tasks {
			currentVersion := m.versionSet.GetCurrent()
			if err := m.DoCompactionWithVersion(task, currentVersion); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetStats 获取 Compaction 统计信息
func (m *CompactionManager) GetStats() *CompactionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &CompactionStats{
		TotalCompactions:   m.totalCompactions,
		LastCompactionTime: m.lastCompactionTime,
	}
}

// GetLevelStats 获取每层的统计信息
func (m *CompactionManager) GetLevelStats() []LevelStats {
	version := m.versionSet.GetCurrent()
	if version == nil {
		return nil
	}

	picker := m.compactor.GetPicker()
	stats := make([]LevelStats, NumLevels)

	for level := range NumLevels {
		files := version.GetLevel(level)
		totalSize := int64(0)
		for _, file := range files {
			totalSize += file.FileSize
		}

		stats[level] = LevelStats{
			Level:     level,
			FileCount: len(files),
			TotalSize: totalSize,
			Score:     picker.GetLevelScore(version, level),
		}
	}

	return stats
}

// backgroundGarbageCollection 后台垃圾回收循环
func (m *CompactionManager) backgroundGarbageCollection() {
	defer m.wg.Done()

	// 使用配置的间隔时间
	m.configMu.RLock()
	interval := m.gcInterval
	disabled := m.disableGC
	m.configMu.RUnlock()

	if disabled {
		return // 禁用垃圾回收，直接退出
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			// 检查配置是否被更新
			m.configMu.RLock()
			newInterval := m.gcInterval
			disabled := m.disableGC
			m.configMu.RUnlock()

			if disabled {
				return // 运行中被禁用，退出
			}

			// 如果间隔时间改变，重新创建 ticker
			if newInterval != interval {
				interval = newInterval
				ticker.Reset(interval)
			}

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
		m.logger.Error("[GC] Failed to scan SST directory", "error", err)
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
			// 使用配置的文件最小年龄（默认 1 分钟，可能正在 LogAndApply）
			m.configMu.RLock()
			minAge := m.gcFileMinAge
			m.configMu.RUnlock()

			fileInfo, err := os.Stat(sstPath)
			if err != nil {
				continue
			}
			if time.Since(fileInfo.ModTime()) < minAge {
				m.logger.Info("[GC] Skipping recently modified file",
					"file_number", fileNum,
					"age", time.Since(fileInfo.ModTime()),
					"min_age", minAge)
				continue
			}

			// 这是孤儿文件，删除它
			err = os.Remove(sstPath)
			if err != nil {
				m.logger.Warn("[GC] Failed to delete orphan file",
					"file_number", fileNum,
					"error", err)
			} else {
				m.logger.Info("[GC] Deleted orphan file",
					"file_number", fileNum)
				orphanCount++
			}
		}
	}

	// 4. 更新统计信息
	m.mu.Lock()
	m.lastGCTime = time.Now()
	m.totalOrphansFound += int64(orphanCount)
	totalOrphans := m.totalOrphansFound
	m.mu.Unlock()

	if orphanCount > 0 {
		m.logger.Info("[GC] Completed",
			"cleaned_up", orphanCount,
			"total_orphans", totalOrphans)
	}
}

// CleanupOrphanFiles 手动触发孤儿文件清理（可在启动时调用）
func (m *CompactionManager) CleanupOrphanFiles() {
	m.logger.Info("[GC] Manual cleanup triggered")
	m.collectOrphanFiles()
}
