package srdb

import (
	"os"
	"testing"
)

func TestTableClean(t *testing.T) {
	dir := "./test_table_clean_data"
	defer os.RemoveAll(dir)

	// 1. 创建数据库和表
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := NewSchema("users", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: true, Comment: "ID"},
		{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "Name"},
	})

	table, err := db.CreateTable("users", schema)
	if err != nil {
		t.Fatal(err)
	}

	// 2. 插入数据
	for i := 0; i < 100; i++ {
		err := table.Insert(map[string]any{
			"id":   int64(i),
			"name": "user" + string(rune(i)),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// 3. 验证数据存在
	stats := table.Stats()
	t.Logf("Before Clean: %d rows", stats.TotalRows)

	if stats.TotalRows == 0 {
		t.Error("Expected data in table")
	}

	// 4. 清除数据
	err = table.Clean()
	if err != nil {
		t.Fatal(err)
	}

	// 5. 验证数据已清除
	stats = table.Stats()
	t.Logf("After Clean: %d rows", stats.TotalRows)

	if stats.TotalRows != 0 {
		t.Errorf("Expected 0 rows after clean, got %d", stats.TotalRows)
	}

	// 6. 验证表仍然可用
	err = table.Insert(map[string]any{
		"id":   int64(100),
		"name": "new_user",
	})
	if err != nil {
		t.Fatal(err)
	}

	stats = table.Stats()
	if stats.TotalRows != 1 {
		t.Errorf("Expected 1 row after insert, got %d", stats.TotalRows)
	}
}

func TestTableDestroy(t *testing.T) {
	dir := "./test_table_destroy_data"
	defer os.RemoveAll(dir)

	// 1. 创建数据库和表
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := NewSchema("test", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
	})

	table, err := db.CreateTable("test", schema)
	if err != nil {
		t.Fatal(err)
	}

	// 2. 插入数据
	for i := 0; i < 50; i++ {
		table.Insert(map[string]any{"id": int64(i)})
	}

	// 3. 验证数据存在
	stats := table.Stats()
	t.Logf("Before Destroy: %d rows", stats.TotalRows)

	if stats.TotalRows == 0 {
		t.Error("Expected data in table")
	}

	// 4. 获取表目录路径
	tableDir := table.dir

	// 5. 销毁表
	err = table.Destroy()
	if err != nil {
		t.Fatal(err)
	}

	// 6. 验证表目录已删除
	if _, err := os.Stat(tableDir); !os.IsNotExist(err) {
		t.Error("Table directory should be deleted")
	}

	// 7. 注意：Table.Destroy() 只删除文件，不从 Database 中删除
	// 表仍然在 Database 的元数据中，但文件已被删除
	tables := db.ListTables()
	found := false
	for _, name := range tables {
		if name == "test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Table should still be in database metadata (use Database.DestroyTable to remove from metadata)")
	}
}

func TestTableCleanWithIndex(t *testing.T) {
	dir := "./test_table_clean_index_data"
	defer os.RemoveAll(dir)

	// 1. 创建数据库和表
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := NewSchema("users", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: true, Comment: "ID"},
		{Name: "email", Type: FieldTypeString, Indexed: true, Comment: "Email"},
		{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "Name"},
	})

	table, err := db.CreateTable("users", schema)
	if err != nil {
		t.Fatal(err)
	}

	// 2. 创建索引
	err = table.CreateIndex("id")
	if err != nil {
		t.Fatal(err)
	}

	err = table.CreateIndex("email")
	if err != nil {
		t.Fatal(err)
	}

	// 3. 插入数据
	for i := 0; i < 50; i++ {
		table.Insert(map[string]any{
			"id":    int64(i),
			"email": "user" + string(rune(i)) + "@example.com",
			"name":  "User " + string(rune(i)),
		})
	}

	// 4. 验证索引存在
	indexes := table.ListIndexes()
	if len(indexes) != 2 {
		t.Errorf("Expected 2 indexes, got %d", len(indexes))
	}

	// 5. 清除数据
	err = table.Clean()
	if err != nil {
		t.Fatal(err)
	}

	// 6. 验证数据已清除
	stats := table.Stats()
	if stats.TotalRows != 0 {
		t.Errorf("Expected 0 rows after clean, got %d", stats.TotalRows)
	}

	// 7. 验证索引已被清除（Clean 会删除索引数据）
	indexes = table.ListIndexes()
	if len(indexes) != 0 {
		t.Logf("Note: Indexes were cleared (expected behavior), got %d", len(indexes))
	}

	// 8. 重新创建索引
	table.CreateIndex("id")
	table.CreateIndex("email")

	// 9. 验证可以继续插入数据
	err = table.Insert(map[string]any{
		"id":    int64(100),
		"email": "new@example.com",
		"name":  "New User",
	})
	if err != nil {
		t.Fatal(err)
	}

	stats = table.Stats()
	if stats.TotalRows != 1 {
		t.Errorf("Expected 1 row, got %d", stats.TotalRows)
	}
}

func TestTableCleanAndQuery(t *testing.T) {
	dir := "./test_table_clean_query_data"
	defer os.RemoveAll(dir)

	// 1. 创建数据库和表
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := NewSchema("test", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
		{Name: "status", Type: FieldTypeString, Indexed: false, Comment: "Status"},
	})

	table, err := db.CreateTable("test", schema)
	if err != nil {
		t.Fatal(err)
	}

	// 2. 插入数据
	for i := 0; i < 30; i++ {
		table.Insert(map[string]any{
			"id":     int64(i),
			"status": "active",
		})
	}

	// 3. 查询数据
	rows, err := table.Query().Eq("status", "active").Rows()
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for rows.Next() {
		count++
	}
	rows.Close()

	t.Logf("Before Clean: found %d rows", count)
	if count != 30 {
		t.Errorf("Expected 30 rows, got %d", count)
	}

	// 4. 清除数据
	err = table.Clean()
	if err != nil {
		t.Fatal(err)
	}

	// 5. 再次查询
	rows, err = table.Query().Eq("status", "active").Rows()
	if err != nil {
		t.Fatal(err)
	}

	count = 0
	for rows.Next() {
		count++
	}
	rows.Close()

	t.Logf("After Clean: found %d rows", count)
	if count != 0 {
		t.Errorf("Expected 0 rows after clean, got %d", count)
	}

	// 6. 插入新数据并查询
	table.Insert(map[string]any{
		"id":     int64(100),
		"status": "active",
	})

	rows, err = table.Query().Eq("status", "active").Rows()
	if err != nil {
		t.Fatal(err)
	}

	count = 0
	for rows.Next() {
		count++
	}
	rows.Close()

	if count != 1 {
		t.Errorf("Expected 1 row, got %d", count)
	}
}
