package srdb

import (
	"testing"
)

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
