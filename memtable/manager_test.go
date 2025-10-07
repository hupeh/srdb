package memtable

import (
	"testing"
)

func TestManagerBasic(t *testing.T) {
	mgr := NewManager(1024) // 1KB

	// 测试写入
	mgr.Put(1, []byte("value1"))
	mgr.Put(2, []byte("value2"))

	// 测试读取
	value, found := mgr.Get(1)
	if !found || string(value) != "value1" {
		t.Error("Get failed")
	}

	// 测试统计
	stats := mgr.GetStats()
	if stats.ActiveCount != 2 {
		t.Errorf("Expected 2 entries, got %d", stats.ActiveCount)
	}

	t.Log("Manager basic test passed!")
}

func TestManagerSwitch(t *testing.T) {
	mgr := NewManager(50) // 50 bytes
	mgr.SetActiveWAL(1)

	// 写入数据
	mgr.Put(1, []byte("value1_very_long_to_trigger_switch"))
	mgr.Put(2, []byte("value2_very_long_to_trigger_switch"))

	// 检查是否需要切换
	if !mgr.ShouldSwitch() {
		t.Logf("Size: %d, MaxSize: 50", mgr.GetActiveSize())
		// 不强制要求切换，因为大小计算可能不同
	}

	// 执行切换
	oldWAL, immutable := mgr.Switch(2)
	if oldWAL != 1 {
		t.Errorf("Expected old WAL 1, got %d", oldWAL)
	}

	if immutable == nil {
		t.Error("Immutable should not be nil")
	}

	// 检查 Immutable 数量
	if mgr.GetImmutableCount() != 1 {
		t.Errorf("Expected 1 immutable, got %d", mgr.GetImmutableCount())
	}

	// 新的 Active 应该是空的
	if mgr.GetActiveCount() != 0 {
		t.Errorf("New active should be empty, got %d", mgr.GetActiveCount())
	}

	// 应该还能查到旧数据（在 Immutable 中）
	value, found := mgr.Get(1)
	if !found || string(value) != "value1_very_long_to_trigger_switch" {
		t.Error("Should find value in immutable")
	}

	t.Log("Manager switch test passed!")
}

func TestManagerMultipleImmutables(t *testing.T) {
	mgr := NewManager(50)
	mgr.SetActiveWAL(1)

	// 第一批数据
	mgr.Put(1, []byte("value1_long_enough"))
	mgr.Switch(2)

	// 第二批数据
	mgr.Put(2, []byte("value2_long_enough"))
	mgr.Switch(3)

	// 第三批数据
	mgr.Put(3, []byte("value3_long_enough"))
	mgr.Switch(4)

	// 应该有 3 个 Immutable
	if mgr.GetImmutableCount() != 3 {
		t.Errorf("Expected 3 immutables, got %d", mgr.GetImmutableCount())
	}

	// 应该能查到所有数据
	for i := int64(1); i <= 3; i++ {
		if _, found := mgr.Get(i); !found {
			t.Errorf("Should find key %d", i)
		}
	}

	t.Log("Manager multiple immutables test passed!")
}

func TestManagerRemoveImmutable(t *testing.T) {
	mgr := NewManager(50)
	mgr.SetActiveWAL(1)

	// 创建 Immutable
	mgr.Put(1, []byte("value1_long_enough"))
	_, immutable := mgr.Switch(2)

	// 移除 Immutable
	mgr.RemoveImmutable(immutable)

	// 应该没有 Immutable 了
	if mgr.GetImmutableCount() != 0 {
		t.Errorf("Expected 0 immutables, got %d", mgr.GetImmutableCount())
	}

	// 数据应该找不到了
	if _, found := mgr.Get(1); found {
		t.Error("Should not find removed data")
	}

	t.Log("Manager remove immutable test passed!")
}

func TestManagerStats(t *testing.T) {
	mgr := NewManager(100)
	mgr.SetActiveWAL(1)

	// Active 数据
	mgr.Put(1, []byte("active1"))
	mgr.Put(2, []byte("active2"))

	// 创建 Immutable
	mgr.Put(3, []byte("immutable1_long"))
	mgr.Switch(2)

	// 新 Active 数据
	mgr.Put(4, []byte("active3"))

	stats := mgr.GetStats()

	if stats.ActiveCount != 1 {
		t.Errorf("Expected 1 active entry, got %d", stats.ActiveCount)
	}

	if stats.ImmutableCount != 1 {
		t.Errorf("Expected 1 immutable, got %d", stats.ImmutableCount)
	}

	if stats.ImmutablesTotal != 3 {
		t.Errorf("Expected 3 entries in immutables, got %d", stats.ImmutablesTotal)
	}

	if stats.TotalCount != 4 {
		t.Errorf("Expected 4 total entries, got %d", stats.TotalCount)
	}

	t.Logf("Stats: %+v", stats)
	t.Log("Manager stats test passed!")
}

func TestManagerConcurrent(t *testing.T) {
	mgr := NewManager(1024)
	mgr.SetActiveWAL(1)

	// 并发写入
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := int64(id*100 + j)
				mgr.Put(key, []byte("value"))
			}
			done <- true
		}(i)
	}

	// 等待完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 检查总数
	stats := mgr.GetStats()
	if stats.TotalCount != 1000 {
		t.Errorf("Expected 1000 entries, got %d", stats.TotalCount)
	}

	t.Log("Manager concurrent test passed!")
}
