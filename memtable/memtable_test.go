package memtable

import (
	"testing"
)

func TestMemTable(t *testing.T) {
	mt := New()

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
	mt := New()

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
	mt := New()

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
	mt := New()
	value := make([]byte, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mt.Put(int64(i), value)
	}
}

func BenchmarkMemTableGet(b *testing.B) {
	mt := New()
	value := make([]byte, 100)

	// 预先插入数据
	for i := int64(0); i < 10000; i++ {
		mt.Put(i, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mt.Get(int64(i % 10000))
	}
}
