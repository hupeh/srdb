package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"code.tczkiot.com/wlw/srdb"
)

func main() {
	fmt.Println("=== Testing Time and Duration Types ===\n")

	// 1. 创建 Schema
	schema, err := srdb.NewSchema("events", []srdb.Field{
		{Name: "name", Type: srdb.String, Comment: "事件名称"},
		{Name: "created_at", Type: srdb.Time, Comment: "创建时间"},
		{Name: "duration", Type: srdb.Duration, Comment: "持续时间"},
		{Name: "count", Type: srdb.Int64, Comment: "计数"},
	})
	if err != nil {
		log.Fatalf("创建 Schema 失败: %v", err)
	}

	fmt.Println("✓ Schema 创建成功")
	fmt.Printf("  字段数: %d\n", len(schema.Fields))
	for _, field := range schema.Fields {
		fmt.Printf("  - %s: %s (%s)\n", field.Name, field.Type.String(), field.Comment)
	}
	fmt.Println()

	// 2. 创建数据库和表
	os.RemoveAll("./test_time_data")
	db, err := srdb.Open("./test_time_data")
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer func() {
		db.Close()
		os.RemoveAll("./test_time_data")
	}()

	table, err := db.CreateTable("events", schema)
	if err != nil {
		log.Fatalf("创建表失败: %v", err)
	}

	fmt.Println("✓ 表创建成功\n")

	// 3. 插入数据（使用原生类型）
	now := time.Now()
	duration := 2 * time.Hour

	err = table.Insert(map[string]any{
		"name":       "event1",
		"created_at": now,
		"duration":   duration,
		"count":      int64(100),
	})
	if err != nil {
		log.Fatalf("插入数据失败: %v", err)
	}

	fmt.Println("✓ 插入数据成功（使用原生类型）")
	fmt.Printf("  时间: %v\n", now)
	fmt.Printf("  持续时间: %v\n", duration)
	fmt.Println()

	// 4. 插入数据（使用字符串格式）
	err = table.Insert(map[string]any{
		"name":       "event2",
		"created_at": now.Format(time.RFC3339),
		"duration":   "1h30m",
		"count":      int64(200),
	})
	if err != nil {
		log.Fatalf("插入数据失败（字符串格式）: %v", err)
	}

	fmt.Println("✓ 插入数据成功（使用字符串格式）")
	fmt.Printf("  时间字符串: %s\n", now.Format(time.RFC3339))
	fmt.Printf("  持续时间字符串: 1h30m\n")
	fmt.Println()

	// 5. 插入数据（使用 int64 格式）
	err = table.Insert(map[string]any{
		"name":       "event3",
		"created_at": now.Unix(),
		"duration":   int64(45 * time.Minute),
		"count":      int64(300),
	})
	if err != nil {
		log.Fatalf("插入数据失败（int64 格式）: %v", err)
	}

	fmt.Println("✓ 插入数据成功（使用 int64 格式）")
	fmt.Printf("  Unix 时间戳: %d\n", now.Unix())
	fmt.Printf("  持续时间（纳秒）: %d\n", int64(45*time.Minute))
	fmt.Println()

	// 6. 查询数据
	fmt.Println("6. 查询所有数据")
	rows, err := table.Query().Rows()
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
		row := rows.Row()
		data := row.Data()

		name := data["name"]
		createdAt := data["created_at"]
		dur := data["duration"]
		cnt := data["count"]

		fmt.Printf("  [%d] 名称: %v\n", count, name)

		// 验证类型
		if t, ok := createdAt.(time.Time); ok {
			fmt.Printf("      时间: %v (类型: time.Time) ✓\n", t.Format(time.RFC3339))
		} else {
			fmt.Printf("      时间: %v (类型: %T) ✗\n", createdAt, createdAt)
		}

		if d, ok := dur.(time.Duration); ok {
			fmt.Printf("      持续时间: %v (类型: time.Duration) ✓\n", d)
		} else {
			fmt.Printf("      持续时间: %v (类型: %T) ✗\n", dur, dur)
		}

		fmt.Printf("      计数: %v\n\n", cnt)
	}

	fmt.Printf("✅ 所有测试完成! 共查询 %d 条记录\n", count)
}
