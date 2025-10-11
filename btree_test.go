package srdb

import (
	"os"
	"testing"

	"github.com/edsrzf/mmap-go"
)

func TestBTree(t *testing.T) {
	// 1. 创建测试文件
	file, err := os.Create("test.sst")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("test.sst")

	// 2. 构建 B+Tree
	builder := NewBTreeBuilder(file, 256) // 从 offset 256 开始

	// 添加 1000 个 key-value
	for i := int64(1); i <= 1000; i++ {
		dataOffset := 1000000 + i*100 // 模拟数据位置
		dataSize := int32(100)
		err := builder.Add(i, dataOffset, dataSize)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 构建
	rootOffset, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Root offset: %d", rootOffset)

	// 3. 关闭并重新打开文件
	file.Close()

	file, err = os.Open("test.sst")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	// 4. mmap 映射
	mmapData, err := mmap.Map(file, mmap.RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer mmapData.Unmap()

	// 5. 查询测试
	reader := NewBTreeReader(mmapData, rootOffset)

	// 测试存在的 key
	for i := int64(1); i <= 1000; i++ {
		offset, size, found := reader.Get(i)
		if !found {
			t.Errorf("Key %d not found", i)
		}
		expectedOffset := 1000000 + i*100
		if offset != expectedOffset {
			t.Errorf("Key %d: expected offset %d, got %d", i, expectedOffset, offset)
		}
		if size != 100 {
			t.Errorf("Key %d: expected size 100, got %d", i, size)
		}
	}

	// 测试不存在的 key
	_, _, found := reader.Get(1001)
	if found {
		t.Error("Key 1001 should not exist")
	}

	_, _, found = reader.Get(0)
	if found {
		t.Error("Key 0 should not exist")
	}

	t.Log("All tests passed!")
}

func TestBTreeSerialization(t *testing.T) {
	// 测试节点序列化
	leaf := NewLeafNode()
	if err := leaf.AddData(1, 1000, 100); err != nil {
		t.Fatal(err)
	}
	if err := leaf.AddData(2, 2000, 200); err != nil {
		t.Fatal(err)
	}
	if err := leaf.AddData(3, 3000, 300); err != nil {
		t.Fatal(err)
	}

	// 序列化
	data := leaf.Marshal()
	if len(data) != BTreeNodeSize {
		t.Errorf("Expected size %d, got %d", BTreeNodeSize, len(data))
	}

	// 反序列化
	leaf2 := UnmarshalBTree(data)
	if leaf2 == nil {
		t.Fatal("Unmarshal failed")
	}

	// 验证
	if leaf2.NodeType != BTreeNodeTypeLeaf {
		t.Error("Wrong node type")
	}
	if leaf2.KeyCount != 3 {
		t.Errorf("Expected 3 keys, got %d", leaf2.KeyCount)
	}
	if len(leaf2.Keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(leaf2.Keys))
	}
	if leaf2.Keys[0] != 1 || leaf2.Keys[1] != 2 || leaf2.Keys[2] != 3 {
		t.Error("Keys mismatch")
	}
	if leaf2.DataOffsets[0] != 1000 || leaf2.DataOffsets[1] != 2000 || leaf2.DataOffsets[2] != 3000 {
		t.Error("Data offsets mismatch")
	}
	if leaf2.DataSizes[0] != 100 || leaf2.DataSizes[1] != 200 || leaf2.DataSizes[2] != 300 {
		t.Error("Data sizes mismatch")
	}

	t.Log("Serialization test passed!")
}

// TestBTreeForEach 测试升序迭代
func TestBTreeForEach(t *testing.T) {
	// 创建测试文件
	file, err := os.Create("test_foreach.sst")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("test_foreach.sst")

	// 构建 B+Tree
	builder := NewBTreeBuilder(file, 256)
	for i := int64(1); i <= 100; i++ {
		err := builder.Add(i, i*100, int32(i*10))
		if err != nil {
			t.Fatal(err)
		}
	}

	rootOffset, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	file.Close()

	// 打开并 mmap
	file, _ = os.Open("test_foreach.sst")
	defer file.Close()
	mmapData, _ := mmap.Map(file, mmap.RDONLY, 0)
	defer mmapData.Unmap()

	reader := NewBTreeReader(mmapData, rootOffset)

	// 测试 1: 完整升序迭代
	t.Run("Complete", func(t *testing.T) {
		var keys []int64
		var offsets []int64
		var sizes []int32

		reader.ForEach(func(key int64, offset int64, size int32) bool {
			keys = append(keys, key)
			offsets = append(offsets, offset)
			sizes = append(sizes, size)
			return true
		})

		// 验证数量
		if len(keys) != 100 {
			t.Errorf("Expected 100 keys, got %d", len(keys))
		}

		// 验证顺序（升序）
		for i := 0; i < len(keys)-1; i++ {
			if keys[i] >= keys[i+1] {
				t.Errorf("Keys not in ascending order: keys[%d]=%d, keys[%d]=%d",
					i, keys[i], i+1, keys[i+1])
			}
		}

		// 验证第一个和最后一个
		if keys[0] != 1 {
			t.Errorf("Expected first key=1, got %d", keys[0])
		}
		if keys[99] != 100 {
			t.Errorf("Expected last key=100, got %d", keys[99])
		}

		// 验证 offset 和 size
		for i, key := range keys {
			expectedOffset := key * 100
			expectedSize := int32(key * 10)
			if offsets[i] != expectedOffset {
				t.Errorf("Key %d: expected offset %d, got %d", key, expectedOffset, offsets[i])
			}
			if sizes[i] != expectedSize {
				t.Errorf("Key %d: expected size %d, got %d", key, expectedSize, sizes[i])
			}
		}
	})

	// 测试 2: 提前终止
	t.Run("EarlyTermination", func(t *testing.T) {
		var keys []int64
		reader.ForEach(func(key int64, offset int64, size int32) bool {
			keys = append(keys, key)
			return len(keys) < 5 // 只收集 5 个
		})

		if len(keys) != 5 {
			t.Errorf("Expected 5 keys, got %d", len(keys))
		}
		if keys[0] != 1 || keys[4] != 5 {
			t.Errorf("Expected keys [1,2,3,4,5], got %v", keys)
		}
	})

	// 测试 3: 条件过滤
	t.Run("ConditionalFilter", func(t *testing.T) {
		var evenKeys []int64
		reader.ForEach(func(key int64, offset int64, size int32) bool {
			if key%2 == 0 {
				evenKeys = append(evenKeys, key)
			}
			return true
		})

		if len(evenKeys) != 50 {
			t.Errorf("Expected 50 even keys, got %d", len(evenKeys))
		}

		// 验证都是偶数
		for _, key := range evenKeys {
			if key%2 != 0 {
				t.Errorf("Key %d is not even", key)
			}
		}
	})

	// 测试 4: 查找第一个满足条件的
	t.Run("FindFirst", func(t *testing.T) {
		var foundKey int64
		count := 0
		reader.ForEach(func(key int64, offset int64, size int32) bool {
			count++
			if key > 50 {
				foundKey = key
				return false // 找到后停止
			}
			return true
		})

		if foundKey != 51 {
			t.Errorf("Expected to find key 51, got %d", foundKey)
		}
		if count != 51 {
			t.Errorf("Expected to iterate 51 times, got %d", count)
		}
	})

	// 测试 5: 与 GetAllKeys 结果一致性
	t.Run("ConsistencyWithGetAllKeys", func(t *testing.T) {
		var iterKeys []int64
		reader.ForEach(func(key int64, offset int64, size int32) bool {
			iterKeys = append(iterKeys, key)
			return true
		})

		allKeys := reader.GetAllKeys()

		if len(iterKeys) != len(allKeys) {
			t.Errorf("Length mismatch: ForEach=%d, GetAllKeys=%d", len(iterKeys), len(allKeys))
		}

		for i := range iterKeys {
			if iterKeys[i] != allKeys[i] {
				t.Errorf("Key mismatch at index %d: ForEach=%d, GetAllKeys=%d",
					i, iterKeys[i], allKeys[i])
			}
		}
	})
}

// TestBTreeForEachDesc 测试降序迭代
func TestBTreeForEachDesc(t *testing.T) {
	// 创建测试文件
	file, err := os.Create("test_foreach_desc.sst")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("test_foreach_desc.sst")

	// 构建 B+Tree
	builder := NewBTreeBuilder(file, 256)
	for i := int64(1); i <= 100; i++ {
		err := builder.Add(i, i*100, int32(i*10))
		if err != nil {
			t.Fatal(err)
		}
	}

	rootOffset, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	file.Close()

	// 打开并 mmap
	file, _ = os.Open("test_foreach_desc.sst")
	defer file.Close()
	mmapData, _ := mmap.Map(file, mmap.RDONLY, 0)
	defer mmapData.Unmap()

	reader := NewBTreeReader(mmapData, rootOffset)

	// 测试 1: 完整降序迭代
	t.Run("Complete", func(t *testing.T) {
		var keys []int64
		reader.ForEachDesc(func(key int64, offset int64, size int32) bool {
			keys = append(keys, key)
			return true
		})

		// 验证数量
		if len(keys) != 100 {
			t.Errorf("Expected 100 keys, got %d", len(keys))
		}

		// 验证顺序（降序）
		for i := 0; i < len(keys)-1; i++ {
			if keys[i] <= keys[i+1] {
				t.Errorf("Keys not in descending order: keys[%d]=%d, keys[%d]=%d",
					i, keys[i], i+1, keys[i+1])
			}
		}

		// 验证第一个和最后一个
		if keys[0] != 100 {
			t.Errorf("Expected first key=100, got %d", keys[0])
		}
		if keys[99] != 1 {
			t.Errorf("Expected last key=1, got %d", keys[99])
		}
	})

	// 测试 2: 获取最新的 N 条记录（时序数据库常见需求）
	t.Run("GetLatestN", func(t *testing.T) {
		var latestKeys []int64
		reader.ForEachDesc(func(key int64, offset int64, size int32) bool {
			latestKeys = append(latestKeys, key)
			return len(latestKeys) < 10 // 只取最新的 10 条
		})

		if len(latestKeys) != 10 {
			t.Errorf("Expected 10 keys, got %d", len(latestKeys))
		}

		// 验证是最新的 10 条（100, 99, 98, ..., 91）
		for i, key := range latestKeys {
			expected := int64(100 - i)
			if key != expected {
				t.Errorf("latestKeys[%d]: expected %d, got %d", i, expected, key)
			}
		}
	})

	// 测试 3: 与 GetAllKeysDesc 结果一致性
	t.Run("ConsistencyWithGetAllKeysDesc", func(t *testing.T) {
		var iterKeys []int64
		reader.ForEachDesc(func(key int64, offset int64, size int32) bool {
			iterKeys = append(iterKeys, key)
			return true
		})

		allKeys := reader.GetAllKeysDesc()

		if len(iterKeys) != len(allKeys) {
			t.Errorf("Length mismatch: ForEachDesc=%d, GetAllKeysDesc=%d", len(iterKeys), len(allKeys))
		}

		for i := range iterKeys {
			if iterKeys[i] != allKeys[i] {
				t.Errorf("Key mismatch at index %d: ForEachDesc=%d, GetAllKeysDesc=%d",
					i, iterKeys[i], allKeys[i])
			}
		}
	})

	// 测试 4: 降序查找第一个满足条件的
	t.Run("FindFirstDesc", func(t *testing.T) {
		var foundKey int64
		count := 0
		reader.ForEachDesc(func(key int64, offset int64, size int32) bool {
			count++
			if key < 50 {
				foundKey = key
				return false // 找到后停止
			}
			return true
		})

		if foundKey != 49 {
			t.Errorf("Expected to find key 49, got %d", foundKey)
		}
		if count != 52 { // 100, 99, ..., 50, 49
			t.Errorf("Expected to iterate 52 times, got %d", count)
		}
	})
}

// TestBTreeForEachEmpty 测试空树的迭代
func TestBTreeForEachEmpty(t *testing.T) {
	// 创建空的 B+Tree
	file, _ := os.Create("test_empty.sst")
	defer os.Remove("test_empty.sst")

	builder := NewBTreeBuilder(file, 256)
	rootOffset, _ := builder.Build()
	file.Close()

	file, _ = os.Open("test_empty.sst")
	defer file.Close()
	mmapData, _ := mmap.Map(file, mmap.RDONLY, 0)
	defer mmapData.Unmap()

	reader := NewBTreeReader(mmapData, rootOffset)

	// 测试升序迭代
	t.Run("ForEach", func(t *testing.T) {
		called := false
		reader.ForEach(func(key int64, offset int64, size int32) bool {
			called = true
			return true
		})

		if called {
			t.Error("Callback should not be called on empty tree")
		}
	})

	// 测试降序迭代
	t.Run("ForEachDesc", func(t *testing.T) {
		called := false
		reader.ForEachDesc(func(key int64, offset int64, size int32) bool {
			called = true
			return true
		})

		if called {
			t.Error("Callback should not be called on empty tree")
		}
	})
}

// TestBTreeForEachSingle 测试单个元素的迭代
func TestBTreeForEachSingle(t *testing.T) {
	// 创建只有一个元素的 B+Tree
	file, _ := os.Create("test_single.sst")
	defer os.Remove("test_single.sst")

	builder := NewBTreeBuilder(file, 256)
	builder.Add(42, 4200, 420)
	rootOffset, _ := builder.Build()
	file.Close()

	file, _ = os.Open("test_single.sst")
	defer file.Close()
	mmapData, _ := mmap.Map(file, mmap.RDONLY, 0)
	defer mmapData.Unmap()

	reader := NewBTreeReader(mmapData, rootOffset)

	// 测试升序迭代
	t.Run("ForEach", func(t *testing.T) {
		var keys []int64
		reader.ForEach(func(key int64, offset int64, size int32) bool {
			keys = append(keys, key)
			if offset != 4200 || size != 420 {
				t.Errorf("Unexpected data: offset=%d, size=%d", offset, size)
			}
			return true
		})

		if len(keys) != 1 || keys[0] != 42 {
			t.Errorf("Expected single key 42, got %v", keys)
		}
	})

	// 测试降序迭代
	t.Run("ForEachDesc", func(t *testing.T) {
		var keys []int64
		reader.ForEachDesc(func(key int64, offset int64, size int32) bool {
			keys = append(keys, key)
			return true
		})

		if len(keys) != 1 || keys[0] != 42 {
			t.Errorf("Expected single key 42, got %v", keys)
		}
	})
}

func BenchmarkBTreeGet(b *testing.B) {
	// 构建测试数据
	file, _ := os.Create("bench.sst")
	defer os.Remove("bench.sst")

	builder := NewBTreeBuilder(file, 256)
	for i := int64(1); i <= 100000; i++ {
		builder.Add(i, i*100, 100)
	}
	rootOffset, _ := builder.Build()
	file.Close()

	// mmap
	file, _ = os.Open("bench.sst")
	defer file.Close()
	mmapData, _ := mmap.Map(file, mmap.RDONLY, 0)
	defer mmapData.Unmap()

	reader := NewBTreeReader(mmapData, rootOffset)

	// 性能测试

	for i := 0; b.Loop(); i++ {
		key := int64(i%100000 + 1)
		reader.Get(key)
	}
}

// BenchmarkBTreeForEach 性能测试：完整迭代
func BenchmarkBTreeForEach(b *testing.B) {
	file, _ := os.Create("bench_foreach.sst")
	defer os.Remove("bench_foreach.sst")

	builder := NewBTreeBuilder(file, 256)
	for i := int64(1); i <= 10000; i++ {
		builder.Add(i, i*100, 100)
	}
	rootOffset, _ := builder.Build()
	file.Close()

	file, _ = os.Open("bench_foreach.sst")
	defer file.Close()
	mmapData, _ := mmap.Map(file, mmap.RDONLY, 0)
	defer mmapData.Unmap()

	reader := NewBTreeReader(mmapData, rootOffset)

	b.ResetTimer()
	for b.Loop() {
		count := 0
		reader.ForEach(func(key int64, offset int64, size int32) bool {
			count++
			return true
		})
	}
}

// BenchmarkBTreeForEachEarlyTermination 性能测试：提前终止
func BenchmarkBTreeForEachEarlyTermination(b *testing.B) {
	file, _ := os.Create("bench_foreach_early.sst")
	defer os.Remove("bench_foreach_early.sst")

	builder := NewBTreeBuilder(file, 256)
	for i := int64(1); i <= 100000; i++ {
		builder.Add(i, i*100, 100)
	}
	rootOffset, _ := builder.Build()
	file.Close()

	file, _ = os.Open("bench_foreach_early.sst")
	defer file.Close()
	mmapData, _ := mmap.Map(file, mmap.RDONLY, 0)
	defer mmapData.Unmap()

	reader := NewBTreeReader(mmapData, rootOffset)

	b.ResetTimer()
	for b.Loop() {
		count := 0
		reader.ForEach(func(key int64, offset int64, size int32) bool {
			count++
			return count < 10 // 只读取前 10 个
		})
	}
}

// BenchmarkBTreeGetAllKeys vs ForEach 对比
func BenchmarkBTreeGetAllKeys(b *testing.B) {
	file, _ := os.Create("bench_getall.sst")
	defer os.Remove("bench_getall.sst")

	builder := NewBTreeBuilder(file, 256)
	for i := int64(1); i <= 10000; i++ {
		builder.Add(i, i*100, 100)
	}
	rootOffset, _ := builder.Build()
	file.Close()

	file, _ = os.Open("bench_getall.sst")
	defer file.Close()
	mmapData, _ := mmap.Map(file, mmap.RDONLY, 0)
	defer mmapData.Unmap()

	reader := NewBTreeReader(mmapData, rootOffset)

	b.ResetTimer()
	for b.Loop() {
		_ = reader.GetAllKeys()
	}
}
