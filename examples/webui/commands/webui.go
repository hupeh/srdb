package commands

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"slices"
	"time"

	"code.tczkiot.com/wlw/srdb"
	"code.tczkiot.com/wlw/srdb/webui"
)

// StartWebUI 启动 WebUI 服务器
func StartWebUI(dbPath string, addr string) {
	// 打开数据库
	db, err := srdb.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 创建示例 Schema
	userSchema, err := srdb.NewSchema("users", []srdb.Field{
		{Name: "name", Type: srdb.String, Indexed: true, Comment: "User name"},
		{Name: "email", Type: srdb.String, Indexed: false, Comment: "Email address"},
		{Name: "age", Type: srdb.Int64, Indexed: false, Comment: "Age"},
		{Name: "city", Type: srdb.String, Indexed: false, Comment: "City"},
	})
	if err != nil {
		log.Fatal(err)
	}

	productSchema, err := srdb.NewSchema("products", []srdb.Field{
		{Name: "product_name", Type: srdb.String, Indexed: true, Comment: "Product name"},
		{Name: "price", Type: srdb.Float64, Indexed: false, Comment: "Price"},
		{Name: "quantity", Type: srdb.Int64, Indexed: false, Comment: "Quantity"},
		{Name: "category", Type: srdb.String, Indexed: false, Comment: "Category"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// 创建表（如果不存在）
	tables := db.ListTables()
	hasUsers := false
	hasProducts := false
	for _, t := range tables {
		if t == "users" {
			hasUsers = true
		}
		if t == "products" {
			hasProducts = true
		}
	}

	if !hasUsers {
		table, err := db.CreateTable("users", userSchema)
		if err != nil {
			log.Printf("Create users table failed: %v", err)
		} else {
			// 插入一些示例数据
			users := []map[string]any{
				{"name": "Alice", "email": "alice@example.com", "age": int64(30), "city": "Beijing"},
				{"name": "Bob", "email": "bob@example.com", "age": int64(25), "city": "Shanghai"},
				{"name": "Charlie", "email": "charlie@example.com", "age": int64(35), "city": "Guangzhou"},
				{"name": "David", "email": "david@example.com", "age": int64(28), "city": "Shenzhen"},
				{"name": "Eve", "email": "eve@example.com", "age": int64(32), "city": "Hangzhou"},
			}
			insertedCount := 0
			for _, user := range users {
				if err := table.Insert(user); err != nil {
					log.Printf("Failed to insert user: %v, error: %v", user, err)
				} else {
					insertedCount++
				}
			}
			log.Printf("Created users table with %d/%d records", insertedCount, len(users))
		}
	}

	if !hasProducts {
		table, err := db.CreateTable("products", productSchema)
		if err != nil {
			log.Printf("Create products table failed: %v", err)
		} else {
			// 插入一些示例数据
			products := []map[string]any{
				{"product_name": "Laptop", "price": 999.99, "quantity": int64(10), "category": "Electronics"},
				{"product_name": "Mouse", "price": 29.99, "quantity": int64(50), "category": "Electronics"},
				{"product_name": "Keyboard", "price": 79.99, "quantity": int64(30), "category": "Electronics"},
				{"product_name": "Monitor", "price": 299.99, "quantity": int64(15), "category": "Electronics"},
				{"product_name": "Desk", "price": 199.99, "quantity": int64(5), "category": "Furniture"},
				{"product_name": "Chair", "price": 149.99, "quantity": int64(8), "category": "Furniture"},
			}
			insertedCount := 0
			for _, product := range products {
				if err := table.Insert(product); err != nil {
					log.Printf("Failed to insert product: %v, error: %v", product, err)
				} else {
					insertedCount++
				}
			}
			log.Printf("Created products table with %d/%d records", insertedCount, len(products))
		}
	}

	// 启动后台数据插入协程
	go autoInsertData(db)

	// 启动 Web UI
	handler := webui.NewWebUI(db)

	fmt.Printf("SRDB Web UI is running at http://%s\n", addr)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println("Background data insertion is running...")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

// generateRandomData 生成指定大小的随机数据 (2KB ~ 512KB)
func generateRandomData() string {
	minSize := 2 * 1024              // 2KB
	maxSize := (1 * 1024 * 1024) / 2 // 512KB

	sizeBig, _ := rand.Int(rand.Reader, big.NewInt(int64(maxSize-minSize)))
	size := int(sizeBig.Int64()) + minSize

	data := make([]byte, size)
	rand.Read(data)

	return fmt.Sprintf("%x", data)
}

// autoInsertData 在后台自动插入数据
func autoInsertData(db *srdb.Database) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	counter := 1
	var logsTable *srdb.Table

	for range ticker.C {
		tables := db.ListTables()
		hasLogs := slices.Contains(tables, "logs")

		if !hasLogs {
			logsSchema, err := srdb.NewSchema("logs", []srdb.Field{
				{Name: "group", Type: srdb.String, Indexed: true, Comment: "Log group (A-E)"},
				{Name: "timestamp", Type: srdb.String, Indexed: false, Comment: "Timestamp"},
				{Name: "data", Type: srdb.String, Indexed: false, Comment: "Random data"},
				{Name: "size_bytes", Type: srdb.Int64, Indexed: false, Comment: "Data size in bytes"},
			})
			if err != nil {
				log.Fatal(err)
			}

			logsTable, err = db.CreateTable("logs", logsSchema)
			if err != nil {
				log.Printf("Failed to create logs table: %v", err)
				continue
			}
			log.Println("Created logs table for background data insertion")
		} else {
			var err error
			logsTable, err = db.GetTable("logs")
			if err != nil || logsTable == nil {
				continue
			}
		}

		data := generateRandomData()
		sizeBytes := len(data)

		// 随机选择一个组 (A-E)
		groups := []string{"A", "B", "C", "D", "E"}
		group := groups[counter%len(groups)]

		record := map[string]any{
			"group":      group,
			"timestamp":  time.Now().Format(time.RFC3339),
			"data":       data,
			"size_bytes": int64(sizeBytes),
		}

		err := logsTable.Insert(record)
		if err != nil {
			log.Printf("Failed to insert data: %v", err)
		} else {
			sizeStr := formatBytes(sizeBytes)
			log.Printf("Inserted record #%d, group: %s, size: %s", counter, group, sizeStr)
			counter++
		}
	}
}

// formatBytes 格式化字节大小显示
func formatBytes(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), units[exp])
}
