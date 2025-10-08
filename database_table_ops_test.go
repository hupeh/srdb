package srdb

import (
	"os"
	"testing"
)

func TestDatabaseCleanTable(t *testing.T) {
	dir := "./test_db_clean_table_data"
	defer os.RemoveAll(dir)

	// 1. 创建数据库和表
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := NewSchema("users", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
		{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "Name"},
	})

	table, err := db.CreateTable("users", schema)
	if err != nil {
		t.Fatal(err)
	}

	// 2. 插入数据
	for i := 0; i < 50; i++ {
		table.Insert(map[string]any{
			"id":   int64(i),
			"name": "user",
		})
	}

	// 3. 验证数据存在
	stats := table.Stats()
	if stats.TotalRows == 0 {
		t.Error("Expected data in table")
	}

	// 4. 清除表数据
	err = db.CleanTable("users")
	if err != nil {
		t.Fatal(err)
	}

	// 5. 验证数据已清除
	stats = table.Stats()
	if stats.TotalRows != 0 {
		t.Errorf("Expected 0 rows after clean, got %d", stats.TotalRows)
	}

	// 6. 验证表仍然存在
	tables := db.ListTables()
	found := false
	for _, name := range tables {
		if name == "users" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Table should still exist after clean")
	}

	// 7. 验证可以继续插入
	err = table.Insert(map[string]any{
		"id":   int64(100),
		"name": "new_user",
	})
	if err != nil {
		t.Fatal(err)
	}

	stats = table.Stats()
	if stats.TotalRows != 1 {
		t.Errorf("Expected 1 row, got %d", stats.TotalRows)
	}
}

func TestDatabaseDestroyTable(t *testing.T) {
	dir := "./test_db_destroy_table_data"
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
	for i := 0; i < 30; i++ {
		table.Insert(map[string]any{"id": int64(i)})
	}

	// 3. 验证数据存在
	stats := table.Stats()
	if stats.TotalRows == 0 {
		t.Error("Expected data in table")
	}

	// 4. 销毁表
	err = db.DestroyTable("test")
	if err != nil {
		t.Fatal(err)
	}

	// 5. 验证表已从 Database 中删除
	tables := db.ListTables()
	for _, name := range tables {
		if name == "test" {
			t.Error("Table should be removed from database")
		}
	}

	// 6. 验证无法再获取该表
	_, err = db.GetTable("test")
	if err == nil {
		t.Error("Should not be able to get table after destroy")
	}
}

func TestDatabaseDestroyTableMultiple(t *testing.T) {
	dir := "./test_db_destroy_multi_data"
	defer os.RemoveAll(dir)

	// 1. 创建数据库和多个表
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := NewSchema("test", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
	})

	// 创建 3 个表
	for i := 1; i <= 3; i++ {
		tableName := "table" + string(rune('0'+i))
		_, err := db.CreateTable(tableName, schema)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 2. 验证有 3 个表
	tables := db.ListTables()
	if len(tables) != 3 {
		t.Fatalf("Expected 3 tables, got %d", len(tables))
	}

	// 3. 销毁中间的表
	err = db.DestroyTable("table2")
	if err != nil {
		t.Fatal(err)
	}

	// 4. 验证只剩 2 个表
	tables = db.ListTables()
	if len(tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(tables))
	}

	// 5. 验证剩余的表是正确的
	hasTable1 := false
	hasTable3 := false
	for _, name := range tables {
		if name == "table1" {
			hasTable1 = true
		}
		if name == "table3" {
			hasTable3 = true
		}
		if name == "table2" {
			t.Error("table2 should be destroyed")
		}
	}

	if !hasTable1 || !hasTable3 {
		t.Error("table1 and table3 should still exist")
	}
}
