package compaction

import (
	"code.tczkiot.com/srdb/manifest"
	"code.tczkiot.com/srdb/sst"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Compactor 负责执行 Compaction
type Compactor struct {
	sstDir     string
	picker     *Picker
	versionSet *manifest.VersionSet
	mu         sync.Mutex
}

// NewCompactor 创建新的 Compactor
func NewCompactor(sstDir string, versionSet *manifest.VersionSet) *Compactor {
	return &Compactor{
		sstDir:     sstDir,
		picker:     NewPicker(),
		versionSet: versionSet,
	}
}

// GetPicker 获取 Picker
func (c *Compactor) GetPicker() *Picker {
	return c.picker
}

// DoCompaction 执行一次 Compaction
// 返回: VersionEdit (记录变更), error
func (c *Compactor) DoCompaction(task *CompactionTask, version *manifest.Version) (*manifest.VersionEdit, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if task == nil {
		return nil, fmt.Errorf("compaction task is nil")
	}

	// 0. 验证输入文件是否存在（防止并发 compaction 导致的竞态）
	existingInputFiles := make([]*manifest.FileMetadata, 0, len(task.InputFiles))
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
	var existingOutputFiles []*manifest.FileMetadata
	var missingOutputFiles []*manifest.FileMetadata
	if len(outputFiles) > 0 {
		// 验证输出文件是否存在
		existingOutputFiles = make([]*manifest.FileMetadata, 0, len(outputFiles))
		missingOutputFiles = make([]*manifest.FileMetadata, 0)
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
	newFiles, err := c.writeOutputFiles(mergedRows, task.OutputLevel, version, avgRowSize)
	if err != nil {
		return nil, fmt.Errorf("write output files: %w", err)
	}

	// 5. 创建 VersionEdit
	edit := manifest.NewVersionEdit()

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

	// 添加新文件
	for _, file := range newFiles {
		edit.AddFile(file)
	}

	// 持久化当前的文件编号计数器（关键修复：防止重启后文件编号重用）
	edit.SetNextFileNumber(c.versionSet.GetNextFileNumber())

	return edit, nil
}

// readInputFiles 读取输入文件的所有行
// 注意：调用者必须确保传入的文件都存在，否则会返回错误
func (c *Compactor) readInputFiles(files []*manifest.FileMetadata) ([]*sst.Row, error) {
	var allRows []*sst.Row

	for _, file := range files {
		sstPath := filepath.Join(c.sstDir, fmt.Sprintf("%06d.sst", file.FileNumber))

		reader, err := sst.NewReader(sstPath)
		if err != nil {
			return nil, fmt.Errorf("open sst %d: %w", file.FileNumber, err)
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
func (c *Compactor) getOverlappingFiles(version *manifest.Version, level int, rows []*sst.Row) []*manifest.FileMetadata {
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
	var overlapping []*manifest.FileMetadata
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
func (c *Compactor) mergeRows(rows []*sst.Row) []*sst.Row {
	if len(rows) == 0 {
		return rows
	}

	// 按 Seq 排序
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Seq < rows[j].Seq
	})

	// 去重：保留相同 Seq 的最新记录 (Timestamp 最大的)
	merged := make([]*sst.Row, 0, len(rows))
	var lastRow *sst.Row

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
func (c *Compactor) calculateAvgRowSize(inputFiles []*manifest.FileMetadata, outputFiles []*manifest.FileMetadata) int64 {
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
func (c *Compactor) writeOutputFiles(rows []*sst.Row, level int, version *manifest.Version, avgRowSize int64) ([]*manifest.FileMetadata, error) {
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

	var newFiles []*manifest.FileMetadata
	var currentRows []*sst.Row
	var currentSize int64

	for _, row := range rows {
		// 使用平均行大小估算（基于输入文件的统计信息）
		rowSize := avgRowSize

		// 如果当前文件大小超过目标，写入文件
		if currentSize > 0 && currentSize+rowSize > targetFileSize {
			file, err := c.writeFile(currentRows, level, version)
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
		file, err := c.writeFile(currentRows, level, version)
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
func (c *Compactor) writeFile(rows []*sst.Row, level int, version *manifest.Version) (*manifest.FileMetadata, error) {
	// 从 VersionSet 分配新的文件编号
	fileNumber := c.versionSet.AllocateFileNumber()
	sstPath := filepath.Join(c.sstDir, fmt.Sprintf("%06d.sst", fileNumber))

	// 创建文件
	file, err := os.Create(sstPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	writer := sst.NewWriter(file)

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
	metadata := &manifest.FileMetadata{
		FileNumber: fileNumber,
		Level:      level,
		FileSize:   fileInfo.Size(),
		MinKey:     rows[0].Seq,
		MaxKey:     rows[len(rows)-1].Seq,
		RowCount:   int64(len(rows)),
	}

	return metadata, nil
}
