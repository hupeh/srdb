package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"code.tczkiot.com/wlw/srdb"
)

// User 用户结构体
type User struct {
	Name     string `srdb:"name;comment:用户名"`
	Age      int64  `srdb:"age;comment:年龄"`
	Email    string `srdb:"email;indexed;comment:邮箱"`
	IsActive bool   `srdb:"is_active;comment:是否激活"`
}

// Product 产品结构体（使用默认 snake_case 转换）
type Product struct {
	ProductID   string  `srdb:";comment:产品ID"`  // 自动转为 product_id
	ProductName string  `srdb:";comment:产品名称"`  // 自动转为 product_name
	Price       float64 `srdb:";comment:价格"`    // 自动转为 price
	InStock     bool    `srdb:";comment:是否有货"` // 自动转为 in_stock
}

func main() {
	fmt.Println("=== SRDB 批量插入示例 ===")

	// 清理旧数据
	os.RemoveAll("./data")

	// 示例 1: 插入单个 map
	example1()

	// 示例 2: 批量插入 map 切片
	example2()

	// 示例 3: 插入单个结构体
	example3()

	// 示例 4: 批量插入结构体切片
	example4()

	// 示例 5: 批量插入结构体指针切片
	example5()

	// 示例 6: 使用 snake_case 自动转换
	example6()

	fmt.Println("\n✓ 所有示例执行成功！")
}

func example1() {
	fmt.Println("=== 示例 1: 插入单个 map ===")

	schema, err := srdb.NewSchema("users", []srdb.Field{
		{Name: "name", Type: srdb.String, Comment: "用户名"},
		{Name: "age", Type: srdb.Int64, Comment: "年龄"},
	})
	if err != nil {
		log.Fatal(err)
	}

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/example1",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 插入单条数据
	err = table.Insert(map[string]any{
		"name": "Alice",
		"age":  int64(25),
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("✓ 插入 1 条数据")

	// 查询
	row, _ := table.Get(1)
	fmt.Printf("  查询结果: name=%s, age=%d\n\n", row.Data["name"], row.Data["age"])
}

func example2() {
	fmt.Println("=== 示例 2: 批量插入 map 切片 ===")

	schema, err := srdb.NewSchema("users", []srdb.Field{
		{Name: "name", Type: srdb.String},
		{Name: "age", Type: srdb.Int64},
		{Name: "email", Type: srdb.String, Indexed: true},
	})
	if err != nil {
		log.Fatal(err)
	}

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/example2",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 批量插入
	start := time.Now()
	err = table.Insert([]map[string]any{
		{"name": "Alice", "age": int64(25), "email": "alice@example.com"},
		{"name": "Bob", "age": int64(30), "email": "bob@example.com"},
		{"name": "Charlie", "age": int64(35), "email": "charlie@example.com"},
		{"name": "David", "age": int64(40), "email": "david@example.com"},
		{"name": "Eve", "age": int64(45), "email": "eve@example.com"},
	})
	if err != nil {
		log.Fatal(err)
	}
	elapsed := time.Since(start)

	fmt.Printf("✓ 批量插入 5 条数据，耗时: %v\n", elapsed)

	// 使用索引查询
	rows, _ := table.Query().Eq("email", "bob@example.com").Rows()
	defer rows.Close()
	if rows.Next() {
		row := rows.Row()
		data := row.Data()
		fmt.Printf("  索引查询结果: name=%s, email=%s\n\n", data["name"], data["email"])
	}
}

func example3() {
	fmt.Println("=== 示例 3: 插入单个结构体 ===")

	schema, err := srdb.NewSchema("users", []srdb.Field{
		{Name: "name", Type: srdb.String},
		{Name: "age", Type: srdb.Int64},
		{Name: "email", Type: srdb.String},
		{Name: "is_active", Type: srdb.Bool},
	})
	if err != nil {
		log.Fatal(err)
	}

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/example3",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 插入结构体
	user := User{
		Name:     "Alice",
		Age:      25,
		Email:    "alice@example.com",
		IsActive: true,
	}

	err = table.Insert(user)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("✓ 插入 1 个结构体")

	// 查询
	row, _ := table.Get(1)
	fmt.Printf("  查询结果: name=%s, age=%d, active=%v\n\n",
		row.Data["name"], row.Data["age"], row.Data["is_active"])
}

func example4() {
	fmt.Println("=== 示例 4: 批量插入结构体切片 ===")

	schema, err := srdb.NewSchema("users", []srdb.Field{
		{Name: "name", Type: srdb.String},
		{Name: "age", Type: srdb.Int64},
		{Name: "email", Type: srdb.String},
		{Name: "is_active", Type: srdb.Bool},
	})
	if err != nil {
		log.Fatal(err)
	}

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/example4",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 批量插入结构体切片
	users := []User{
		{Name: "Alice", Age: 25, Email: "alice@example.com", IsActive: true},
		{Name: "Bob", Age: 30, Email: "bob@example.com", IsActive: true},
		{Name: "Charlie", Age: 35, Email: "charlie@example.com", IsActive: false},
	}

	start := time.Now()
	err = table.Insert(users)
	if err != nil {
		log.Fatal(err)
	}
	elapsed := time.Since(start)

	fmt.Printf("✓ 批量插入 %d 个结构体，耗时: %v\n", len(users), elapsed)

	// 查询所有激活用户
	rows, _ := table.Query().Eq("is_active", true).Rows()
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	fmt.Printf("  查询结果: 找到 %d 个激活用户\n\n", count)
}

func example5() {
	fmt.Println("=== 示例 5: 批量插入结构体指针切片 ===")

	schema, err := srdb.NewSchema("users", []srdb.Field{
		{Name: "name", Type: srdb.String},
		{Name: "age", Type: srdb.Int64},
		{Name: "email", Type: srdb.String},
		{Name: "is_active", Type: srdb.Bool},
	})
	if err != nil {
		log.Fatal(err)
	}

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/example5",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 批量插入结构体指针切片
	users := []*User{
		{Name: "Alice", Age: 25, Email: "alice@example.com", IsActive: true},
		{Name: "Bob", Age: 30, Email: "bob@example.com", IsActive: true},
		nil, // nil 指针会被自动跳过
		{Name: "Charlie", Age: 35, Email: "charlie@example.com", IsActive: false},
	}

	err = table.Insert(users)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("✓ 批量插入 %d 个结构体指针（nil 自动跳过）\n", len(users))

	// 验证插入数量
	row, _ := table.Get(3)
	fmt.Printf("  实际插入: 3 条数据, 最后一条 name=%s\n\n", row.Data["name"])
}

func example6() {
	fmt.Println("=== 示例 6: 使用 snake_case 自动转换 ===")

	schema, err := srdb.NewSchema("products", []srdb.Field{
		{Name: "product_id", Type: srdb.String, Comment: "产品ID"},
		{Name: "product_name", Type: srdb.String, Comment: "产品名称"},
		{Name: "price", Type: srdb.Float64, Comment: "价格"},
		{Name: "in_stock", Type: srdb.Bool, Comment: "是否有货"},
	})
	if err != nil {
		log.Fatal(err)
	}

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/example6",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 结构体字段名是驼峰命名，会自动转为 snake_case
	products := []Product{
		{ProductID: "P001", ProductName: "Laptop", Price: 999.99, InStock: true},
		{ProductID: "P002", ProductName: "Mouse", Price: 29.99, InStock: true},
		{ProductID: "P003", ProductName: "Keyboard", Price: 79.99, InStock: false},
	}

	err = table.Insert(products)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("✓ 批量插入 %d 个产品（自动 snake_case 转换）\n", len(products))

	// 查询
	row, _ := table.Get(1)
	fmt.Printf("  字段名自动转换:\n")
	fmt.Printf("    ProductID   -> product_id   = %s\n", row.Data["product_id"])
	fmt.Printf("    ProductName -> product_name = %s\n", row.Data["product_name"])
	fmt.Printf("    Price       -> price        = %.2f\n", row.Data["price"])
	fmt.Printf("    InStock     -> in_stock     = %v\n\n", row.Data["in_stock"])
}
