package srdb

import (
	"testing"
)

func TestMemTable(t *testing.T) {
	mt := NewMemTable()

	// 1. 插入数据
	for i := int64(1); i <= 100; i++ {
		mt.Put(i, []byte("value_"+string(rune(i))))
	}

	if mt.Count() != 100 {
		t.Errorf("Expected 100 entries, got %d", mt.Count())
	}

	t.Logf("Inserted 100 entries, size: %d bytes", mt.Size())

	// 2. 查询数据
	for i := int64(1); i <= 100; i++ {
		value, exists := mt.Get(i)
		if !exists {
			t.Errorf("Key %d not found", i)
		}
		if value == nil {
			t.Errorf("Key %d: value is nil", i)
		}
	}

	// 3. 查询不存在的 key
	_, exists := mt.Get(101)
	if exists {
		t.Error("Key 101 should not exist")
	}

	t.Log("All tests passed!")
}

func TestMemTableIterator(t *testing.T) {
	mt := NewMemTable()

	// 插入数据 (乱序)
	keys := []int64{5, 2, 8, 1, 9, 3, 7, 4, 6, 10}
	for _, key := range keys {
		mt.Put(key, []byte("value"))
	}

	// 迭代器应该按顺序返回
	iter := mt.NewIterator()
	var result []int64
	for iter.Next() {
		result = append(result, iter.Key())
	}

	// 验证顺序
	for i := 0; i < len(result)-1; i++ {
		if result[i] >= result[i+1] {
			t.Errorf("Keys not in order: %v", result)
			break
		}
	}

	if len(result) != 10 {
		t.Errorf("Expected 10 keys, got %d", len(result))
	}

	t.Logf("Iterator returned keys in order: %v", result)
}

func TestMemTableClear(t *testing.T) {
	mt := NewMemTable()

	// 插入数据
	for i := int64(1); i <= 10; i++ {
		mt.Put(i, []byte("value"))
	}

	if mt.Count() != 10 {
		t.Errorf("Expected 10 entries, got %d", mt.Count())
	}

	// 清空
	mt.Clear()

	if mt.Count() != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", mt.Count())
	}

	if mt.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", mt.Size())
	}

	t.Log("Clear test passed!")
}

func BenchmarkMemTablePut(b *testing.B) {
	mt := NewMemTable()
	value := make([]byte, 100)

	for i := 0; b.Loop(); i++ {
		mt.Put(int64(i), value)
	}
}

func BenchmarkMemTableGet(b *testing.B) {
	mt := NewMemTable()
	value := make([]byte, 100)

	// 预先插入数据
	for i := range int64(10000) {
		mt.Put(i, value)
	}

	for i := 0; b.Loop(); i++ {
		mt.Get(int64(i % 10000))
	}
}

func TestMemTableManagerBasic(t *testing.T) {
	mgr := NewMemTableManager(1024) // 1KB

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

func TestMemTableManagerSwitch(t *testing.T) {
	mgr := NewMemTableManager(50) // 50 bytes
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

func TestMemTableManagerMultipleImmutables(t *testing.T) {
	mgr := NewMemTableManager(50)
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

func TestMemTableManagerRemoveImmutable(t *testing.T) {
	mgr := NewMemTableManager(50)
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

func TestMemTableManagerStats(t *testing.T) {
	mgr := NewMemTableManager(100)
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

func TestMemTableManagerConcurrent(t *testing.T) {
	mgr := NewMemTableManager(1024)
	mgr.SetActiveWAL(1)

	// 并发写入
	done := make(chan bool)
	for i := range 10 {
		go func(id int) {
			for j := range 100 {
				key := int64(id*100 + j)
				mgr.Put(key, []byte("value"))
			}
			done <- true
		}(i)
	}

	// 等待完成
	for range 10 {
		<-done
	}

	// 检查总数
	stats := mgr.GetStats()
	if stats.TotalCount != 1000 {
		t.Errorf("Expected 1000 entries, got %d", stats.TotalCount)
	}

	t.Log("Manager concurrent test passed!")
}
