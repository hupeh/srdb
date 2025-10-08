package srdb

import (
	"os"
	"testing"
)

func TestIndexVersionControl(t *testing.T) {
	dir := "test_index_version"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	testSchema := NewSchema("test", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "名称"},
	})

	// 1. 创建索引管理器
	mgr := NewIndexManager(dir, testSchema)

	// 2. 创建索引
	mgr.CreateIndex("name")
	idx, _ := mgr.GetIndex("name")

	// 3. 添加数据
	idx.Add("Alice", 1)
	idx.Add("Bob", 2)
	idx.Add("Alice", 3)

	// 4. 保存索引
	idx.Build()

	// 5. 检查元数据
	metadata := idx.GetMetadata()
	if metadata.Version != 1 {
		t.Errorf("Expected version 1, got %d", metadata.Version)
	}
	if metadata.MinSeq != 1 {
		t.Errorf("Expected MinSeq 1, got %d", metadata.MinSeq)
	}
	if metadata.MaxSeq != 3 {
		t.Errorf("Expected MaxSeq 3, got %d", metadata.MaxSeq)
	}
	if metadata.RowCount != 3 {
		t.Errorf("Expected RowCount 3, got %d", metadata.RowCount)
	}

	t.Logf("Metadata: Version=%d, MinSeq=%d, MaxSeq=%d, RowCount=%d",
		metadata.Version, metadata.MinSeq, metadata.MaxSeq, metadata.RowCount)

	// 6. 关闭并重新加载
	mgr.Close()

	mgr2 := NewIndexManager(dir, testSchema)
	idx2, _ := mgr2.GetIndex("name")

	// 7. 验证元数据被正确加载
	metadata2 := idx2.GetMetadata()
	if metadata2.Version != metadata.Version {
		t.Errorf("Version mismatch after reload")
	}
	if metadata2.MaxSeq != metadata.MaxSeq {
		t.Errorf("MaxSeq mismatch after reload")
	}

	t.Log("索引版本控制测试通过！")
}

func TestIncrementalUpdate(t *testing.T) {
	dir := "test_incremental_update"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	testSchema := NewSchema("test", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "名称"},
	})

	// 1. 创建索引并添加初始数据
	mgr := NewIndexManager(dir, testSchema)
	mgr.CreateIndex("name")
	idx, _ := mgr.GetIndex("name")

	idx.Add("Alice", 1)
	idx.Add("Bob", 2)
	idx.Build()

	initialMetadata := idx.GetMetadata()
	t.Logf("Initial: MaxSeq=%d, RowCount=%d", initialMetadata.MaxSeq, initialMetadata.RowCount)

	// 2. 模拟新数据
	mockData := map[int64]map[string]any{
		3: {"name": "Charlie"},
		4: {"name": "David"},
		5: {"name": "Alice"},
	}

	getData := func(seq int64) (map[string]any, error) {
		if data, exists := mockData[seq]; exists {
			return data, nil
		}
		return nil, nil
	}

	// 3. 增量更新
	err := idx.IncrementalUpdate(getData, 3, 5)
	if err != nil {
		t.Fatal(err)
	}

	// 4. 验证更新后的元数据
	updatedMetadata := idx.GetMetadata()
	if updatedMetadata.MaxSeq != 5 {
		t.Errorf("Expected MaxSeq 5, got %d", updatedMetadata.MaxSeq)
	}
	if updatedMetadata.RowCount != 5 {
		t.Errorf("Expected RowCount 5, got %d", updatedMetadata.RowCount)
	}
	if updatedMetadata.Version != 2 {
		t.Errorf("Expected Version 2, got %d", updatedMetadata.Version)
	}

	t.Logf("Updated: MaxSeq=%d, RowCount=%d, Version=%d",
		updatedMetadata.MaxSeq, updatedMetadata.RowCount, updatedMetadata.Version)

	// 5. 验证数据
	seqs, _ := idx.Get("Alice")
	if len(seqs) != 2 {
		t.Errorf("Expected 2 seqs for Alice, got %d", len(seqs))
	}

	seqs, _ = idx.Get("Charlie")
	if len(seqs) != 1 {
		t.Errorf("Expected 1 seq for Charlie, got %d", len(seqs))
	}

	t.Log("增量更新测试通过！")
}

func TestNeedsUpdate(t *testing.T) {
	dir := "test_needs_update"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	testSchema := NewSchema("test", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "名称"},
	})

	mgr := NewIndexManager(dir, testSchema)
	mgr.CreateIndex("name")
	idx, _ := mgr.GetIndex("name")

	idx.Add("Alice", 1)
	idx.Add("Bob", 2)
	idx.Build()

	// 测试 NeedsUpdate
	if idx.NeedsUpdate(2) {
		t.Error("Should not need update when currentMaxSeq = 2")
	}

	if !idx.NeedsUpdate(5) {
		t.Error("Should need update when currentMaxSeq = 5")
	}

	t.Log("NeedsUpdate 测试通过！")
}

func TestIndexPersistence(t *testing.T) {
	dir := "test_index_persistence"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	// 创建 Schema
	testSchema := NewSchema("test", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "名称"},
		{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
	})

	// 1. 创建索引管理器
	mgr := NewIndexManager(dir, testSchema)

	// 2. 创建索引
	err := mgr.CreateIndex("name")
	if err != nil {
		t.Fatal(err)
	}

	// 3. 添加数据到索引
	idx, _ := mgr.GetIndex("name")
	idx.Add("Alice", 1)
	idx.Add("Bob", 2)
	idx.Add("Alice", 3)
	idx.Add("Charlie", 4)

	// 4. 构建并保存索引
	err = idx.Build()
	if err != nil {
		t.Fatal(err)
	}

	t.Log("索引已保存到磁盘")

	// 5. 关闭管理器
	mgr.Close()

	// 6. 创建新的管理器（模拟重启）
	mgr2 := NewIndexManager(dir, testSchema)

	// 7. 检查索引是否自动加载
	indexes := mgr2.ListIndexes()
	if len(indexes) != 1 {
		t.Errorf("Expected 1 index, got %d", len(indexes))
	}

	// 8. 验证索引数据
	idx2, exists := mgr2.GetIndex("name")
	if !exists {
		t.Fatal("Index 'name' not found after reload")
	}

	if !idx2.IsReady() {
		t.Error("Index should be ready after reload")
	}

	// 9. 查询索引
	seqs, err := idx2.Get("Alice")
	if err != nil {
		t.Fatal(err)
	}

	if len(seqs) != 2 {
		t.Errorf("Expected 2 seqs for 'Alice', got %d", len(seqs))
	}

	seqs, err = idx2.Get("Bob")
	if err != nil {
		t.Fatal(err)
	}

	if len(seqs) != 1 {
		t.Errorf("Expected 1 seq for 'Bob', got %d", len(seqs))
	}

	t.Log("索引持久化测试通过！")
}

func TestIndexDropWithFile(t *testing.T) {
	dir := "test_index_drop"
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	testSchema := NewSchema("test", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "名称"},
	})

	mgr := NewIndexManager(dir, testSchema)

	// 创建索引
	mgr.CreateIndex("name")
	idx, _ := mgr.GetIndex("name")
	idx.Add("Alice", 1)
	idx.Build()

	// 检查文件是否存在
	indexPath := dir + "/idx_name.sst"
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("Index file should exist")
	}

	// 删除索引
	err := mgr.DropIndex("name")
	if err != nil {
		t.Fatal(err)
	}

	// 检查文件是否被删除
	if _, err := os.Stat(indexPath); !os.IsNotExist(err) {
		t.Error("Index file should be deleted")
	}

	t.Log("索引删除测试通过！")
}
