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
