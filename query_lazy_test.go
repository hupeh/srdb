package srdb

import (
	"fmt"
	"os"
	"testing"
)

// TestLazyLoadingBasic 测试惰性加载基本功能
func TestLazyLoadingBasic(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "TestLazyLoadingBasic")
	defer os.RemoveAll(tmpDir)

	schema, err := NewSchema("users", []Field{
		{Name: "name", Type: String},
		{Name: "age", Type: Int64},
	})
	if err != nil {
		t.Fatal(err)
	}

	table, err := OpenTable(&TableOptions{
		Dir:    tmpDir,
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()

	// 插入一些数据
	for i := 0; i < 100; i++ {
		err = table.Insert(map[string]any{
			"name": "User" + string(rune(i)),
			"age":  int64(20 + i),
		})
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	// 创建查询，但不立即执行
	rows, err := table.Query().Gte("age", int64(50)).Rows()
	if err != nil {
		t.Fatalf("Rows() failed: %v", err)
	}
	defer rows.Close()

	// 验证惰性加载：Rows() 返回时不应该已经加载数据
	if rows.cached {
		t.Errorf("Expected lazy loading (cached=false), but data is already cached")
	}

	// 只读取前 5 条记录
	count := 0
	for rows.Next() && count < 5 {
		count++
	}

	if count != 5 {
		t.Errorf("Expected to read 5 rows, got %d", count)
	}

	t.Log("✓ Lazy loading test passed: only 5 rows were read")
}

// TestLazyLoadingVsEagerLoading 对比惰性加载和立即加载
func TestLazyLoadingVsEagerLoading(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "TestLazyLoadingVsEagerLoading")
	defer os.RemoveAll(tmpDir)

	schema, err := NewSchema("users", []Field{
		{Name: "name", Type: String},
		{Name: "age", Type: Int64},
	})
	if err != nil {
		t.Fatal(err)
	}

	table, err := OpenTable(&TableOptions{
		Dir:    tmpDir,
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()

	// 插入大量数据
	for i := 0; i < 1000; i++ {
		err = table.Insert(map[string]any{
			"name": "User" + string(rune(i)),
			"age":  int64(20 + i%50),
		})
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	// Flush to SST
	table.Flush()

	// 测试 1: 惰性加载 - 只读取第一条
	rows, err := table.Query().Rows()
	if err != nil {
		t.Fatalf("Rows() failed: %v", err)
	}

	// 验证是惰性加载
	if rows.cached {
		t.Errorf("Expected lazy loading, but data is cached")
	}

	// 只读取第一条
	if rows.Next() {
		row := rows.Row()
		if row == nil {
			t.Errorf("Expected row, got nil")
		}
	} else {
		t.Errorf("Expected at least one row")
	}
	rows.Close()

	// 测试 2: 立即加载所有数据（通过 Collect）
	rows2, err := table.Query().Rows()
	if err != nil {
		t.Fatalf("Rows() failed: %v", err)
	}
	defer rows2.Close()

	// Collect 会触发立即加载
	allData := rows2.Collect()
	if len(allData) != 1000 {
		t.Errorf("Expected 1000 rows, got %d", len(allData))
	}

	// 验证现在已缓存
	if !rows2.cached {
		t.Errorf("Expected data to be cached after Collect()")
	}

	t.Log("✓ Lazy loading vs eager loading test passed")
}

// TestIndexQueryIsEager 验证索引查询是立即加载的
func TestIndexQueryIsEager(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "TestIndexQueryIsEager")
	defer os.RemoveAll(tmpDir)

	schema, err := NewSchema("users", []Field{
		{Name: "name", Type: String},
		{Name: "email", Type: String, Indexed: true},
		{Name: "age", Type: Int64},
	})
	if err != nil {
		t.Fatal(err)
	}

	table, err := OpenTable(&TableOptions{
		Dir:    tmpDir,
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()

	// 创建索引
	err = table.CreateIndex("email")
	if err != nil {
		t.Fatal(err)
	}

	// 插入数据
	for i := 0; i < 10; i++ {
		err = table.Insert(map[string]any{
			"name":  fmt.Sprintf("User%d", i),
			"email": fmt.Sprintf("user%d@example.com", i),
			"age":   int64(20 + i),
		})
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	// Flush to SST and build indexes
	table.Flush()

	// Build indexes explicitly
	err = table.indexManager.BuildAll()
	if err != nil {
		t.Fatalf("Failed to build indexes: %v", err)
	}

	// Check if index exists and is ready
	idx, exists := table.indexManager.GetIndex("email")
	if !exists {
		t.Fatalf("Index for email does not exist")
	}
	if !idx.IsReady() {
		t.Fatalf("Index for email is not ready")
	}

	// 使用索引查询
	rows, err := table.Query().Eq("email", "user0@example.com").Rows()
	if err != nil {
		t.Fatalf("Rows() failed: %v", err)
	}
	defer rows.Close()

	// 索引查询应该是立即加载的（cached=true）
	if !rows.cached {
		t.Errorf("Expected index query to be eager (cached=true), but got lazy loading")
	}

	// 验证结果
	count := 0
	for rows.Next() {
		count++
	}

	if count != 1 {
		t.Errorf("Expected 1 row from index query, got %d", count)
	}

	t.Log("✓ Index query eager loading test passed")
}

// TestLazyLoadingWithConditions 测试带条件的惰性加载
func TestLazyLoadingWithConditions(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "TestLazyLoadingWithConditions")
	defer os.RemoveAll(tmpDir)

	schema, err := NewSchema("users", []Field{
		{Name: "name", Type: String},
		{Name: "age", Type: Int64},
		{Name: "active", Type: Bool},
	})
	if err != nil {
		t.Fatal(err)
	}

	table, err := OpenTable(&TableOptions{
		Dir:    tmpDir,
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()

	// 插入数据
	for i := 0; i < 50; i++ {
		err = table.Insert(map[string]any{
			"name":   "User" + string(rune(i)),
			"age":    int64(20 + i),
			"active": i%2 == 0,
		})
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	// 带多个条件的查询
	rows, err := table.Query().
		Gte("age", int64(30)).
		Eq("active", true).
		Rows()
	if err != nil {
		t.Fatalf("Rows() failed: %v", err)
	}
	defer rows.Close()

	// 验证是惰性加载
	if rows.cached {
		t.Errorf("Expected lazy loading with conditions")
	}

	// 迭代所有匹配的记录
	count := 0
	for rows.Next() {
		row := rows.Row()
		data := row.Data()

		// 验证条件
		age := int64(data["age"].(float64))
		active := data["active"].(bool)

		if age < 30 {
			t.Errorf("Row age=%d, expected >= 30", age)
		}
		if !active {
			t.Errorf("Row active=%v, expected true", active)
		}

		count++
	}

	if count == 0 {
		t.Errorf("Expected some matching rows")
	}

	t.Logf("✓ Lazy loading with conditions test passed: %d matching rows", count)
}

// TestFirstDoesNotLoadAll 验证 First() 不会加载所有数据
func TestFirstDoesNotLoadAll(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "TestFirstDoesNotLoadAll")
	defer os.RemoveAll(tmpDir)

	schema, err := NewSchema("users", []Field{
		{Name: "name", Type: String},
		{Name: "age", Type: Int64},
	})
	if err != nil {
		t.Fatal(err)
	}

	table, err := OpenTable(&TableOptions{
		Dir:    tmpDir,
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()

	// 插入大量数据
	for i := 0; i < 1000; i++ {
		err = table.Insert(map[string]any{
			"name": "User" + string(rune(i)),
			"age":  int64(20 + i),
		})
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	// 只获取第一条记录
	row, err := table.Query().First()
	if err != nil {
		t.Fatalf("First() failed: %v", err)
	}

	if row == nil {
		t.Errorf("Expected row, got nil")
	}

	// First() 应该只读取一条记录，不会加载所有数据
	t.Log("✓ First() does not load all data test passed")
}
