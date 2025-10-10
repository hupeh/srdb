package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"code.tczkiot.com/wlw/srdb"
)

// User 使用指针类型表示 nullable 字段
type User struct {
	ID        uint32    `srdb:"field:id"`
	Name      string    `srdb:"field:name"`
	Email     *string   `srdb:"field:email;comment:邮箱（可选）"`
	Phone     *string   `srdb:"field:phone;comment:手机号（可选）"`
	Age       *int32    `srdb:"field:age;comment:年龄（可选）"`
	CreatedAt time.Time `srdb:"field:created_at"`
}

// Product 商品表
type Product struct {
	ID          uint32     `srdb:"field:id"`
	Name        string     `srdb:"field:name;indexed"`
	Price       *float64   `srdb:"field:price;comment:价格（可选）"`
	Stock       *int32     `srdb:"field:stock;comment:库存（可选）"`
	Description *string    `srdb:"field:description;comment:描述（可选）"`
	CreatedAt   time.Time  `srdb:"field:created_at"`
}

func main() {
	fmt.Println("=== Nullable 字段测试（指针类型） ===\n")

	dataPath := "./nullable_data"
	os.RemoveAll(dataPath)
	defer os.RemoveAll(dataPath)

	db, err := srdb.Open(dataPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	// ==================== 测试 1: 用户表 ====================
	fmt.Println("【测试 1】用户表（指针类型）")
	fmt.Println("─────────────────────────────")

	userFields, err := srdb.StructToFields(User{})
	if err != nil {
		log.Fatalf("生成 User 字段失败: %v", err)
	}

	fmt.Println("User Schema 字段:")
	for _, f := range userFields {
		fmt.Printf("  - %s (%s)", f.Name, f.Type)
		if f.Nullable {
			fmt.Print(" [nullable]")
		}
		if f.Comment != "" {
			fmt.Printf(" // %s", f.Comment)
		}
		fmt.Println()
	}

	userSchema, err := srdb.NewSchema("users", userFields)
	if err != nil {
		log.Fatalf("创建 User Schema 失败: %v", err)
	}

	userTable, err := db.CreateTable("users", userSchema)
	if err != nil {
		log.Fatalf("创建 User 表失败: %v", err)
	}

	// 插入数据（所有字段都有值）
	fmt.Println("\n插入用户数据:")
	err = userTable.Insert(map[string]any{
		"id":         uint32(1),
		"name":       "Alice",
		"email":      "alice@example.com",
		"phone":      "13800138000",
		"age":        int32(25),
		"created_at": time.Now(),
	})
	if err != nil {
		log.Fatalf("插入用户失败: %v", err)
	}
	fmt.Println("  ✓ Alice (所有字段都有值)")

	// 插入数据（部分字段为 NULL）
	err = userTable.Insert(map[string]any{
		"id":         uint32(2),
		"name":       "Bob",
		"email":      nil,  // NULL
		"phone":      "13900139000",
		"age":        nil,  // NULL
		"created_at": time.Now(),
	})
	if err != nil {
		log.Fatalf("插入用户失败: %v", err)
	}
	fmt.Println("  ✓ Bob (email 和 age 为 NULL)")

	// 插入数据（全部可选字段为 NULL）
	err = userTable.Insert(map[string]any{
		"id":         uint32(3),
		"name":       "Charlie",
		"email":      nil,
		"phone":      nil,
		"age":        nil,
		"created_at": time.Now(),
	})
	if err != nil {
		log.Fatalf("插入用户失败: %v", err)
	}
	fmt.Println("  ✓ Charlie (所有可选字段都为 NULL)")

	// 查询
	fmt.Println("\n查询结果:")
	rows, err := userTable.Query().Rows()
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		data := rows.Row().Data()

		fmt.Printf("  - %s:", data["name"])

		if email := data["email"]; email != nil {
			fmt.Printf(" email=%s", email)
		} else {
			fmt.Print(" email=<NULL>")
		}

		if phone := data["phone"]; phone != nil {
			fmt.Printf(", phone=%s", phone)
		} else {
			fmt.Print(", phone=<NULL>")
		}

		if age := data["age"]; age != nil {
			fmt.Printf(", age=%d", age)
		} else {
			fmt.Print(", age=<NULL>")
		}

		fmt.Println()
	}

	// ==================== 测试 2: 商品表 ====================
	fmt.Println("\n【测试 2】商品表（指针类型）")
	fmt.Println("─────────────────────────────")

	productFields, err := srdb.StructToFields(Product{})
	if err != nil {
		log.Fatalf("生成 Product 字段失败: %v", err)
	}

	fmt.Println("Product Schema 字段:")
	for _, f := range productFields {
		fmt.Printf("  - %s (%s)", f.Name, f.Type)
		if f.Nullable {
			fmt.Print(" [nullable]")
		}
		if f.Indexed {
			fmt.Print(" [indexed]")
		}
		if f.Comment != "" {
			fmt.Printf(" // %s", f.Comment)
		}
		fmt.Println()
	}

	productSchema, err := srdb.NewSchema("products", productFields)
	if err != nil {
		log.Fatalf("创建 Product Schema 失败: %v", err)
	}

	productTable, err := db.CreateTable("products", productSchema)
	if err != nil {
		log.Fatalf("创建 Product 表失败: %v", err)
	}

	// 插入商品
	fmt.Println("\n插入商品数据:")
	err = productTable.Insert(map[string]any{
		"id":          uint32(101),
		"name":        "iPhone 15",
		"price":       6999.0,
		"stock":       int32(50),
		"description": "最新款智能手机",
		"created_at":  time.Now(),
	})
	if err != nil {
		log.Fatalf("插入商品失败: %v", err)
	}
	fmt.Println("  ✓ iPhone 15 (所有字段都有值)")

	// 待定商品（价格和库存未定）
	err = productTable.Insert(map[string]any{
		"id":          uint32(102),
		"name":        "新品预告",
		"price":       nil,  // 价格未定
		"stock":       nil,  // 库存未定
		"description": "即将发布",
		"created_at":  time.Now(),
	})
	if err != nil {
		log.Fatalf("插入商品失败: %v", err)
	}
	fmt.Println("  ✓ 新品预告 (price 和 stock 为 NULL)")

	// 查询商品
	fmt.Println("\n查询结果:")
	rows2, err := productTable.Query().Rows()
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		data := rows2.Row().Data()

		fmt.Printf("  - %s:", data["name"])

		if price := data["price"]; price != nil {
			fmt.Printf(" price=%.2f", price)
		} else {
			fmt.Print(" price=<未定价>")
		}

		if stock := data["stock"]; stock != nil {
			fmt.Printf(", stock=%d", stock)
		} else {
			fmt.Print(", stock=<未定>")
		}

		if desc := data["description"]; desc != nil {
			fmt.Printf(", desc=%s", desc)
		}

		fmt.Println()
	}

	// ==================== 测试 3: 使用索引查询 ====================
	fmt.Println("\n【测试 3】使用索引查询")
	fmt.Println("─────────────────────────────")

	// 再插入几个商品
	productTable.Insert(map[string]any{
		"id":         uint32(103),
		"name":       "MacBook Pro",
		"price":      12999.0,
		"stock":      int32(20),
		"created_at": time.Now(),
	})

	productTable.Insert(map[string]any{
		"id":         uint32(104),
		"name":       "iPad Air",
		"price":      4999.0,
		"stock":      nil,  // 缺货
		"created_at": time.Now(),
	})

	// 按名称查询（使用索引）
	fmt.Println("\n按名称查询 'iPhone 15':")
	rows3, err := productTable.Query().Eq("name", "iPhone 15").Rows()
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	defer rows3.Close()

	for rows3.Next() {
		data := rows3.Row().Data()
		fmt.Printf("  找到: %s, price=%.2f\n", data["name"], data["price"])
	}

	fmt.Println("\n=== 测试完成 ===")
	fmt.Println("\n✨ Nullable 支持总结:")
	fmt.Println("  • 使用指针类型 (*string, *int32, ...) 表示 nullable 字段")
	fmt.Println("  • StructToFields 自动识别指针类型并设置 nullable=true")
	fmt.Println("  • 插入时直接传值或 nil")
	fmt.Println("  • 查询时检查字段是否为 nil")
	fmt.Println("  • 简单、直观、符合 Go 习惯")
}
