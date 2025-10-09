package srdb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompactionBasic(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	sstDir := filepath.Join(tmpDir, "sst")
	manifestDir := tmpDir

	err := os.MkdirAll(sstDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// 创建 Schema
	schema := NewSchema("test", []Field{
		{Name: "value", Type: FieldTypeInt64},
	})

	// 创建 VersionSet
	versionSet, err := NewVersionSet(manifestDir)
	if err != nil {
		t.Fatal(err)
	}
	defer versionSet.Close()

	// 创建 SST Manager
	sstMgr, err := NewSSTableManager(sstDir)
	if err != nil {
		t.Fatal(err)
	}
	defer sstMgr.Close()

	// 设置 Schema
	sstMgr.SetSchema(schema)

	// 创建测试数据
	rows1 := make([]*SSTableRow, 100)
	for i := range 100 {
		rows1[i] = &SSTableRow{
			Seq:  int64(i),
			Time: 1000,
			Data: map[string]any{"value": i},
		}
	}

	// 创建第一个 SST 文件
	reader1, err := sstMgr.CreateSST(1, rows1)
	if err != nil {
		t.Fatal(err)
	}

	// 添加到 Version
	edit1 := NewVersionEdit()
	edit1.AddFile(&FileMetadata{
		FileNumber: 1,
		Level:      0,
		FileSize:   1024,
		MinKey:     0,
		MaxKey:     99,
		RowCount:   100,
	})
	nextFileNum := int64(2)
	edit1.SetNextFileNumber(nextFileNum)

	err = versionSet.LogAndApply(edit1)
	if err != nil {
		t.Fatal(err)
	}

	// 验证 Version
	version := versionSet.GetCurrent()
	if version.GetLevelFileCount(0) != 1 {
		t.Errorf("Expected 1 file in L0, got %d", version.GetLevelFileCount(0))
	}

	// 创建 Compaction Manager
	compactionMgr := NewCompactionManager(sstDir, versionSet, sstMgr)
	compactionMgr.SetSchema(schema)

	// 创建更多文件触发 Compaction
	for i := 1; i < 5; i++ {
		rows := make([]*SSTableRow, 50)
		for j := range 50 {
			rows[j] = &SSTableRow{
				Seq:  int64(i*100 + j),
				Time: int64(1000 + i),
				Data: map[string]any{"value": i*100 + j},
			}
		}

		_, err := sstMgr.CreateSST(int64(i+1), rows)
		if err != nil {
			t.Fatal(err)
		}

		edit := NewVersionEdit()
		edit.AddFile(&FileMetadata{
			FileNumber: int64(i + 1),
			Level:      0,
			FileSize:   512,
			MinKey:     int64(i * 100),
			MaxKey:     int64(i*100 + 49),
			RowCount:   50,
		})
		nextFileNum := int64(i + 2)
		edit.SetNextFileNumber(nextFileNum)

		err = versionSet.LogAndApply(edit)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 验证 L0 有 5 个文件
	version = versionSet.GetCurrent()
	if version.GetLevelFileCount(0) != 5 {
		t.Errorf("Expected 5 files in L0, got %d", version.GetLevelFileCount(0))
	}

	// 检查是否需要 Compaction
	picker := compactionMgr.GetPicker()
	if !picker.ShouldCompact(version) {
		t.Error("Expected compaction to be needed")
	}

	// 获取 Compaction 任务
	tasks := picker.PickCompaction(version)
	if len(tasks) == 0 {
		t.Fatal("Expected compaction task")
	}

	task := tasks[0] // 获取第一个任务（优先级最高）

	if task.Level != 0 {
		t.Errorf("Expected L0 compaction, got L%d", task.Level)
	}

	// 注意：L0 compaction 任务的 OutputLevel 设为 0（建议层级）
	// 实际层级由 determineLevel 根据合并后的文件大小决定
	if task.OutputLevel != 0 {
		t.Errorf("Expected output to L0 (suggested), got L%d", task.OutputLevel)
	}

	t.Logf("Found %d compaction tasks", len(tasks))
	t.Logf("First task: L%d -> L%d, %d files (determineLevel will decide actual level)", task.Level, task.OutputLevel, len(task.InputFiles))

	// 清理
	reader1.Close()
}

func TestPickerLevelScore(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	manifestDir := tmpDir

	// 创建 VersionSet
	versionSet, err := NewVersionSet(manifestDir)
	if err != nil {
		t.Fatal(err)
	}
	defer versionSet.Close()

	// 创建 Picker
	picker := NewPicker()

	// 添加一些文件到 L0
	edit := NewVersionEdit()
	for i := range 3 {
		edit.AddFile(&FileMetadata{
			FileNumber: int64(i + 1),
			Level:      0,
			FileSize:   1024 * 1024, // 1MB
			MinKey:     int64(i * 100),
			MaxKey:     int64((i+1)*100 - 1),
			RowCount:   100,
		})
	}
	nextFileNum := int64(4)
	edit.SetNextFileNumber(nextFileNum)

	err = versionSet.LogAndApply(edit)
	if err != nil {
		t.Fatal(err)
	}

	version := versionSet.GetCurrent()

	// 计算 L0 的得分
	score := picker.GetLevelScore(version, 0)

	// L0 有 3 个文件，每个 1MB，总共 3MB
	// 下一级（L1）的限制是 256MB
	// 得分应该是 3MB / 256MB = 0.01171875
	totalSize := int64(3 * 1024 * 1024) // 3MB
	expectedScore := float64(totalSize) / float64(level1SizeLimit)

	t.Logf("L0 score: %.4f (files: %d, total: %d bytes, next level limit: %d)",
		score, version.GetLevelFileCount(0), totalSize, level1SizeLimit)

	if score != expectedScore {
		t.Errorf("Expected L0 score %.4f, got %.4f", expectedScore, score)
	}
}

func TestCompactionMerge(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	sstDir := filepath.Join(tmpDir, "sst")
	manifestDir := tmpDir

	err := os.MkdirAll(sstDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// 创建 Schema
	schema := NewSchema("test", []Field{
		{Name: "value", Type: FieldTypeString},
	})

	// 创建 VersionSet
	versionSet, err := NewVersionSet(manifestDir)
	if err != nil {
		t.Fatal(err)
	}
	defer versionSet.Close()

	// 创建 SST Manager
	sstMgr, err := NewSSTableManager(sstDir)
	if err != nil {
		t.Fatal(err)
	}
	defer sstMgr.Close()

	// 设置 Schema
	sstMgr.SetSchema(schema)

	// 创建两个有重叠 key 的 SST 文件
	rows1 := []*SSTableRow{
		{Seq: 1, Time: 1000, Data: map[string]any{"value": "old"}},
		{Seq: 2, Time: 1000, Data: map[string]any{"value": "old"}},
	}

	rows2 := []*SSTableRow{
		{Seq: 1, Time: 2000, Data: map[string]any{"value": "new"}}, // 更新
		{Seq: 3, Time: 2000, Data: map[string]any{"value": "new"}},
	}

	reader1, err := sstMgr.CreateSST(1, rows1)
	if err != nil {
		t.Fatal(err)
	}
	defer reader1.Close()

	reader2, err := sstMgr.CreateSST(2, rows2)
	if err != nil {
		t.Fatal(err)
	}
	defer reader2.Close()

	// 添加到 Version
	edit := NewVersionEdit()
	edit.AddFile(&FileMetadata{
		FileNumber: 1,
		Level:      0,
		FileSize:   512,
		MinKey:     1,
		MaxKey:     2,
		RowCount:   2,
	})
	edit.AddFile(&FileMetadata{
		FileNumber: 2,
		Level:      0,
		FileSize:   512,
		MinKey:     1,
		MaxKey:     3,
		RowCount:   2,
	})
	nextFileNum := int64(3)
	edit.SetNextFileNumber(nextFileNum)

	err = versionSet.LogAndApply(edit)
	if err != nil {
		t.Fatal(err)
	}

	// 创建 Compactor
	compactor := NewCompactor(sstDir, versionSet)
	compactor.SetSchema(schema)

	// 创建 Compaction 任务
	version := versionSet.GetCurrent()
	task := &CompactionTask{
		Level:       0,
		InputFiles:  version.GetLevel(0),
		OutputLevel: 1,
	}

	// 执行 Compaction
	resultEdit, err := compactor.DoCompaction(task, version)
	if err != nil {
		t.Fatal(err)
	}

	// 验证结果
	if len(resultEdit.DeletedFiles) != 2 {
		t.Errorf("Expected 2 deleted files, got %d", len(resultEdit.DeletedFiles))
	}

	if len(resultEdit.AddedFiles) == 0 {
		t.Error("Expected at least 1 new file")
	}

	t.Logf("Compaction result: deleted %d files, added %d files", len(resultEdit.DeletedFiles), len(resultEdit.AddedFiles))

	// 验证新文件在 L1
	for _, file := range resultEdit.AddedFiles {
		if file.Level != 1 {
			t.Errorf("Expected new file in L1, got L%d", file.Level)
		}
		t.Logf("New file: %d, L%d, rows: %d, key range: [%d, %d]",
			file.FileNumber, file.Level, file.RowCount, file.MinKey, file.MaxKey)
	}
}

func BenchmarkCompaction(b *testing.B) {
	// 创建临时目录
	tmpDir := b.TempDir()
	sstDir := filepath.Join(tmpDir, "sst")
	manifestDir := tmpDir

	err := os.MkdirAll(sstDir, 0755)
	if err != nil {
		b.Fatal(err)
	}

	// 创建 Schema
	schema := NewSchema("test", []Field{
		{Name: "value", Type: FieldTypeString},
	})

	// 创建 VersionSet
	versionSet, err := NewVersionSet(manifestDir)
	if err != nil {
		b.Fatal(err)
	}
	defer versionSet.Close()

	// 创建 SST Manager
	sstMgr, err := NewSSTableManager(sstDir)
	if err != nil {
		b.Fatal(err)
	}
	defer sstMgr.Close()

	// 设置 Schema
	sstMgr.SetSchema(schema)

	// 创建测试数据
	const numFiles = 5
	const rowsPerFile = 1000

	for i := range numFiles {
		rows := make([]*SSTableRow, rowsPerFile)
		for j := range rowsPerFile {
			rows[j] = &SSTableRow{
				Seq:  int64(i*rowsPerFile + j),
				Time: int64(1000 + i),
				Data: map[string]any{
					"value": fmt.Sprintf("data-%d-%d", i, j),
				},
			}
		}

		reader, err := sstMgr.CreateSST(int64(i+1), rows)
		if err != nil {
			b.Fatal(err)
		}
		reader.Close()

		edit := NewVersionEdit()
		edit.AddFile(&FileMetadata{
			FileNumber: int64(i + 1),
			Level:      0,
			FileSize:   10240,
			MinKey:     int64(i * rowsPerFile),
			MaxKey:     int64((i+1)*rowsPerFile - 1),
			RowCount:   rowsPerFile,
		})
		nextFileNum := int64(i + 2)
		edit.SetNextFileNumber(nextFileNum)

		err = versionSet.LogAndApply(edit)
		if err != nil {
			b.Fatal(err)
		}
	}

	// 创建 Compactor
	compactor := NewCompactor(sstDir, versionSet)
	compactor.SetSchema(schema)
	version := versionSet.GetCurrent()

	task := &CompactionTask{
		Level:       0,
		InputFiles:  version.GetLevel(0),
		OutputLevel: 1,
	}

	for b.Loop() {
		_, err := compactor.DoCompaction(task, version)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestCompactionQueryOrder 测试 compaction 后查询结果的排序
func TestCompactionQueryOrder(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建 Schema - 包含多个字段以增加数据大小
	schema := NewSchema("test", []Field{
		{Name: "id", Type: FieldTypeInt64},
		{Name: "name", Type: FieldTypeString},
		{Name: "data", Type: FieldTypeString},
		{Name: "timestamp", Type: FieldTypeInt64},
	})

	// 打开 Table (使用较小的 MemTable 触发频繁 flush)
	table, err := OpenTable(&TableOptions{
		Dir:          tmpDir,
		MemTableSize: 2 * 1024 * 1024, // 2MB MemTable
		Name:         schema.Name, Fields: schema.Fields,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()

	t.Logf("开始插入 4000 条数据...")

	// 插入 4000 条数据，每条数据大小在 2KB-1MB 之间
	for i := range 4000 {
		// 生成 2KB 到 1MB 的随机数据
		dataSize := 2*1024 + (i % (1024*1024 - 2*1024)) // 2KB ~ 1MB
		largeData := make([]byte, dataSize)
		for j := range largeData {
			largeData[j] = byte('A' + (j % 26))
		}

		err := table.Insert(map[string]any{
			"id":        int64(i),
			"name":      fmt.Sprintf("user_%d", i),
			"data":      string(largeData),
			"timestamp": int64(1000000 + i),
		})
		if err != nil {
			t.Fatal(err)
		}

		if (i+1)%500 == 0 {
			t.Logf("已插入 %d 条数据", i+1)
		}
	}

	t.Logf("插入完成，等待后台 compaction...")

	// 等待一段时间让后台 compaction 有机会运行
	// 后台 compaction 每 10 秒检查一次，所以需要等待至少 12 秒
	time.Sleep(12 * time.Second)

	t.Logf("开始查询所有数据...")

	// 查询所有数据
	rows, err := table.Query().Rows()
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	// 验证顺序和数据完整性
	var lastSeq int64 = 0
	count := 0
	expectedIDs := make(map[int64]bool) // 用于验证所有 ID 都存在

	for rows.Next() {
		row := rows.Row()
		data := row.Data()
		currentSeq := data["_seq"].(int64)

		// 验证顺序
		if currentSeq <= lastSeq {
			t.Errorf("Query results NOT in order: got seq %d after seq %d", currentSeq, lastSeq)
		}

		// 验证数据完整性
		id, ok := data["id"].(int64)
		if !ok {
			// 尝试其他类型
			if idFloat, ok2 := data["id"].(float64); ok2 {
				id = int64(idFloat)
				expectedIDs[id] = true
			} else {
				t.Errorf("Seq %d: missing or invalid id field, actual type: %T, value: %v",
					currentSeq, data["id"], data["id"])
			}
		} else {
			expectedIDs[id] = true
		}

		// 验证 name 字段
		name, ok := data["name"].(string)
		if !ok || name != fmt.Sprintf("user_%d", id) {
			t.Errorf("Seq %d: invalid name field, expected 'user_%d', got '%v'", currentSeq, id, name)
		}

		// 验证 data 字段存在且不为空
		dataStr, ok := data["data"].(string)
		if !ok || len(dataStr) < 2*1024 {
			t.Errorf("Seq %d: invalid data field size", currentSeq)
		}

		lastSeq = currentSeq
		count++
	}

	if count != 4000 {
		t.Errorf("Expected 4000 rows, got %d", count)
	}

	// 验证所有 ID 都存在
	for i := range int64(4000) {
		if !expectedIDs[i] {
			t.Errorf("Missing ID: %d", i)
		}
	}

	t.Logf("✓ 查询返回 %d 条记录，顺序正确 (seq 1→%d)", count, lastSeq)
	t.Logf("✓ 所有数据完整性验证通过")

	// 输出 compaction 统计信息
	stats := table.GetCompactionManager().GetLevelStats()
	t.Logf("Compaction 统计:")
	for _, levelStat := range stats {
		level := levelStat.Level
		fileCount := levelStat.FileCount
		totalSize := levelStat.TotalSize
		if fileCount > 0 {
			t.Logf("  L%d: %d 个文件, %.2f MB", level, fileCount, float64(totalSize)/(1024*1024))
		}
	}
}
