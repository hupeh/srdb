package main

import (
	"fmt"
	"log"
	"os"

	"code.tczkiot.com/wlw/srdb"
)


func main() {
	fmt.Println("=== SRDB 完整类型系统示例 ===\n")

	// 清理旧数据
	os.RemoveAll("./data")

	// 示例 1: 展示所有类型
	fmt.Println("=== 示例 1: 展示所有 14 种支持的类型 ===")
	showAllTypes()

	// 示例 2: 实际应用场景
	fmt.Println("\n=== 示例 2: 实际应用 - 物联网传感器数据 ===")
	sensorDataExample()

	fmt.Println("\n✓ 所有示例执行成功！")
}

func showAllTypes() {
	// 展示类型映射
	types := []struct {
		name    string
		goType  string
		srdbType srdb.FieldType
	}{
		{"有符号整数", "int", srdb.Int},
		{"8位有符号整数", "int8", srdb.Int8},
		{"16位有符号整数", "int16", srdb.Int16},
		{"32位有符号整数", "int32", srdb.Int32},
		{"64位有符号整数", "int64", srdb.Int64},
		{"无符号整数", "uint", srdb.Uint},
		{"8位无符号整数", "uint8 (byte)", srdb.Uint8},
		{"16位无符号整数", "uint16", srdb.Uint16},
		{"32位无符号整数", "uint32", srdb.Uint32},
		{"64位无符号整数", "uint64", srdb.Uint64},
		{"单精度浮点", "float32", srdb.Float32},
		{"双精度浮点", "float64", srdb.Float64},
		{"字符串", "string", srdb.String},
		{"布尔", "bool", srdb.Bool},
	}

	fmt.Println("SRDB 类型系统（精确映射到 Go 基础类型）：\n")
	for i, t := range types {
		fmt.Printf("%2d. %-20s %-20s -> %s\n", i+1, t.name, t.goType, t.srdbType.String())
	}
}

func sensorDataExample() {
	// 创建 Schema
	schema, err := srdb.NewSchema("sensors", []srdb.Field{
		{Name: "device_id", Type: srdb.Uint32, Indexed: true, Comment: "设备ID"},
		{Name: "temperature", Type: srdb.Float32, Comment: "温度（摄氏度）"},
		{Name: "humidity", Type: srdb.Uint8, Comment: "湿度（0-100）"},
		{Name: "online", Type: srdb.Bool, Comment: "是否在线"},
	})
	if err != nil {
		log.Fatal(err)
	}

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/sensors",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 插入数据
	sensors := []map[string]any{
		{"device_id": uint32(1001), "temperature": float32(23.5), "humidity": uint8(65), "online": true},
		{"device_id": uint32(1002), "temperature": float32(18.2), "humidity": uint8(72), "online": true},
		{"device_id": uint32(1003), "temperature": float32(25.8), "humidity": uint8(58), "online": false},
	}

	err = table.Insert(sensors)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("✓ 插入 %d 个传感器数据\n", len(sensors))
	fmt.Println("\n类型优势演示：")
	fmt.Println("  - device_id 使用 uint32 (节省空间，支持 42 亿设备)")
	fmt.Println("  - temperature 使用 float32 (单精度足够，节省 50% 空间)")
	fmt.Println("  - humidity 使用 uint8 (0-100 范围，仅需 1 字节)")
	fmt.Println("  - online 使用 bool (语义清晰)")
}
