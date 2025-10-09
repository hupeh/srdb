package main

import (
	"fmt"
	"log"

	"code.tczkiot.com/wlw/srdb"
	"github.com/shopspring/decimal"
)

// User 用户结构体，展示 struct tag 的完整使用
type User struct {
	// 基本字段（必填）
	Username string `srdb:"username;indexed;comment:用户名（索引）"`
	Age      int64  `srdb:"age;comment:年龄"`

	// 可选字段（nullable）
	Email       string `srdb:"email;nullable;comment:邮箱（可选）"`
	PhoneNumber string `srdb:"phone_number;nullable;indexed;comment:手机号（可空且索引）"`
	Bio         string `srdb:"bio;nullable;comment:个人简介（可选）"`
	Avatar      string `srdb:"avatar;nullable;comment:头像 URL（可选）"`

	// 财务字段
	Balance decimal.Decimal `srdb:"balance;nullable;comment:账户余额（可空）"`

	// 布尔字段
	IsActive bool `srdb:"is_active;comment:是否激活"`

	// 忽略字段
	internalData string `srdb:"-"` // 未导出字段会自动忽略
	TempData     string `srdb:"-"` // 使用 "-" 显式忽略导出字段
}

func main() {
	fmt.Println("=== SRDB Struct Tags Example ===\n")

	// 1. 从结构体生成 Schema
	fmt.Println("1. 从结构体生成 Schema")
	fields, err := srdb.StructToFields(User{})
	if err != nil {
		log.Fatalf("StructToFields failed: %v", err)
	}

	schema, err := srdb.NewSchema("users", fields)
	if err != nil {
		log.Fatalf("NewSchema failed: %v", err)
	}

	fmt.Printf("Schema 名称: %s\n", schema.Name)
	fmt.Printf("字段数量: %d\n\n", len(schema.Fields))

	// 打印所有字段
	fmt.Println("字段详情:")
	for _, field := range schema.Fields {
		fmt.Printf("  - %s: Type=%s, Indexed=%v, Nullable=%v",
			field.Name, field.Type, field.Indexed, field.Nullable)
		if field.Comment != "" {
			fmt.Printf(", Comment=%q", field.Comment)
		}
		fmt.Println()
	}
	fmt.Println()

	// 2. 创建数据库和表
	fmt.Println("2. 创建数据库和表")
	db, err := srdb.Open("./data")
	if err != nil {
		log.Fatalf("Open database failed: %v", err)
	}
	defer db.Close()

	table, err := db.CreateTable("users", schema)
	if err != nil {
		log.Fatalf("CreateTable failed: %v", err)
	}
	fmt.Println("✓ 表创建成功\n")

	// 3. 插入数据 - 完整数据（所有字段都有值）
	fmt.Println("3. 插入完整数据")
	avatar1 := "https://example.com/avatar1.png"
	err = table.Insert(map[string]any{
		"username":     "alice",
		"age":          int64(25),
		"email":        "alice@example.com",
		"phone_number": "13800138001",
		"bio":          "Software Engineer",
		"avatar":       avatar1,
		"balance":      decimal.NewFromFloat(1000.50),
		"is_active":    true,
	})
	if err != nil {
		log.Fatalf("Insert failed: %v", err)
	}
	fmt.Println("✓ 插入用户 alice（完整数据）")

	// 4. 插入数据 - 部分字段为 NULL
	fmt.Println("\n4. 插入部分数据（可选字段为 NULL）")
	err = table.Insert(map[string]any{
		"username":  "bob",
		"age":       int64(30),
		"email":     nil, // NULL 值
		"bio":       nil, // NULL 值
		"balance":   nil, // NULL 值
		"is_active": true,
	})
	if err != nil {
		log.Fatalf("Insert failed: %v", err)
	}
	fmt.Println("✓ 插入用户 bob（email、bio、balance 为 NULL）")

	// 5. 插入数据 - 必填字段不能为 NULL
	fmt.Println("\n5. 测试必填字段不能为 NULL")
	err = table.Insert(map[string]any{
		"username":  nil, // 尝试将必填字段设为 NULL
		"age":       int64(28),
		"is_active": true,
	})
	if err != nil {
		fmt.Printf("✓ 符合预期的错误: %v\n", err)
	} else {
		log.Fatal("应该返回错误，但成功了！")
	}

	// 6. 查询所有数据
	fmt.Println("\n6. 查询所有用户")
	rows, err := table.Query().Rows()
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		row := rows.Row()
		data := row.Data()

		username := data["username"]
		email := data["email"]
		balance := data["balance"]

		fmt.Printf("  用户: %v", username)
		if email == nil {
			fmt.Printf(", 邮箱: <NULL>")
		} else {
			fmt.Printf(", 邮箱: %v", email)
		}
		if balance == nil {
			fmt.Printf(", 余额: <NULL>")
		} else {
			fmt.Printf(", 余额: %v", balance)
		}
		fmt.Println()
	}

	// 7. 按索引字段查询
	fmt.Println("\n7. 按索引字段查询（username='alice'）")
	rows2, err := table.Query().Eq("username", "alice").Rows()
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows2.Close()

	if rows2.Next() {
		row := rows2.Row()
		data := row.Data()
		fmt.Printf("  找到用户: %v, 年龄: %v\n", data["username"], data["age"])
	}

	fmt.Println("\n✅ 所有操作完成!")
	fmt.Println("\nStruct Tag 使用总结:")
	fmt.Println("  - srdb:\"name\"                   # 指定字段名")
	fmt.Println("  - srdb:\"name;indexed\"           # 字段名 + 索引")
	fmt.Println("  - srdb:\"name;nullable\"          # 字段名 + 可空")
	fmt.Println("  - srdb:\"name;comment:注释\"      # 字段名 + 注释")
	fmt.Println("  - srdb:\"name;indexed;nullable;comment:XX\"  # 完整格式")
	fmt.Println("  - srdb:\"-\"                      # 忽略字段")
}
