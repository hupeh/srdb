package srdb

import (
	"fmt"
	"os"
	"testing"
)

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
	usersSchema := NewSchema("users", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: true, Comment: "User ID"},
		{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "Name"},
	})
	usersTable, err := db.CreateTable("users", usersSchema)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 50; i++ {
		usersTable.Insert(map[string]any{
			"id":   int64(i),
			"name": "user" + string(rune(i)),
		})
	}

	// 表 2: orders
	ordersSchema := NewSchema("orders", []Field{
		{Name: "order_id", Type: FieldTypeInt64, Indexed: true, Comment: "Order ID"},
		{Name: "amount", Type: FieldTypeInt64, Indexed: false, Comment: "Amount"},
	})
	ordersTable, err := db.CreateTable("orders", ordersSchema)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 30; i++ {
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

	schema := NewSchema("test", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
	})
	table, err := db.CreateTable("test", schema)
	if err != nil {
		t.Fatal(err)
	}

	// 插入数据
	for i := 0; i < 20; i++ {
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
	for i := 0; i < 5; i++ {
		tableName := fmt.Sprintf("table%d", i)
		schema := NewSchema(tableName, []Field{
			{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
			{Name: "value", Type: FieldTypeString, Indexed: false, Comment: "Value"},
		})

		table, err := db.CreateTable(tableName, schema)
		if err != nil {
			t.Fatal(err)
		}

		// 每个表插入 10 条数据
		for j := 0; j < 10; j++ {
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

	schema := NewSchema("test", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: false, Comment: "ID"},
	})
	table, err := db.CreateTable("test", schema)
	if err != nil {
		t.Fatal(err)
	}

	// 插入数据
	for i := 0; i < 50; i++ {
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
