package commands

import (
	"fmt"
	"log"

	"code.tczkiot.com/wlw/srdb"
)

// GenerateTables 生成指定数量的测试表
func GenerateTables(dbPath string, count int) {
	// 打开数据库
	db, err := srdb.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Printf("Generating %d tables...\n", count)

	successCount := 0
	skipCount := 0

	for i := 1; i <= count; i++ {
		tableName := fmt.Sprintf("test_table_%03d", i)

		// 检查表是否已存在
		tables := db.ListTables()
		exists := false
		for _, t := range tables {
			if t == tableName {
				exists = true
				break
			}
		}

		if exists {
			fmt.Printf("[%d/%d] Table '%s' already exists, skipping...\n", i, count, tableName)
			skipCount++
			continue
		}

		// 创建多样化的 Schema
		var schema *srdb.Schema
		var schemaErr error

		// 根据不同的表使用不同的字段配置
		switch i % 5 {
		case 0:
			// 传感器数据表
			schema, schemaErr = srdb.NewSchema(tableName, []srdb.Field{
				{Name: "device_id", Type: srdb.Uint32, Indexed: true, Comment: "设备ID"},
				{Name: "temperature", Type: srdb.Float32, Indexed: false, Comment: "温度"},
				{Name: "humidity", Type: srdb.Uint8, Indexed: false, Comment: "湿度"},
				{Name: "status", Type: srdb.Bool, Indexed: false, Comment: "状态"},
			})
		case 1:
			// 用户活动表
			schema, schemaErr = srdb.NewSchema(tableName, []srdb.Field{
				{Name: "user_id", Type: srdb.Int64, Indexed: true, Comment: "用户ID"},
				{Name: "action", Type: srdb.String, Indexed: true, Comment: "操作类型"},
				{Name: "duration", Type: srdb.Int32, Indexed: false, Comment: "持续时间(秒)"},
				{Name: "score", Type: srdb.Float64, Indexed: false, Comment: "评分"},
			})
		case 2:
			// 商品库存表
			schema, schemaErr = srdb.NewSchema(tableName, []srdb.Field{
				{Name: "product_id", Type: srdb.String, Indexed: true, Comment: "商品ID"},
				{Name: "name", Type: srdb.String, Indexed: false, Comment: "商品名称"},
				{Name: "quantity", Type: srdb.Uint32, Indexed: false, Comment: "库存数量"},
				{Name: "price", Type: srdb.Float32, Indexed: false, Comment: "价格"},
			})
		case 3:
			// 日志记录表
			schema, schemaErr = srdb.NewSchema(tableName, []srdb.Field{
				{Name: "level", Type: srdb.String, Indexed: true, Comment: "日志级别"},
				{Name: "message", Type: srdb.String, Indexed: false, Comment: "日志消息"},
				{Name: "code", Type: srdb.Int32, Indexed: false, Comment: "错误码"},
				{Name: "timestamp", Type: srdb.String, Indexed: false, Comment: "时间戳"},
			})
		case 4:
			// 订单表
			schema, schemaErr = srdb.NewSchema(tableName, []srdb.Field{
				{Name: "order_id", Type: srdb.String, Indexed: true, Comment: "订单ID"},
				{Name: "customer", Type: srdb.String, Indexed: true, Comment: "客户名称"},
				{Name: "amount", Type: srdb.Float64, Indexed: false, Comment: "订单金额"},
				{Name: "status", Type: srdb.String, Indexed: false, Comment: "订单状态"},
			})
		}

		if schemaErr != nil {
			log.Printf("[%d/%d] Failed to create schema for table '%s': %v\n", i, count, tableName, schemaErr)
			continue
		}

		// 创建表
		table, err := db.CreateTable(tableName, schema)
		if err != nil {
			log.Printf("[%d/%d] Failed to create table '%s': %v\n", i, count, tableName, err)
			continue
		}

		// 插入一些示例数据
		sampleCount := 3
		insertedCount := 0

		for j := 0; j < sampleCount; j++ {
			var record map[string]any

			switch i % 5 {
			case 0:
				record = map[string]any{
					"device_id":   uint32(1000 + j),
					"temperature": float32(20.5 + float64(j)*2.5),
					"humidity":    uint8(50 + j*10),
					"status":      j%2 == 0,
				}
			case 1:
				actions := []string{"login", "view", "purchase"}
				record = map[string]any{
					"user_id":  int64(100 + j),
					"action":   actions[j%len(actions)],
					"duration": int32(30 + j*15),
					"score":    float64(4.5 + float64(j)*0.2),
				}
			case 2:
				products := []string{"Laptop", "Mouse", "Keyboard"}
				record = map[string]any{
					"product_id": fmt.Sprintf("P%04d", 1000+j),
					"name":       products[j%len(products)],
					"quantity":   uint32(10 + j*5),
					"price":      float32(99.99 + float64(j)*50.0),
				}
			case 3:
				levels := []string{"INFO", "WARN", "ERROR"}
				record = map[string]any{
					"level":     levels[j%len(levels)],
					"message":   fmt.Sprintf("Test message #%d", j+1),
					"code":      int32(200 + j*100),
					"timestamp": fmt.Sprintf("2024-01-%02d 10:00:00", j+1),
				}
			case 4:
				statuses := []string{"pending", "processing", "completed"}
				record = map[string]any{
					"order_id": fmt.Sprintf("ORD%06d", 10000+j),
					"customer": fmt.Sprintf("Customer %d", j+1),
					"amount":   float64(100.0 + float64(j)*50.0),
					"status":   statuses[j%len(statuses)],
				}
			}

			if err := table.Insert(record); err != nil {
				log.Printf("Failed to insert sample data: %v", err)
			} else {
				insertedCount++
			}
		}

		successCount++
		fmt.Printf("[%d/%d] Created table '%s' with %d/%d sample records\n", i, count, tableName, insertedCount, sampleCount)
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Total: %d\n", count)
	fmt.Printf("  Created: %d\n", successCount)
	fmt.Printf("  Skipped (already exists): %d\n", skipCount)
	fmt.Printf("  Failed: %d\n", count-successCount-skipCount)
}
