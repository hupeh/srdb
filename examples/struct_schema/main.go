package main

import (
	"fmt"
	"log"

	"code.tczkiot.com/wlw/srdb"
)

// User 用户结构体
// 使用 struct tags 定义 Schema
type User struct {
	Name     string  `srdb:"name;indexed;comment:用户名"`
	Age      int64   `srdb:"age;comment:年龄"`
	Email    string  `srdb:"email;indexed;comment:邮箱"`
	Score    float64 `srdb:"score;comment:分数"`
	IsActive bool    `srdb:"is_active;comment:是否激活"`
	Internal string  `srdb:"-"` // 不会被包含在 Schema 中
}

// Product 产品结构体
// 不使用 srdb tag，字段名会自动转为 snake_case
type Product struct {
	ProductID   string  // 字段名: product_id
	ProductName string  // 字段名: product_name
	Price       int64   // 字段名: price
	InStock     bool    // 字段名: in_stock
}

func main() {
	// 示例 1: 使用结构体创建 Schema
	fmt.Println("=== 示例 1: 从结构体创建 Schema ===")

	// 从 User 结构体生成 Field 列表
	fields, err := srdb.StructToFields(User{})
	if err != nil {
		log.Fatal(err)
	}

	// 创建 Schema
	schema, err := srdb.NewSchema("users", fields)
	if err != nil {
		log.Fatal(err)
	}

	// 打印 Schema 信息
	fmt.Printf("Schema 名称: %s\n", schema.Name)
	fmt.Printf("字段数量: %d\n", len(schema.Fields))
	fmt.Println("\n字段列表:")
	for _, field := range schema.Fields {
		indexed := ""
		if field.Indexed {
			indexed = " [索引]"
		}
		fmt.Printf("  - %s (%s)%s: %s\n",
			field.Name, field.Type.String(), indexed, field.Comment)
	}

	// 示例 2: 使用 Schema 创建表
	fmt.Println("\n=== 示例 2: 使用 Schema 创建表 ===")

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/users",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 插入数据
	err = table.Insert(map[string]any{
		"name":      "张三",
		"age":       int64(25),
		"email":     "zhangsan@example.com",
		"score":     95.5,
		"is_active": true,
	})
	if err != nil {
		log.Fatal(err)
	}

	err = table.Insert(map[string]any{
		"name":      "李四",
		"age":       int64(30),
		"email":     "lisi@example.com",
		"score":     88.0,
		"is_active": true,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("✓ 插入 2 条数据")

	// 查询数据
	rows, err := table.Query().Eq("email", "zhangsan@example.com").Rows()
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("\n查询结果 (email = zhangsan@example.com):")
	for rows.Next() {
		data := rows.Row().Data()
		fmt.Printf("  姓名: %s, 年龄: %v, 邮箱: %s, 分数: %v, 激活: %v\n",
			data["name"], data["age"], data["email"], data["score"], data["is_active"])
	}

	// 示例 3: 使用默认字段名（snake_case）
	fmt.Println("\n=== 示例 3: 使用默认字段名（snake_case）===")

	productFields, err := srdb.StructToFields(Product{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Product 字段（使用默认 snake_case 名称）:")
	for _, field := range productFields {
		fmt.Printf("  - %s (%s)\n", field.Name, field.Type.String())
	}

	// 示例 4: 获取索引字段
	fmt.Println("\n=== 示例 4: 获取索引字段 ===")
	indexedFields := schema.GetIndexedFields()
	fmt.Printf("User Schema 中的索引字段（共 %d 个）:\n", len(indexedFields))
	for _, field := range indexedFields {
		fmt.Printf("  - %s: %s\n", field.Name, field.Comment)
	}

	fmt.Println("\n✓ 所有示例执行成功！")
}
