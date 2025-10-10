package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"code.tczkiot.com/wlw/srdb"
)

// 使用新的 tag 格式（顺序无关）
type Product struct {
	ID          uint32        `srdb:"field:id;comment:商品ID"`
	Name        string        `srdb:"comment:商品名称;field:name;indexed"`
	Price       float64       `srdb:"field:price;nullable;comment:价格"`
	Stock       int32         `srdb:"indexed;field:stock;comment:库存数量"`
	Category    string        `srdb:"field:category;indexed;nullable;comment:分类"`
	Description string        `srdb:"nullable;field:description;comment:商品描述"`
	CreatedAt   time.Time     `srdb:"field:created_at;comment:创建时间"`
	UpdatedAt   time.Time     `srdb:"comment:更新时间;field:updated_at;nullable"`
	ExpireIn    time.Duration `srdb:"field:expire_in;comment:过期时间;nullable"`
}

func main() {
	fmt.Println("=== 新 Tag 格式演示 ===\n")

	dataPath := "./tag_format_data"
	os.RemoveAll(dataPath)
	defer os.RemoveAll(dataPath)

	// 1. 从结构体生成 Schema
	fmt.Println("1. 从结构体生成 Schema")
	fields, err := srdb.StructToFields(Product{})
	if err != nil {
		log.Fatalf("生成字段失败: %v", err)
	}

	schema, err := srdb.NewSchema("products", fields)
	if err != nil {
		log.Fatalf("创建 Schema 失败: %v", err)
	}

	fmt.Println("   Schema 字段:")
	for _, f := range schema.Fields {
		fmt.Printf("     - %s (%s)", f.Name, f.Type)
		if f.Indexed {
			fmt.Print(" [索引]")
		}
		if f.Nullable {
			fmt.Print(" [可空]")
		}
		if f.Comment != "" {
			fmt.Printf(" // %s", f.Comment)
		}
		fmt.Println()
	}

	// 2. 创建数据库和表
	fmt.Println("\n2. 创建数据库和表")
	db, err := srdb.Open(dataPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	table, err := db.CreateTable("products", schema)
	if err != nil {
		log.Fatalf("创建表失败: %v", err)
	}
	fmt.Println("   ✓ 表创建成功")

	// 3. 检查自动创建的索引
	fmt.Println("\n3. 检查自动创建的索引")
	indexes := table.ListIndexes()
	fmt.Printf("   索引列表: %v\n", indexes)
	if len(indexes) == 3 {
		fmt.Println("   ✓ 自动为 name, stock, category 创建了索引")
	}

	// 4. 插入测试数据
	fmt.Println("\n4. 插入测试数据")
	now := time.Now()
	testData := []map[string]any{
		{
			"id":          uint32(1001),
			"name":        "苹果 iPhone 15",
			"price":       6999.0,
			"stock":       int32(50),
			"category":    "电子产品",
			"description": "最新款智能手机",
			"created_at":  now,
			"updated_at":  now,
			"expire_in":   24 * time.Hour * 365, // 1年保修
		},
		{
			"id":         uint32(1002),
			"name":       "联想笔记本",
			"price":      nil, // 价格待定（Nullable）
			"stock":      int32(0),
			"category":   "电子产品",
			"created_at": now.Add(-24 * time.Hour),
			"expire_in":  24 * time.Hour * 365 * 2, // 2年保修
		},
		{
			"id":          uint32(1003),
			"name":        "办公椅",
			"price":       899.0,
			"stock":       int32(100),
			"category":    "家具",
			"description": "人体工学设计",
			"created_at":  now.Add(-48 * time.Hour),
			"expire_in":   24 * time.Hour * 365 * 5, // 5年质保
		},
	}

	for _, data := range testData {
		err := table.Insert(data)
		if err != nil {
			log.Fatalf("插入数据失败: %v", err)
		}
	}
	fmt.Printf("   ✓ 已插入 %d 条数据\n", len(testData))

	// 5. 使用索引查询
	fmt.Println("\n5. 使用索引查询")

	fmt.Println("   a) 查询 category = '电子产品'")
	rows, err := table.Query().Eq("category", "电子产品").Rows()
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		row := rows.Row()
		data := row.Data()
		fmt.Printf("      - %s (ID: %d, 库存: %d)\n", data["name"], data["id"], data["stock"])
		count++
	}
	fmt.Printf("   ✓ 找到 %d 条记录\n", count)

	fmt.Println("\n   b) 查询 stock = 0 (缺货)")
	rows2, err := table.Query().Eq("stock", int32(0)).Rows()
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		row := rows2.Row()
		data := row.Data()
		fmt.Printf("      - %s (缺货)\n", data["name"])
	}

	// 6. 验证 Nullable 字段
	fmt.Println("\n6. 验证 Nullable 字段")
	rows3, err := table.Query().Rows()
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	defer rows3.Close()

	for rows3.Next() {
		row := rows3.Row()
		data := row.Data()
		price := data["price"]
		if price == nil {
			fmt.Printf("   - %s: 价格待定 (NULL)\n", data["name"])
		} else {
			fmt.Printf("   - %s: ¥%.2f\n", data["name"], price)
		}
	}

	// 7. 验证 Time 和 Duration 类型
	fmt.Println("\n7. 验证 Time 和 Duration 类型")
	rows4, err := table.Query().Rows()
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	defer rows4.Close()

	for rows4.Next() {
		row := rows4.Row()
		data := row.Data()
		createdAt := data["created_at"].(time.Time)
		expireIn := data["expire_in"].(time.Duration)

		fmt.Printf("   - %s:\n", data["name"])
		fmt.Printf("     创建时间: %s\n", createdAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("     质保期: %v\n", expireIn)
	}

	fmt.Println("\n=== 演示完成 ===")
	fmt.Println("\n✨ 新 Tag 格式特点:")
	fmt.Println("   • 使用 field:xxx、comment:xxx 等 key:value 格式")
	fmt.Println("   • 顺序无关，可以任意排列")
	fmt.Println("   • 支持 indexed、nullable 标记")
	fmt.Println("   • 完全向后兼容旧格式")
}
