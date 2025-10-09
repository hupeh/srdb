package main

import (
	"fmt"
	"log"
	"os"

	"code.tczkiot.com/wlw/srdb"
	"github.com/shopspring/decimal"
)

func main() {
	fmt.Println("=== SRDB 新类型系统示例 ===")

	// 清理旧数据
	os.RemoveAll("./data")

	// 示例 1: Byte 类型 - 适用于状态码、标志位等小整数
	fmt.Println("\n=== 示例 1: Byte 类型（状态码）===")
	byteExample()

	// 示例 2: Rune 类型 - 适用于单个字符、等级标识等
	fmt.Println("\n=== 示例 2: Rune 类型（等级字符）===")
	runeExample()

	// 示例 3: Decimal 类型 - 适用于金融计算、精确数值
	fmt.Println("\n=== 示例 3: Decimal 类型（金融数据）===")
	decimalExample()

	// 示例 4: Nullable 支持 - 允许字段为 NULL
	fmt.Println("\n=== 示例 4: Nullable 支持 ===")
	nullableExample()

	// 示例 5: 完整类型系统 - 展示所有 17 种类型
	fmt.Println("\n=== 示例 5: 完整类型系统（17 种类型）===")
	allTypesExample()

	fmt.Println("\n✓ 所有示例执行成功！")
}

// byteExample 演示 Byte 类型的使用
func byteExample() {
	// 创建 Schema - 使用 byte 类型存储状态码
	schema, err := srdb.NewSchema("api_logs", []srdb.Field{
		{Name: "endpoint", Type: srdb.String, Comment: "API 端点"},
		{Name: "status_code", Type: srdb.Byte, Comment: "HTTP 状态码（用 byte 节省空间）"},
		{Name: "response_time_ms", Type: srdb.Uint16, Comment: "响应时间（毫秒）"},
	})
	if err != nil {
		log.Fatal(err)
	}

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/api_logs",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 插入数据 - status_code 使用 byte 类型（0-255）
	logs := []map[string]any{
		{"endpoint": "/api/users", "status_code": uint8(200), "response_time_ms": uint16(45)},
		{"endpoint": "/api/orders", "status_code": uint8(201), "response_time_ms": uint16(89)},
		{"endpoint": "/api/products", "status_code": uint8(255), "response_time_ms": uint16(12)},
		{"endpoint": "/api/auth", "status_code": uint8(128), "response_time_ms": uint16(234)},
	}

	err = table.Insert(logs)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("✓ 插入 %d 条 API 日志\n", len(logs))
	fmt.Println("类型优势：")
	fmt.Println("  - status_code 使用 byte (仅 1 字节，相比 int64 节省 87.5% 空间)")
	fmt.Println("  - response_time_ms 使用 uint16 (0-65535ms 范围足够)")

	// 查询数据
	row, _ := table.Get(1)
	fmt.Printf("\n查询结果: endpoint=%s, status_code=%d, response_time=%dms\n",
		row.Data["endpoint"], row.Data["status_code"], row.Data["response_time_ms"])
}

// runeExample 演示 Rune 类型的使用
func runeExample() {
	// 创建 Schema - 使用 rune 类型存储等级字符
	schema, err := srdb.NewSchema("user_levels", []srdb.Field{
		{Name: "username", Type: srdb.String, Indexed: true, Comment: "用户名"},
		{Name: "level", Type: srdb.Rune, Comment: "等级字符 (S/A/B/C/D)"},
		{Name: "score", Type: srdb.Uint32, Comment: "积分"},
	})
	if err != nil {
		log.Fatal(err)
	}

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/user_levels",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 插入数据 - level 使用 rune 类型存储单个字符
	users := []map[string]any{
		{"username": "Alice", "level": rune('S'), "score": uint32(9500)},
		{"username": "Bob", "level": rune('A'), "score": uint32(7200)},
		{"username": "Charlie", "level": rune('B'), "score": uint32(5800)},
		{"username": "David", "level": rune('C'), "score": uint32(3400)},
	}

	err = table.Insert(users)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("✓ 插入 %d 个用户等级数据\n", len(users))
	fmt.Println("类型优势：")
	fmt.Println("  - level 使用 rune 存储单个字符（语义清晰）")
	fmt.Println("  - 支持 Unicode 字符，如中文等级：'甲'、'乙'、'丙'")

	// 查询数据
	row, _ := table.Get(1)
	levelRune := row.Data["level"].(rune)
	fmt.Printf("\n查询结果: username=%s, level=%c, score=%d\n",
		row.Data["username"], levelRune, row.Data["score"])
}

// decimalExample 演示 Decimal 类型的使用
func decimalExample() {
	// 创建 Schema - 使用 decimal 类型存储金融数据
	schema, err := srdb.NewSchema("transactions", []srdb.Field{
		{Name: "tx_id", Type: srdb.String, Indexed: true, Comment: "交易ID"},
		{Name: "amount", Type: srdb.Decimal, Comment: "交易金额（高精度）"},
		{Name: "fee", Type: srdb.Decimal, Comment: "手续费（高精度）"},
		{Name: "currency", Type: srdb.String, Comment: "货币类型"},
	})
	if err != nil {
		log.Fatal(err)
	}

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/transactions",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 插入数据 - amount 和 fee 使用 decimal 类型（无精度损失）
	transactions := []map[string]any{
		{
			"tx_id":    "TX001",
			"amount":   decimal.NewFromFloat(1234.56789012345), // 高精度
			"fee":      decimal.NewFromFloat(1.23),
			"currency": "USD",
		},
		{
			"tx_id":    "TX002",
			"amount":   decimal.RequireFromString("9876.543210987654321"), // 字符串创建，更精确
			"fee":      decimal.NewFromFloat(9.88),
			"currency": "EUR",
		},
		{
			"tx_id":    "TX003",
			"amount":   decimal.NewFromFloat(0.00000001), // 极小值
			"fee":      decimal.NewFromFloat(0.0000001),
			"currency": "BTC",
		},
	}

	err = table.Insert(transactions)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("✓ 插入 %d 笔交易\n", len(transactions))
	fmt.Println("类型优势：")
	fmt.Println("  - decimal 类型无精度损失（使用 shopspring/decimal）")
	fmt.Println("  - 适合金融计算、科学计算等需要精确数值的场景")
	fmt.Println("  - 避免浮点数运算误差（如 0.1 + 0.2 ≠ 0.3）")

	// 查询数据并进行计算
	row, _ := table.Get(1)
	amount := row.Data["amount"].(decimal.Decimal)
	fee := row.Data["fee"].(decimal.Decimal)
	total := amount.Add(fee) // decimal 类型的精确加法

	fmt.Printf("\n查询结果: tx_id=%s, currency=%s\n", row.Data["tx_id"], row.Data["currency"])
	fmt.Printf("  金额: %s\n", amount.String())
	fmt.Printf("  手续费: %s\n", fee.String())
	fmt.Printf("  总计: %s (精确计算，无误差)\n", total.String())
}

// nullableExample 演示 Nullable 支持
func nullableExample() {
	// 创建 Schema - 某些字段允许为 NULL
	schema, err := srdb.NewSchema("user_profiles", []srdb.Field{
		{Name: "username", Type: srdb.String, Nullable: false, Comment: "用户名（必填）"},
		{Name: "email", Type: srdb.String, Nullable: true, Comment: "邮箱（可选）"},
		{Name: "age", Type: srdb.Uint8, Nullable: true, Comment: "年龄（可选）"},
		{Name: "bio", Type: srdb.String, Nullable: true, Comment: "个人简介（可选）"},
	})
	if err != nil {
		log.Fatal(err)
	}

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/user_profiles",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 插入数据 - 可选字段可以为 nil
	profiles := []map[string]any{
		{
			"username": "Alice",
			"email":    "alice@example.com",
			"age":      uint8(25),
			"bio":      "Hello, I'm Alice!",
		},
		{
			"username": "Bob",
			"email":    nil, // email 为 NULL
			"age":      uint8(30),
			"bio":      "Software Engineer",
		},
		{
			"username": "Charlie",
			"email":    "charlie@example.com",
			"age":      nil, // age 为 NULL
			"bio":      nil, // bio 为 NULL
		},
	}

	err = table.Insert(profiles)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("✓ 插入 %d 个用户资料（包含 NULL 值）\n", len(profiles))
	fmt.Println("类型优势：")
	fmt.Println("  - Nullable 字段可以为 NULL，区分'未填写'和'空字符串'")
	fmt.Println("  - 非 Nullable 字段必须有值，保证数据完整性")

	// 查询数据
	for i := 1; i <= 3; i++ {
		row, _ := table.Get(int64(i))
		fmt.Printf("\n用户 %d: username=%s", i, row.Data["username"])
		if email, ok := row.Data["email"]; ok && email != nil {
			fmt.Printf(", email=%s", email)
		} else {
			fmt.Print(", email=NULL")
		}
		if age, ok := row.Data["age"]; ok && age != nil {
			fmt.Printf(", age=%d", age)
		} else {
			fmt.Print(", age=NULL")
		}
	}
	fmt.Println()
}

// allTypesExample 展示所有 17 种类型
func allTypesExample() {
	schema, err := srdb.NewSchema("all_types_demo", []srdb.Field{
		// 有符号整数类型 (5 种)
		{Name: "f_int", Type: srdb.Int, Comment: "int"},
		{Name: "f_int8", Type: srdb.Int8, Comment: "int8"},
		{Name: "f_int16", Type: srdb.Int16, Comment: "int16"},
		{Name: "f_int32", Type: srdb.Int32, Comment: "int32"},
		{Name: "f_int64", Type: srdb.Int64, Comment: "int64"},

		// 无符号整数类型 (5 种)
		{Name: "f_uint", Type: srdb.Uint, Comment: "uint"},
		{Name: "f_uint8", Type: srdb.Uint8, Comment: "uint8"},
		{Name: "f_uint16", Type: srdb.Uint16, Comment: "uint16"},
		{Name: "f_uint32", Type: srdb.Uint32, Comment: "uint32"},
		{Name: "f_uint64", Type: srdb.Uint64, Comment: "uint64"},

		// 浮点类型 (2 种)
		{Name: "f_float32", Type: srdb.Float32, Comment: "float32"},
		{Name: "f_float64", Type: srdb.Float64, Comment: "float64"},

		// 字符串类型 (1 种)
		{Name: "f_string", Type: srdb.String, Comment: "string"},

		// 布尔类型 (1 种)
		{Name: "f_bool", Type: srdb.Bool, Comment: "bool"},

		// 特殊类型 (3 种)
		{Name: "f_byte", Type: srdb.Byte, Comment: "byte (=uint8)"},
		{Name: "f_rune", Type: srdb.Rune, Comment: "rune (=int32)"},
		{Name: "f_decimal", Type: srdb.Decimal, Comment: "decimal (高精度)"},
	})
	if err != nil {
		log.Fatal(err)
	}

	table, err := srdb.OpenTable(&srdb.TableOptions{
		Dir:    "./data/all_types",
		Name:   schema.Name,
		Fields: schema.Fields,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer table.Close()

	// 插入包含所有类型的数据
	record := map[string]any{
		// 有符号整数
		"f_int":   int(-12345),
		"f_int8":  int8(-128),
		"f_int16": int16(-32768),
		"f_int32": int32(-2147483648),
		"f_int64": int64(-9223372036854775808),

		// 无符号整数
		"f_uint":   uint(12345),
		"f_uint8":  uint8(255),
		"f_uint16": uint16(65535),
		"f_uint32": uint32(4294967295),
		"f_uint64": uint64(18446744073709551615),

		// 浮点
		"f_float32": float32(3.14159),
		"f_float64": float64(2.718281828459045),

		// 字符串
		"f_string": "Hello, SRDB! 你好！",

		// 布尔
		"f_bool": true,

		// 特殊类型
		"f_byte":    byte(255),
		"f_rune":    rune('中'),
		"f_decimal": decimal.NewFromFloat(123456.789012345),
	}

	err = table.Insert(record)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("✓ 插入包含所有 17 种类型的数据")
	fmt.Println("\nSRDB 完整类型系统：")
	fmt.Println("  有符号整数: int, int8, int16, int32, int64 (5 种)")
	fmt.Println("  无符号整数: uint, uint8, uint16, uint32, uint64 (5 种)")
	fmt.Println("  浮点类型:   float32, float64 (2 种)")
	fmt.Println("  字符串类型: string (1 种)")
	fmt.Println("  布尔类型:   bool (1 种)")
	fmt.Println("  特殊类型:   byte, rune, decimal (3 种)")
	fmt.Println("  总计:       17 种类型")

	// 查询并验证数据
	row, _ := table.Get(1)
	fmt.Println("\n数据验证：")
	fmt.Printf("  f_int=%d, f_int64=%d\n", row.Data["f_int"], row.Data["f_int64"])
	fmt.Printf("  f_uint=%d, f_uint64=%d\n", row.Data["f_uint"], row.Data["f_uint64"])
	fmt.Printf("  f_float32=%f, f_float64=%f\n", row.Data["f_float32"], row.Data["f_float64"])
	fmt.Printf("  f_string=%s\n", row.Data["f_string"])
	fmt.Printf("  f_bool=%v\n", row.Data["f_bool"])
	fmt.Printf("  f_byte=%d, f_rune=%c\n", row.Data["f_byte"], row.Data["f_rune"])
	fmt.Printf("  f_decimal=%s\n", row.Data["f_decimal"].(decimal.Decimal).String())
}
