package srdb

import (
	"fmt"
	"os"
	"slices"
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
	userSchema, err := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
	})
	if err != nil {
		t.Fatal(err)
	}

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
	userSchema, err := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
	})
	if err != nil {
		t.Fatal(err)
	}

	orderSchema, err := NewSchema("orders", []Field{
		{Name: "order_id", Type: FieldTypeString, Indexed: true, Comment: "订单ID"},
		{Name: "amount", Type: FieldTypeInt64, Indexed: true, Comment: "金额"},
	})
	if err != nil {
		t.Fatal(err)
	}

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
	userSchema, err := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
	})
	if err != nil {
		t.Fatal(err)
	}

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
	userSchema, err := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
	})
	if err != nil {
		t.Fatal(err)
	}

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

	userSchema, err := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
	})
	if err != nil {
		t.Fatal(err)
	}

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

func TestDatabaseClean(t *testing.T) {
	dir := "./test_db_clean_data"
	defer os.RemoveAll(dir)

	// 1. 创建数据库
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	// 2. 创建多个表并插入数据
	// 表 1: users
	usersSchema, err := NewSchema("users", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: true, Comment: "User ID"},
		{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "Name"},
	})
	if err != nil {
		t.Fatal(err)
	}
	usersTable, err := db.CreateTable("users", usersSchema)
	if err != nil {
		t.Fatal(err)
	}

	for i := range 50 {
		usersTable.Insert(map[string]any{
			"id":   int64(i),
			"name": "user" + string(rune(i)),
		})
	}

	// 表 2: orders
	ordersSchema, err := NewSchema("orders", []Field{
		{Name: "order_id", Type: FieldTypeInt64, Indexed: true, Comment: "Order ID"},
		{Name: "amount", Type: FieldTypeInt64, Indexed: false, Comment: "Amount"},
	})
	if err != nil {
		t.Fatal(err)
	}
	ordersTable, err := db.CreateTable("orders", ordersSchema)
	if err != nil {
		t.Fatal(err)
	}

	for i := range 30 {
		ordersTable.Insert(map[string]any{
			"order_id": int64(i),
			"amount":   int64(i * 100),
		})
	}

	// 3. 验证数据存在
	usersStats := usersTable.Stats()
	ordersStats := ordersTable.Stats()
	t.Logf("Before Clean - Users: %d rows, Orders: %d rows",
		usersStats.TotalRows, ordersStats.TotalRows)

	if usersStats.TotalRows == 0 || ordersStats.TotalRows == 0 {
		t.Error("Expected data in tables")
	}

	// 4. 清除所有表的数据
	err = db.Clean()
	if err != nil {
		t.Fatal(err)
	}

	// 5. 验证数据已清除
	usersStats = usersTable.Stats()
	ordersStats = ordersTable.Stats()
	t.Logf("After Clean - Users: %d rows, Orders: %d rows",
		usersStats.TotalRows, ordersStats.TotalRows)

	if usersStats.TotalRows != 0 {
		t.Errorf("Expected 0 rows in users, got %d", usersStats.TotalRows)
	}
	if ordersStats.TotalRows != 0 {
		t.Errorf("Expected 0 rows in orders, got %d", ordersStats.TotalRows)
	}

	// 6. 验证表结构仍然存在
	tables := db.ListTables()
	if len(tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(tables))
	}

	// 7. 验证可以继续插入数据
	err = usersTable.Insert(map[string]any{
		"id":   int64(100),
		"name": "new_user",
	})
	if err != nil {
		t.Fatal(err)
	}

	usersStats = usersTable.Stats()
	if usersStats.TotalRows != 1 {
		t.Errorf("Expected 1 row after insert, got %d", usersStats.TotalRows)
	}

	db.Close()
}

func TestDatabaseDestroy(t *testing.T) {
	dir := "./test_db_destroy_data"
	defer os.RemoveAll(dir)

	// 1. 创建数据库和表
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	schema, err := NewSchema("test", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
	})
	if err != nil {
		t.Fatal(err)
	}
	table, err := db.CreateTable("test", schema)
	if err != nil {
		t.Fatal(err)
	}

	// 插入数据
	for i := range 20 {
		table.Insert(map[string]any{"id": int64(i)})
	}

	// 2. 验证数据存在
	stats := table.Stats()
	t.Logf("Before Destroy: %d rows", stats.TotalRows)

	if stats.TotalRows == 0 {
		t.Error("Expected data in table")
	}

	// 3. 销毁数据库
	err = db.Destroy()
	if err != nil {
		t.Fatal(err)
	}

	// 4. 验证数据目录已删除
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("Database directory should be deleted")
	}

	// 5. 验证数据库不可用
	tables := db.ListTables()
	if len(tables) != 0 {
		t.Errorf("Expected 0 tables after destroy, got %d", len(tables))
	}
}

func TestDatabaseCleanMultipleTables(t *testing.T) {
	dir := "./test_db_clean_multi_data"
	defer os.RemoveAll(dir)

	// 1. 创建数据库和多个表
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 创建 5 个表
	for i := range 5 {
		tableName := fmt.Sprintf("table%d", i)
		schema, err := NewSchema(tableName, []Field{
			{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
			{Name: "value", Type: FieldTypeString, Indexed: false, Comment: "Value"},
		})
		if err != nil {
			t.Fatal(err)
		}

		table, err := db.CreateTable(tableName, schema)
		if err != nil {
			t.Fatal(err)
		}

		// 每个表插入 10 条数据
		for j := range 10 {
			table.Insert(map[string]any{
				"id":    int64(j),
				"value": fmt.Sprintf("value_%d_%d", i, j),
			})
		}
	}

	// 2. 验证所有表都有数据
	tables := db.ListTables()
	if len(tables) != 5 {
		t.Fatalf("Expected 5 tables, got %d", len(tables))
	}

	totalRows := 0
	for _, tableName := range tables {
		table, _ := db.GetTable(tableName)
		stats := table.Stats()
		totalRows += int(stats.TotalRows)
	}
	t.Logf("Total rows before clean: %d", totalRows)

	if totalRows == 0 {
		t.Error("Expected data in tables")
	}

	// 3. 清除所有表
	err = db.Clean()
	if err != nil {
		t.Fatal(err)
	}

	// 4. 验证所有表数据已清除
	totalRows = 0
	for _, tableName := range tables {
		table, _ := db.GetTable(tableName)
		stats := table.Stats()
		totalRows += int(stats.TotalRows)

		if stats.TotalRows != 0 {
			t.Errorf("Table %s should have 0 rows, got %d", tableName, stats.TotalRows)
		}
	}
	t.Logf("Total rows after clean: %d", totalRows)

	// 5. 验证表结构仍然存在
	tables = db.ListTables()
	if len(tables) != 5 {
		t.Errorf("Expected 5 tables after clean, got %d", len(tables))
	}
}

func TestDatabaseCleanAndReopen(t *testing.T) {
	dir := "./test_db_clean_reopen_data"
	defer os.RemoveAll(dir)

	// 1. 创建数据库和表
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	schema, err := NewSchema("test", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
	})
	if err != nil {
		t.Fatal(err)
	}
	table, err := db.CreateTable("test", schema)
	if err != nil {
		t.Fatal(err)
	}

	// 插入数据
	for i := range 50 {
		table.Insert(map[string]any{"id": int64(i)})
	}

	// 2. 清除数据
	err = db.Clean()
	if err != nil {
		t.Fatal(err)
	}

	// 3. 关闭并重新打开
	db.Close()

	db2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	// 4. 验证表存在但数据为空
	tables := db2.ListTables()
	if len(tables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(tables))
	}

	table2, err := db2.GetTable("test")
	if err != nil {
		t.Fatal(err)
	}

	stats := table2.Stats()
	if stats.TotalRows != 0 {
		t.Errorf("Expected 0 rows after reopen, got %d", stats.TotalRows)
	}

	// 5. 验证可以插入新数据
	err = table2.Insert(map[string]any{"id": int64(100)})
	if err != nil {
		t.Fatal(err)
	}

	stats = table2.Stats()
	if stats.TotalRows != 1 {
		t.Errorf("Expected 1 row, got %d", stats.TotalRows)
	}
}

func TestDatabaseCleanTable(t *testing.T) {
	dir := "./test_db_clean_table_data"
	defer os.RemoveAll(dir)

	// 1. 创建数据库和表
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema, err := NewSchema("users", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
		{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "Name"},
	})
	if err != nil {
		t.Fatal(err)
	}

	table, err := db.CreateTable("users", schema)
	if err != nil {
		t.Fatal(err)
	}

	// 2. 插入数据
	for i := range 50 {
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
	found := slices.Contains(tables, "users")
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

	schema, err := NewSchema("test", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
	})
	if err != nil {
		t.Fatal(err)
	}

	table, err := db.CreateTable("test", schema)
	if err != nil {
		t.Fatal(err)
	}

	// 2. 插入数据
	for i := range 30 {
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

	schema, err := NewSchema("test", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
	})
	if err != nil {
		t.Fatal(err)
	}

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
