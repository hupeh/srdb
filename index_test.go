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

	testSchema, err := NewSchema("test", []Field{
		{Name: "name", Type: String, Indexed: true, Comment: "名称"},
	})
	if err != nil {
		t.Fatal(err)
	}

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

	testSchema, err := NewSchema("test", []Field{
		{Name: "name", Type: String, Indexed: true, Comment: "名称"},
	})
	if err != nil {
		t.Fatal(err)
	}

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
	err = idx.IncrementalUpdate(getData, 3, 5)
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

	testSchema, err := NewSchema("test", []Field{
		{Name: "name", Type: String, Indexed: true, Comment: "名称"},
	})
	if err != nil {
		t.Fatal(err)
	}

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
	testSchema, err := NewSchema("test", []Field{
		{Name: "name", Type: String, Indexed: true, Comment: "名称"},
		{Name: "age", Type: Int64, Indexed: true, Comment: "年龄"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. 创建索引管理器
	mgr := NewIndexManager(dir, testSchema)

	// 2. 创建索引
	err = mgr.CreateIndex("name")
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

	testSchema, err := NewSchema("test", []Field{
		{Name: "name", Type: String, Indexed: true, Comment: "名称"},
	})
	if err != nil {
		t.Fatal(err)
	}

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
	err = mgr.DropIndex("name")
	if err != nil {
		t.Fatal(err)
	}

	// 检查文件是否被删除
	if _, err := os.Stat(indexPath); !os.IsNotExist(err) {
		t.Error("Index file should be deleted")
	}

	t.Log("索引删除测试通过！")
}

// TestIndexQueryIntegration 测试索引查询的完整流程
func TestIndexQueryIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. 创建带索引字段的 Schema
	schema, err := NewSchema("users", []Field{
		{Name: "name", Type: String, Indexed: false},
		{Name: "email", Type: String, Indexed: true}, // email 字段有索引
		{Name: "age", Type: Int64, Indexed: false},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. 打开表
	table, err := OpenTable(&TableOptions{
		Dir:          tmpDir,
		Name:         schema.Name,
		Fields:       schema.Fields,
		MemTableSize: 1024 * 1024, // 1MB
	})
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()

	// 3. 创建索引
	err = table.CreateIndex("email")
	if err != nil {
		t.Fatal(err)
	}

	// 4. 插入测试数据
	testData := []map[string]any{
		{"name": "Alice", "email": "alice@example.com", "age": int64(25)},
		{"name": "Bob", "email": "bob@example.com", "age": int64(30)},
		{"name": "Charlie", "email": "alice@example.com", "age": int64(35)}, // 相同 email
		{"name": "David", "email": "david@example.com", "age": int64(40)},
	}

	for _, data := range testData {
		err := table.Insert(data)
		if err != nil {
			t.Fatalf("Failed to insert data: %v", err)
		}
	}

	// 5. 构建索引（持久化）
	err = table.indexManager.BuildAll()
	if err != nil {
		t.Fatalf("Failed to build indexes: %v", err)
	}

	// 6. 验证索引文件存在
	indexPath := tmpDir + "/idx/idx_email.sst"
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Fatalf("Index file not created: %s", indexPath)
	}
	t.Logf("✓ Index file created: %s", indexPath)

	// 7. 使用索引查询
	rows, err := table.Query().Eq("email", "alice@example.com").Rows()
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	// 8. 验证结果
	var results []map[string]any
	for rows.Next() {
		results = append(results, rows.Row().Data())
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// 验证结果内容
	for _, result := range results {
		if result["email"] != "alice@example.com" {
			t.Errorf("Unexpected email: %v", result["email"])
		}
		name := result["name"].(string)
		if name != "Alice" && name != "Charlie" {
			t.Errorf("Unexpected name: %s", name)
		}
	}

	t.Logf("✓ Index query returned correct results: %d rows", len(results))

	// 9. 测试没有索引的查询（应该正常工作但不使用索引）
	rows2, err := table.Query().Eq("name", "Bob").Rows()
	if err != nil {
		t.Fatalf("Query without index failed: %v", err)
	}
	defer rows2.Close()

	results2 := []map[string]any{}
	for rows2.Next() {
		results2 = append(results2, rows2.Row().Data())
	}

	if len(results2) != 1 {
		t.Errorf("Expected 1 result for Bob, got %d", len(results2))
	}

	t.Logf("✓ Non-indexed query works correctly: %d rows", len(results2))

	// 10. 测试索引在新数据上的工作
	err = table.Insert(map[string]any{
		"name":  "Eve",
		"email": "eve@example.com",
		"age":   int64(28),
	})
	if err != nil {
		t.Fatalf("Failed to insert new data: %v", err)
	}

	// 查询新插入的数据（索引尚未持久化，但应该在内存中）
	rows3, err := table.Query().Eq("email", "eve@example.com").Rows()
	if err != nil {
		t.Fatalf("Query for new data failed: %v", err)
	}
	defer rows3.Close()

	results3 := []map[string]any{}
	for rows3.Next() {
		results3 = append(results3, rows3.Row().Data())
	}

	if len(results3) != 1 {
		t.Errorf("Expected 1 result for Eve (new data), got %d", len(results3))
	}

	t.Logf("✓ Index works for new data (before persistence): %d rows", len(results3))

	// 11. 再次构建索引并验证
	err = table.indexManager.BuildAll()
	if err != nil {
		t.Fatalf("Failed to rebuild indexes: %v", err)
	}

	rows4, err := table.Query().Eq("email", "eve@example.com").Rows()
	if err != nil {
		t.Fatalf("Query after rebuild failed: %v", err)
	}
	defer rows4.Close()

	results4 := []map[string]any{}
	for rows4.Next() {
		results4 = append(results4, rows4.Row().Data())
	}

	if len(results4) != 1 {
		t.Errorf("Expected 1 result for Eve (after rebuild), got %d", len(results4))
	}

	t.Logf("✓ Index works after rebuild: %d rows", len(results4))

	t.Log("=== All index query tests passed ===")
}

// TestIndexPersistenceAcrossRestart 测试索引在重启后的持久化
func TestIndexPersistenceAcrossRestart(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. 第一次打开：创建数据和索引
	{
		schema, err := NewSchema("products", []Field{
			{Name: "name", Type: String, Indexed: false},
			{Name: "category", Type: String, Indexed: true},
			{Name: "price", Type: Int64, Indexed: false},
		})
		if err != nil {
			t.Fatal(err)
		}

		table, err := OpenTable(&TableOptions{
			Dir:          tmpDir,
			Name:         schema.Name,
			Fields:       schema.Fields,
			MemTableSize: 1024 * 1024,
		})
		if err != nil {
			t.Fatal(err)
		}

		// 创建索引
		err = table.CreateIndex("category")
		if err != nil {
			t.Fatal(err)
		}

		// 插入数据
		testData := []map[string]any{
			{"name": "Laptop", "category": "Electronics", "price": int64(1000)},
			{"name": "Mouse", "category": "Electronics", "price": int64(50)},
			{"name": "Desk", "category": "Furniture", "price": int64(300)},
		}

		for _, data := range testData {
			err := table.Insert(data)
			if err != nil {
				t.Fatal(err)
			}
		}

		// 构建索引
		err = table.indexManager.BuildAll()
		if err != nil {
			t.Fatal(err)
		}

		// 关闭表
		table.Close()

		t.Log("✓ First session: data and index created")
	}

	// 2. 第二次打开：验证索引仍然可用
	{
		table, err := OpenTable(&TableOptions{
			Dir:          tmpDir,
			MemTableSize: 1024 * 1024,
		})
		if err != nil {
			t.Fatal(err)
		}
		defer table.Close()

		// 验证索引存在
		indexes := table.ListIndexes()
		if len(indexes) != 1 || indexes[0] != "category" {
			t.Errorf("Expected index on 'category', got: %v", indexes)
		}

		t.Log("✓ Index loaded after restart")

		// 使用索引查询
		rows, err := table.Query().Eq("category", "Electronics").Rows()
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		defer rows.Close()

		results := []map[string]any{}
		for rows.Next() {
			results = append(results, rows.Row().Data())
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 Electronics products, got %d", len(results))
		}

		t.Logf("✓ Index query after restart: %d rows", len(results))
	}

	t.Log("=== Index persistence test passed ===")
}
