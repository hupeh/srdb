package compaction

import (
	"code.tczkiot.com/srdb/manifest"
	"code.tczkiot.com/srdb/sst"
	"fmt"
	"os"
	"path/filepath"
	"testing"
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

	// 创建 VersionSet
	versionSet, err := manifest.NewVersionSet(manifestDir)
	if err != nil {
		t.Fatal(err)
	}
	defer versionSet.Close()

	// 创建 SST Manager
	sstMgr, err := sst.NewManager(sstDir)
	if err != nil {
		t.Fatal(err)
	}
	defer sstMgr.Close()

	// 创建测试数据
	rows1 := make([]*sst.Row, 100)
	for i := 0; i < 100; i++ {
		rows1[i] = &sst.Row{
			Seq:  int64(i),
			Time: 1000,
			Data: map[string]interface{}{"value": i},
		}
	}

	// 创建第一个 SST 文件
	reader1, err := sstMgr.CreateSST(1, rows1)
	if err != nil {
		t.Fatal(err)
	}

	// 添加到 Version
	edit1 := manifest.NewVersionEdit()
	edit1.AddFile(&manifest.FileMetadata{
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
	compactionMgr := NewManager(sstDir, versionSet)

	// 创建更多文件触发 Compaction
	for i := 1; i < 5; i++ {
		rows := make([]*sst.Row, 50)
		for j := 0; j < 50; j++ {
			rows[j] = &sst.Row{
				Seq:  int64(i*100 + j),
				Time: int64(1000 + i),
				Data: map[string]interface{}{"value": i*100 + j},
			}
		}

		_, err := sstMgr.CreateSST(int64(i+1), rows)
		if err != nil {
			t.Fatal(err)
		}

		edit := manifest.NewVersionEdit()
		edit.AddFile(&manifest.FileMetadata{
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

	if task.OutputLevel != 1 {
		t.Errorf("Expected output to L1, got L%d", task.OutputLevel)
	}

	t.Logf("Found %d compaction tasks", len(tasks))
	t.Logf("First task: L%d -> L%d, %d files", task.Level, task.OutputLevel, len(task.InputFiles))

	// 清理
	reader1.Close()
}

func TestPickerLevelScore(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	manifestDir := tmpDir

	// 创建 VersionSet
	versionSet, err := manifest.NewVersionSet(manifestDir)
	if err != nil {
		t.Fatal(err)
	}
	defer versionSet.Close()

	// 创建 Picker
	picker := NewPicker()

	// 添加一些文件到 L0
	edit := manifest.NewVersionEdit()
	for i := 0; i < 3; i++ {
		edit.AddFile(&manifest.FileMetadata{
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
	t.Logf("L0 score: %.2f (files: %d, limit: %d)", score, version.GetLevelFileCount(0), picker.levelFileLimits[0])

	// L0 有 3 个文件，限制是 4，得分应该是 0.75
	expectedScore := 3.0 / 4.0
	if score != expectedScore {
		t.Errorf("Expected L0 score %.2f, got %.2f", expectedScore, score)
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

	// 创建 VersionSet
	versionSet, err := manifest.NewVersionSet(manifestDir)
	if err != nil {
		t.Fatal(err)
	}
	defer versionSet.Close()

	// 创建 SST Manager
	sstMgr, err := sst.NewManager(sstDir)
	if err != nil {
		t.Fatal(err)
	}
	defer sstMgr.Close()

	// 创建两个有重叠 key 的 SST 文件
	rows1 := []*sst.Row{
		{Seq: 1, Time: 1000, Data: map[string]interface{}{"value": "old"}},
		{Seq: 2, Time: 1000, Data: map[string]interface{}{"value": "old"}},
	}

	rows2 := []*sst.Row{
		{Seq: 1, Time: 2000, Data: map[string]interface{}{"value": "new"}}, // 更新
		{Seq: 3, Time: 2000, Data: map[string]interface{}{"value": "new"}},
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
	edit := manifest.NewVersionEdit()
	edit.AddFile(&manifest.FileMetadata{
		FileNumber: 1,
		Level:      0,
		FileSize:   512,
		MinKey:     1,
		MaxKey:     2,
		RowCount:   2,
	})
	edit.AddFile(&manifest.FileMetadata{
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

	// 创建 VersionSet
	versionSet, err := manifest.NewVersionSet(manifestDir)
	if err != nil {
		b.Fatal(err)
	}
	defer versionSet.Close()

	// 创建 SST Manager
	sstMgr, err := sst.NewManager(sstDir)
	if err != nil {
		b.Fatal(err)
	}
	defer sstMgr.Close()

	// 创建测试数据
	const numFiles = 5
	const rowsPerFile = 1000

	for i := 0; i < numFiles; i++ {
		rows := make([]*sst.Row, rowsPerFile)
		for j := 0; j < rowsPerFile; j++ {
			rows[j] = &sst.Row{
				Seq:  int64(i*rowsPerFile + j),
				Time: int64(1000 + i),
				Data: map[string]interface{}{
					"value": fmt.Sprintf("data-%d-%d", i, j),
				},
			}
		}

		reader, err := sstMgr.CreateSST(int64(i+1), rows)
		if err != nil {
			b.Fatal(err)
		}
		reader.Close()

		edit := manifest.NewVersionEdit()
		edit.AddFile(&manifest.FileMetadata{
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
	version := versionSet.GetCurrent()

	task := &CompactionTask{
		Level:       0,
		InputFiles:  version.GetLevel(0),
		OutputLevel: 1,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := compactor.DoCompaction(task, version)
		if err != nil {
			b.Fatal(err)
		}
	}
}
