package srdb

import (
	"os"
	"testing"
)

// TestIndexQueryIntegration 测试索引查询的完整流程
func TestIndexQueryIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. 创建带索引字段的 Schema
	schema := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: false},
		{Name: "email", Type: FieldTypeString, Indexed: true}, // email 字段有索引
		{Name: "age", Type: FieldTypeInt64, Indexed: false},
	})

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
		schema := NewSchema("products", []Field{
			{Name: "name", Type: FieldTypeString, Indexed: false},
			{Name: "category", Type: FieldTypeString, Indexed: true},
			{Name: "price", Type: FieldTypeInt64, Indexed: false},
		})

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
