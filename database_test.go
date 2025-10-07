package srdb

import (
	"os"
	"testing"
)

func TestDatabaseBasic(t *testing.T) {
	dir := "./test_db"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	// 打开数据库
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 检查初始状态
	tables := db.ListTables()
	if len(tables) != 0 {
		t.Errorf("Expected 0 tables, got %d", len(tables))
	}

	t.Log("Database basic test passed!")
}

func TestCreateTable(t *testing.T) {
	dir := "./test_db_create"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 创建 Schema
	userSchema := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
	})

	// 创建表
	usersTable, err := db.CreateTable("users", userSchema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	if usersTable.GetName() != "users" {
		t.Errorf("Expected table name 'users', got '%s'", usersTable.GetName())
	}

	// 检查表列表
	tables := db.ListTables()
	if len(tables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(tables))
	}

	t.Log("Create table test passed!")
}

func TestMultipleTables(t *testing.T) {
	dir := "./test_db_multiple"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 创建多个表
	userSchema := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
	})

	orderSchema := NewSchema("orders", []Field{
		{Name: "order_id", Type: FieldTypeString, Indexed: true, Comment: "订单ID"},
		{Name: "amount", Type: FieldTypeInt64, Indexed: true, Comment: "金额"},
	})

	_, err = db.CreateTable("users", userSchema)
	if err != nil {
		t.Fatalf("CreateTable users failed: %v", err)
	}

	_, err = db.CreateTable("orders", orderSchema)
	if err != nil {
		t.Fatalf("CreateTable orders failed: %v", err)
	}

	// 检查表列表
	tables := db.ListTables()
	if len(tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(tables))
	}

	t.Log("Multiple tables test passed!")
}

func TestTableOperations(t *testing.T) {
	dir := "./test_db_ops"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 创建表
	userSchema := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
	})

	usersTable, err := db.CreateTable("users", userSchema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	// 插入数据
	err = usersTable.Insert(map[string]any{
		"name": "Alice",
		"age":  int64(25),
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	err = usersTable.Insert(map[string]any{
		"name": "Bob",
		"age":  int64(30),
	})
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// 查询数据
	rows, err := usersTable.Query().Eq("name", "Alice").Rows()
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if rows.Len() != 1 {
		t.Errorf("Expected 1 result, got %d", rows.Len())
	}

	if rows.Data()[0]["name"] != "Alice" {
		t.Errorf("Expected name 'Alice', got '%v'", rows.Data()[0]["name"])
	}

	// 统计
	stats := usersTable.Stats()
	if stats.TotalRows != 2 {
		t.Errorf("Expected 2 rows, got %d", stats.TotalRows)
	}

	t.Log("Table operations test passed!")
}

func TestDropTable(t *testing.T) {
	dir := "./test_db_drop"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// 创建表
	userSchema := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
	})

	_, err = db.CreateTable("users", userSchema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	// 删除表
	err = db.DropTable("users")
	if err != nil {
		t.Fatalf("DropTable failed: %v", err)
	}

	// 检查表列表
	tables := db.ListTables()
	if len(tables) != 0 {
		t.Errorf("Expected 0 tables after drop, got %d", len(tables))
	}

	t.Log("Drop table test passed!")
}

func TestDatabaseRecover(t *testing.T) {
	dir := "./test_db_recover"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	// 第一次：创建数据库和表
	db1, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	userSchema := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
	})

	usersTable, err := db1.CreateTable("users", userSchema)
	if err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	// 插入数据
	usersTable.Insert(map[string]any{
		"name": "Alice",
		"age":  int64(25),
	})

	db1.Close()

	// 第二次：重新打开数据库
	db2, err := Open(dir)
	if err != nil {
		t.Fatalf("Open after recover failed: %v", err)
	}
	defer db2.Close()

	// 检查表是否恢复
	tables := db2.ListTables()
	if len(tables) != 1 {
		t.Errorf("Expected 1 table after recover, got %d", len(tables))
	}

	// 获取表
	usersTable2, err := db2.GetTable("users")
	if err != nil {
		t.Fatalf("GetTable failed: %v", err)
	}

	// 检查数据是否恢复
	stats := usersTable2.Stats()
	if stats.TotalRows != 1 {
		t.Errorf("Expected 1 row after recover, got %d", stats.TotalRows)
	}

	t.Log("Database recover test passed!")
}
