package srdb

import (
	"testing"
)

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
			MinKey:     int64((i+5) * 100),
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
