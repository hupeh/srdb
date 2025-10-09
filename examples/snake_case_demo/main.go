package main

import (
	"fmt"
	"log"

	"code.tczkiot.com/wlw/srdb"
)

// 演示各种驼峰命名自动转换为 snake_case
type DemoStruct struct {
	// 基本转换
	UserName      string `srdb:";comment:用户名"`      // -> user_name
	EmailAddress  string `srdb:";comment:邮箱地址"`      // -> email_address
	PhoneNumber   string `srdb:";comment:手机号"`       // -> phone_number

	// 连续大写字母
	HTTPEndpoint  string `srdb:";comment:HTTP 端点"`   // -> http_endpoint
	URLPath       string `srdb:";comment:URL 路径"`    // -> url_path
	XMLParser     string `srdb:";comment:XML 解析器"`   // -> xml_parser

	// 短命名
	ID            int64  `srdb:";comment:ID"`         // -> id

	// 布尔值
	IsActive      bool   `srdb:";comment:是否激活"`      // -> is_active
	IsDeleted     bool   `srdb:";comment:是否删除"`      // -> is_deleted

	// 数字混合
	Address1      string `srdb:";comment:地址1"`       // -> address1
	User2Name     string `srdb:";comment:用户2名称"`     // -> user2_name
}

func main() {
	fmt.Println("=== snake_case 自动转换演示 ===")

	// 生成 Field 列表
	fields, err := srdb.StructToFields(DemoStruct{})
	if err != nil {
		log.Fatal(err)
	}

	// 打印转换结果
	fmt.Println("\n字段名转换（驼峰命名 -> snake_case）：")
	fmt.Printf("%-20s -> %-20s %s\n", "Go 字段名", "数据库字段名", "注释")
	fmt.Println(string(make([]byte, 70)) + "\n" + string(make([]byte, 70)))

	type fieldInfo struct {
		goName string
		dbName string
		comment string
	}

	fieldMapping := []fieldInfo{
		{"UserName", "user_name", "用户名"},
		{"EmailAddress", "email_address", "邮箱地址"},
		{"PhoneNumber", "phone_number", "手机号"},
		{"HTTPEndpoint", "http_endpoint", "HTTP 端点"},
		{"URLPath", "url_path", "URL 路径"},
		{"XMLParser", "xml_parser", "XML 解析器"},
		{"ID", "id", "ID"},
		{"IsActive", "is_active", "是否激活"},
		{"IsDeleted", "is_deleted", "是否删除"},
		{"Address1", "address1", "地址1"},
		{"User2Name", "user2_name", "用户2名称"},
	}

	for i, field := range fields {
		if i < len(fieldMapping) {
			fmt.Printf("%-20s -> %-20s %s\n",
				fieldMapping[i].goName,
				field.Name,
				field.Comment)
		}
	}

	// 验证转换
	fmt.Println("\n=== 转换验证 ===")
	allCorrect := true
	for i, field := range fields {
		if i < len(fieldMapping) {
			expected := fieldMapping[i].dbName
			if field.Name != expected {
				fmt.Printf("❌ %s: 期望 %s, 实际 %s\n",
					fieldMapping[i].goName, expected, field.Name)
				allCorrect = false
			}
		}
	}

	if allCorrect {
		fmt.Println("✅ 所有字段名转换正确！")
	}

	// 创建 Schema
	schema, err := srdb.NewSchema("demo", fields)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n✅ 成功创建 Schema，包含 %d 个字段\n", len(schema.Fields))
}
