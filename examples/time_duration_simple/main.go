package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"code.tczkiot.com/wlw/srdb"
)

func main() {
	fmt.Println("=== Testing Time and Duration Types (Simple) ===\n")

	// 1. 创建 Schema
	schema, err := srdb.NewSchema("events", []srdb.Field{
		{Name: "name", Type: srdb.String, Comment: "事件名称"},
		{Name: "created_at", Type: srdb.Time, Comment: "创建时间"},
		{Name: "duration", Type: srdb.Duration, Comment: "持续时间"},
	})
	if err != nil {
		log.Fatalf("创建 Schema 失败: %v", err)
	}

	fmt.Println("✓ Schema 创建成功")
	for _, field := range schema.Fields {
		fmt.Printf("  - %s: %s\n", field.Name, field.Type.String())
	}
	fmt.Println()

	// 2. 创建数据库和表
	os.RemoveAll("./test_data")
	db, err := srdb.Open("./test_data")
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}

	table, err := db.CreateTable("events", schema)
	if err != nil {
		log.Fatalf("创建表失败: %v", err)
	}

	fmt.Println("✓ 表创建成功\n")

	// 3. 插入数据
	now := time.Now()
	duration := 2 * time.Hour

	err = table.Insert(map[string]any{
		"name":       "event1",
		"created_at": now,
		"duration":   duration,
	})
	if err != nil {
		log.Fatalf("插入数据失败: %v", err)
	}

	fmt.Println("✓ 插入数据成功")
	fmt.Printf("  时间: %v\n", now.Format(time.RFC3339))
	fmt.Printf("  持续时间: %v\n\n", duration)

	// 4. 立即查询（从 MemTable）
	fmt.Println("4. 查询数据（从 MemTable）")
	rows, err := table.Query().Rows()
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	defer rows.Close()

	success := true
	for rows.Next() {
		row := rows.Row()
		data := row.Data()

		name := data["name"]
		createdAt := data["created_at"]
		dur := data["duration"]

		fmt.Printf("  名称: %v\n", name)

		// 验证类型
		if t, ok := createdAt.(time.Time); ok {
			fmt.Printf("  时间: %v (类型: time.Time) ✓\n", t.Format(time.RFC3339))
		} else {
			fmt.Printf("  时间: %v (类型: %T) ✗ FAILED\n", createdAt, createdAt)
			success = false
		}

		if d, ok := dur.(time.Duration); ok {
			fmt.Printf("  持续时间: %v (类型: time.Duration) ✓\n", d)
		} else {
			fmt.Printf("  持续时间: %v (类型: %T) ✗ FAILED\n", dur, dur)
			success = false
		}
	}

	if success {
		fmt.Println("\n✅ 测试通过! Time 和 Duration 类型正确保留")
	} else {
		fmt.Println("\n❌ 测试失败! 类型未正确保留")
		os.Exit(1)
	}

	// 快速退出，不等待清理
	os.Exit(0)
}
