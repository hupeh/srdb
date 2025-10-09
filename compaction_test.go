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
	schema, err := NewSchema("test", []Field{
		{Name: "value", Type: Int64},
	})
	if err != nil {
		t.Fatal(err)
	}

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
	schema, err := NewSchema("test", []Field{
		{Name: "value", Type: String},
	})
	if err != nil {
		t.Fatal(err)
	}

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
	schema, err := NewSchema("test", []Field{
		{Name: "value", Type: String},
	})
	if err != nil {
		b.Fatal(err)
	}

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
	schema, err := NewSchema("test", []Field{
		{Name: "id", Type: Int64},
		{Name: "name", Type: String},
		{Name: "data", Type: String},
		{Name: "timestamp", Type: Int64},
	})
	if err != nil {
		t.Fatal(err)
	}

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

// TestPickerStageRotation 测试 Picker 的阶段轮换机制
func TestPickerStageRotation(t *testing.T) {
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

	// 初始阶段应该是 L0
	if stage := picker.GetCurrentStage(); stage != 0 {
		t.Errorf("Initial stage should be 0 (L0), got %d", stage)
	}

	// 添加 L0 文件（触发 L0 compaction）
	edit := NewVersionEdit()
	for i := 0; i < 10; i++ {
		edit.AddFile(&FileMetadata{
			FileNumber: int64(i + 1),
			Level:      0,
			FileSize:   10 * 1024 * 1024, // 10MB each
			MinKey:     int64(i * 100),
			MaxKey:     int64((i+1)*100 - 1),
			RowCount:   100,
		})
	}
	edit.SetNextFileNumber(11)
	err = versionSet.LogAndApply(edit)
	if err != nil {
		t.Fatal(err)
	}

	version := versionSet.GetCurrent()

	// 第1次调用：应该返回 L0 任务，然后推进到 L1
	t.Log("=== 第1次调用 PickCompaction ===")
	tasks1 := picker.PickCompaction(version)
	if len(tasks1) == 0 {
		t.Error("Expected L0 tasks on first call")
	}
	for _, task := range tasks1 {
		if task.Level != 0 {
			t.Errorf("Expected L0 task, got L%d", task.Level)
		}
	}
	if stage := picker.GetCurrentStage(); stage != 1 {
		t.Errorf("After L0 tasks, stage should be 1 (L1), got %d", stage)
	}
	t.Logf("✓ Returned %d L0 tasks, stage advanced to L1", len(tasks1))

	// 第2次调用：应该尝试 Stage 1 (L0-upgrade，没有大文件)
	t.Log("=== 第2次调用 PickCompaction ===")
	tasks2 := picker.PickCompaction(version)
	if len(tasks2) == 0 {
		t.Log("✓ Stage 1 (L0-upgrade) has no tasks")
	}
	// 此时 stage 应该已经循环（尝试了 Stage 1→2→3→0...）
	if stage := picker.GetCurrentStage(); stage >= 0 {
		t.Logf("After trying, current stage is %d", stage)
	}

	// 现在添加 L1 文件
	edit2 := NewVersionEdit()
	for i := 0; i < 20; i++ {
		edit2.AddFile(&FileMetadata{
			FileNumber: int64(100 + i + 1),
			Level:      1,
			FileSize:   20 * 1024 * 1024, // 20MB each
			MinKey:     int64(i * 200),
			MaxKey:     int64((i+1)*200 - 1),
			RowCount:   200,
		})
	}
	edit2.SetNextFileNumber(121)
	err = versionSet.LogAndApply(edit2)
	if err != nil {
		t.Fatal(err)
	}

	version2 := versionSet.GetCurrent()

	// 现在可能需要多次调用才能到达 Stage 2 (L1-upgrade)
	// 因为要经过 Stage 1 (L0-upgrade) 和 Stage 0 (L0-merge)
	t.Log("=== 多次调用 PickCompaction 直到找到 L1 任务 ===")
	var tasks3 []*CompactionTask
	for i := 0; i < 8; i++ { // 最多尝试两轮（4个阶段×2）
		tasks3 = picker.PickCompaction(version2)
		if len(tasks3) > 0 && tasks3[0].Level == 1 {
			t.Logf("✓ Found %d L1 tasks after %d attempts", len(tasks3), i+1)
			break
		}
	}
	if len(tasks3) == 0 || tasks3[0].Level != 1 {
		t.Error("Expected to find L1 tasks within 8 attempts")
	}

	t.Log("=== Stage rotation test passed ===")
}

// TestPickerStageWithMultipleLevels 测试多层级同时有任务时的阶段轮换
func TestPickerStageWithMultipleLevels(t *testing.T) {
	tmpDir := t.TempDir()
	manifestDir := tmpDir

	versionSet, err := NewVersionSet(manifestDir)
	if err != nil {
		t.Fatal(err)
	}
	defer versionSet.Close()

	picker := NewPicker()

	// 同时添加 L0、L1、L2 文件
	edit := NewVersionEdit()

	// L0 小文件: 5 files × 10MB = 50MB (应该触发 Stage 0: L0-merge)
	for i := 0; i < 5; i++ {
		edit.AddFile(&FileMetadata{
			FileNumber: int64(i + 1),
			Level:      0,
			FileSize:   10 * 1024 * 1024,
			MinKey:     int64(i * 100),
			MaxKey:     int64((i+1)*100 - 1),
			RowCount:   100,
		})
	}

	// L0 大文件: 5 files × 40MB = 200MB (应该触发 Stage 1: L0-upgrade)
	for i := 0; i < 5; i++ {
		edit.AddFile(&FileMetadata{
			FileNumber: int64(10 + i + 1),
			Level:      0,
			FileSize:   40 * 1024 * 1024,
			MinKey:     int64((i + 5) * 100),
			MaxKey:     int64((i+6)*100 - 1),
			RowCount:   100,
		})
	}

	// L1: 20 files × 20MB = 400MB (应该触发 Stage 2: L1-upgrade，256MB阈值)
	for i := 0; i < 20; i++ {
		edit.AddFile(&FileMetadata{
			FileNumber: int64(100 + i + 1),
			Level:      1,
			FileSize:   20 * 1024 * 1024,
			MinKey:     int64(i * 200),
			MaxKey:     int64((i+1)*200 - 1),
			RowCount:   200,
		})
	}

	// L2: 10 files × 150MB = 1500MB (应该触发 Stage 3: L2-upgrade，1GB阈值)
	for i := 0; i < 10; i++ {
		edit.AddFile(&FileMetadata{
			FileNumber: int64(200 + i + 1),
			Level:      2,
			FileSize:   150 * 1024 * 1024,
			MinKey:     int64(i * 300),
			MaxKey:     int64((i+1)*300 - 1),
			RowCount:   300,
		})
	}

	edit.SetNextFileNumber(301)
	err = versionSet.LogAndApply(edit)
	if err != nil {
		t.Fatal(err)
	}

	version := versionSet.GetCurrent()

	// 验证阶段按顺序执行：Stage 0→1→2→3→0→1→2→3
	expectedStages := []struct {
		stage int
		name  string
		level int
	}{
		{0, "L0-merge", 0},
		{1, "L0-upgrade", 0},
		{2, "L1-upgrade", 1},
		{3, "L2-upgrade", 2},
		{0, "L0-merge", 0},
		{1, "L0-upgrade", 0},
		{2, "L1-upgrade", 1},
		{3, "L2-upgrade", 2},
	}

	for i, expected := range expectedStages {
		t.Logf("=== 第%d次调用 PickCompaction (期望 Stage %d: %s) ===", i+1, expected.stage, expected.name)
		tasks := picker.PickCompaction(version)

		if len(tasks) == 0 {
			t.Errorf("Call %d: Expected tasks from Stage %d (%s), got no tasks", i+1, expected.stage, expected.name)
			continue
		}

		actualLevel := tasks[0].Level
		if actualLevel != expected.level {
			t.Errorf("Call %d: Expected L%d tasks, got L%d tasks", i+1, expected.level, actualLevel)
		} else {
			t.Logf("✓ Call %d: Got %d tasks from L%d (Stage %d: %s) as expected",
				i+1, len(tasks), actualLevel, expected.stage, expected.name)
		}
	}

	t.Log("=== Multi-level stage rotation test passed ===")
}

// TestPickL0MergeContinuity 测试 L0 合并任务的连续性
func TestPickL0MergeContinuity(t *testing.T) {
	tmpDir := t.TempDir()
	manifestDir := tmpDir

	versionSet, err := NewVersionSet(manifestDir)
	if err != nil {
		t.Fatal(err)
	}
	defer versionSet.Close()

	picker := NewPicker()

	// 创建混合大小的文件：小-大-小-小
	// 这是触发 bug 的场景
	edit := NewVersionEdit()

	// 文件1: 29MB (小文件)
	edit.AddFile(&FileMetadata{
		FileNumber: 1,
		Level:      0,
		FileSize:   29 * 1024 * 1024,
		MinKey:     1,
		MaxKey:     100,
		RowCount:   100,
	})

	// 文件2: 36MB (大文件)
	edit.AddFile(&FileMetadata{
		FileNumber: 2,
		Level:      0,
		FileSize:   36 * 1024 * 1024,
		MinKey:     101,
		MaxKey:     200,
		RowCount:   100,
	})

	// 文件3: 8MB (小文件)
	edit.AddFile(&FileMetadata{
		FileNumber: 3,
		Level:      0,
		FileSize:   8 * 1024 * 1024,
		MinKey:     201,
		MaxKey:     300,
		RowCount:   100,
	})

	// 文件4: 15MB (小文件)
	edit.AddFile(&FileMetadata{
		FileNumber: 4,
		Level:      0,
		FileSize:   15 * 1024 * 1024,
		MinKey:     301,
		MaxKey:     400,
		RowCount:   100,
	})

	edit.SetNextFileNumber(5)
	err = versionSet.LogAndApply(edit)
	if err != nil {
		t.Fatal(err)
	}

	version := versionSet.GetCurrent()

	// 测试 Stage 0: L0 合并任务
	t.Log("=== 测试 Stage 0: L0 合并 ===")
	tasks := picker.pickL0MergeTasks(version)

	if len(tasks) == 0 {
		t.Fatal("Expected L0 merge tasks")
	}

	t.Logf("找到 %d 个合并任务", len(tasks))

	// 验证任务：应该只有1个任务，包含文件3和文件4
	// 文件1是单个小文件，不合并（len > 1 才合并）
	// 文件2是大文件，跳过
	// 文件3+文件4是连续的2个小文件，应该合并
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
		for i, task := range tasks {
			t.Logf("Task %d: %d files", i+1, len(task.InputFiles))
			for _, f := range task.InputFiles {
				t.Logf("  - File %d", f.FileNumber)
			}
		}
	}
	task1 := tasks[0]
	if len(task1.InputFiles) != 2 {
		t.Errorf("Task 1: expected 2 files, got %d", len(task1.InputFiles))
	}
	if task1.InputFiles[0].FileNumber != 3 || task1.InputFiles[1].FileNumber != 4 {
		t.Errorf("Task 1: expected files 3,4, got %d,%d",
			task1.InputFiles[0].FileNumber, task1.InputFiles[1].FileNumber)
	}
	t.Logf("✓ 合并任务: 文件3+文件4 (连续的2个小文件)")
	t.Logf("✓ 文件1 (单个小文件) 不合并，留给升级阶段")

	// 验证 seq 范围连续性
	// 任务1: seq 201-400 (文件3+文件4)
	// 文件1（seq 1-100, 单个小文件）留给升级阶段
	// 文件2（seq 101-200, 大文件）留给 Stage 1
	if task1.InputFiles[0].MinKey != 201 || task1.InputFiles[1].MaxKey != 400 {
		t.Errorf("Task 1 seq range incorrect: [%d, %d]",
			task1.InputFiles[0].MinKey, task1.InputFiles[1].MaxKey)
	}
	t.Logf("✓ Seq 范围正确：任务1 [201-400]")

	// 测试 Stage 1: L0 升级任务
	t.Log("=== 测试 Stage 1: L0 升级 ===")
	upgradeTasks := picker.pickL0UpgradeTasks(version)

	if len(upgradeTasks) == 0 {
		t.Fatal("Expected L0 upgrade tasks")
	}

	// 应该有1个任务：以文件2（大文件）为中心，搭配周围的小文件
	// 文件2向左收集文件1，向右收集文件3和文件4
	// 总共：文件1 (29MB) + 文件2 (36MB) + 文件3 (8MB) + 文件4 (15MB) = 88MB
	if len(upgradeTasks) != 1 {
		t.Errorf("Expected 1 upgrade task, got %d", len(upgradeTasks))
	}
	upgradeTask := upgradeTasks[0]

	// 应该包含所有4个文件
	if len(upgradeTask.InputFiles) != 4 {
		t.Errorf("Upgrade task: expected 4 files, got %d", len(upgradeTask.InputFiles))
		for i, f := range upgradeTask.InputFiles {
			t.Logf("  File %d: %d", i+1, f.FileNumber)
		}
	}

	// 验证文件顺序：1, 2, 3, 4
	expectedFiles := []int64{1, 2, 3, 4}
	for i, expected := range expectedFiles {
		if upgradeTask.InputFiles[i].FileNumber != expected {
			t.Errorf("Upgrade task file %d: expected %d, got %d",
				i, expected, upgradeTask.InputFiles[i].FileNumber)
		}
	}

	if upgradeTask.OutputLevel != 1 {
		t.Errorf("Upgrade task: expected OutputLevel 1, got %d", upgradeTask.OutputLevel)
	}
	t.Logf("✓ 升级任务: 文件1+文件2+文件3+文件4 (以大文件为中心，搭配周围小文件) → L1")

	t.Log("=== 连续性测试通过 ===")
}

// TestPickL0UpgradeContinuity 测试 L0 升级任务的连续性
func TestPickL0UpgradeContinuity(t *testing.T) {
	tmpDir := t.TempDir()
	manifestDir := tmpDir

	versionSet, err := NewVersionSet(manifestDir)
	if err != nil {
		t.Fatal(err)
	}
	defer versionSet.Close()

	picker := NewPicker()

	// 创建混合大小的文件：大-小-大-大
	edit := NewVersionEdit()

	// 文件1: 40MB (大文件)
	edit.AddFile(&FileMetadata{
		FileNumber: 1,
		Level:      0,
		FileSize:   40 * 1024 * 1024,
		MinKey:     1,
		MaxKey:     100,
		RowCount:   100,
	})

	// 文件2: 20MB (小文件)
	edit.AddFile(&FileMetadata{
		FileNumber: 2,
		Level:      0,
		FileSize:   20 * 1024 * 1024,
		MinKey:     101,
		MaxKey:     200,
		RowCount:   100,
	})

	// 文件3: 50MB (大文件)
	edit.AddFile(&FileMetadata{
		FileNumber: 3,
		Level:      0,
		FileSize:   50 * 1024 * 1024,
		MinKey:     201,
		MaxKey:     300,
		RowCount:   100,
	})

	// 文件4: 45MB (大文件)
	edit.AddFile(&FileMetadata{
		FileNumber: 4,
		Level:      0,
		FileSize:   45 * 1024 * 1024,
		MinKey:     301,
		MaxKey:     400,
		RowCount:   100,
	})

	edit.SetNextFileNumber(5)
	err = versionSet.LogAndApply(edit)
	if err != nil {
		t.Fatal(err)
	}

	version := versionSet.GetCurrent()

	// 测试 L0 升级任务
	t.Log("=== 测试 L0 升级任务连续性 ===")
	tasks := picker.pickL0UpgradeTasks(version)

	if len(tasks) == 0 {
		t.Fatal("Expected L0 upgrade tasks")
	}

	t.Logf("找到 %d 个升级任务", len(tasks))

	// 验证任务1：应该包含所有4个文件（以大文件为锚点，搭配周围文件）
	// 文件1（大文件）作为锚点 → 向左无文件 → 向右收集文件2（小）+文件3（大）+文件4（大）
	// 总大小：40+20+50+45 = 155MB < 256MB，符合 L1 限制
	task1 := tasks[0]
	expectedFileCount := 4
	if len(task1.InputFiles) != expectedFileCount {
		t.Errorf("Task 1: expected %d files, got %d", expectedFileCount, len(task1.InputFiles))
		for i, f := range task1.InputFiles {
			t.Logf("  File %d: %d", i+1, f.FileNumber)
		}
	}

	// 验证文件顺序：1, 2, 3, 4
	expectedFiles := []int64{1, 2, 3, 4}
	for i, expected := range expectedFiles {
		if i >= len(task1.InputFiles) {
			break
		}
		if task1.InputFiles[i].FileNumber != expected {
			t.Errorf("Task 1 file %d: expected %d, got %d",
				i, expected, task1.InputFiles[i].FileNumber)
		}
	}
	t.Logf("✓ Task 1: 文件1+文件2+文件3+文件4 (以大文件为锚点，搭配周围文件，总155MB < 256MB)")

	// 只应该有1个任务（所有文件都被收集了）
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task (all files collected), got %d", len(tasks))
		for i, task := range tasks {
			t.Logf("Task %d: %d files", i+1, len(task.InputFiles))
			for _, f := range task.InputFiles {
				t.Logf("  - File %d", f.FileNumber)
			}
		}
	}

	// 验证所有任务的 OutputLevel 都是 1
	for i, task := range tasks {
		if task.OutputLevel != 1 {
			t.Errorf("Task %d: expected OutputLevel 1, got %d", i+1, task.OutputLevel)
		}
	}
	t.Logf("✓ 所有任务都升级到 L1")

	t.Log("=== 升级任务连续性测试通过 ===")
}
