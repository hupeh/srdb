package srdb

import (
	"strings"
	"testing"
)

// Package-level test schemas
var (
	UserSchema  *Schema
	LogSchema   *Schema
	OrderSchema *Schema
)

func init() {
	var err error

	// UserSchema 用户表 Schema
	UserSchema, err = NewSchema("users", []Field{
		{Name: "name", Type: String, Indexed: true, Comment: "用户名"},
		{Name: "age", Type: Int64, Indexed: true, Comment: "年龄"},
		{Name: "email", Type: String, Indexed: true, Comment: "邮箱"},
		{Name: "description", Type: String, Indexed: false, Comment: "描述"},
	})
	if err != nil {
		panic("Failed to create UserSchema: " + err.Error())
	}

	// LogSchema 日志表 Schema
	LogSchema, err = NewSchema("logs", []Field{
		{Name: "level", Type: String, Indexed: true, Comment: "日志级别"},
		{Name: "message", Type: String, Indexed: false, Comment: "日志消息"},
		{Name: "source", Type: String, Indexed: true, Comment: "来源"},
		{Name: "error_code", Type: Int64, Indexed: true, Comment: "错误码"},
	})
	if err != nil {
		panic("Failed to create LogSchema: " + err.Error())
	}

	// OrderSchema 订单表 Schema
	OrderSchema, err = NewSchema("orders", []Field{
		{Name: "order_id", Type: String, Indexed: true, Comment: "订单ID"},
		{Name: "user_id", Type: Int64, Indexed: true, Comment: "用户ID"},
		{Name: "amount", Type: Float64, Indexed: true, Comment: "金额"},
		{Name: "status", Type: String, Indexed: true, Comment: "状态"},
		{Name: "paid", Type: Bool, Indexed: true, Comment: "是否支付"},
	})
	if err != nil {
		panic("Failed to create OrderSchema: " + err.Error())
	}
}

func TestSchema(t *testing.T) {
	// 创建 Schema
	schema, err := NewSchema("test", []Field{
		{Name: "name", Type: String, Indexed: true, Comment: "名称"},
		{Name: "age", Type: Int64, Indexed: true, Comment: "年龄"},
		{Name: "score", Type: Float64, Indexed: false, Comment: "分数"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 测试数据
	data := map[string]any{
		"name":  "Alice",
		"age":   25,
		"score": 95.5,
	}

	// 验证
	err = schema.Validate(data)
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	// 获取索引字段
	indexedFields := schema.GetIndexedFields()
	if len(indexedFields) != 2 {
		t.Errorf("Expected 2 indexed fields, got %d", len(indexedFields))
	}

	t.Log("Schema test passed!")
}

func TestSchemaValidation(t *testing.T) {
	schema, err := NewSchema("test", []Field{
		{Name: "name", Type: String, Indexed: true, Comment: "名称"},
		{Name: "age", Type: Int64, Indexed: true, Comment: "年龄"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 正确的数据
	validData := map[string]any{
		"name": "Bob",
		"age":  30,
	}

	err = schema.Validate(validData)
	if err != nil {
		t.Errorf("Valid data failed validation: %v", err)
	}

	// 错误的数据类型
	invalidData := map[string]any{
		"name": "Charlie",
		"age":  "thirty", // 应该是 int64
	}

	err = schema.Validate(invalidData)
	if err == nil {
		t.Error("Invalid data should fail validation")
	}

	t.Log("Schema validation test passed!")
}

// TestNewSchemaValidation 测试 NewSchema 的各种验证场景
func TestNewSchemaValidation(t *testing.T) {
	tests := []struct {
		name        string
		schemaName  string
		fields      []Field
		shouldError bool
		errorMsg    string
	}{
		{
			name:       "Valid schema",
			schemaName: "users",
			fields: []Field{
				{Name: "id", Type: Int64},
				{Name: "name", Type: String},
			},
			shouldError: false,
		},
		{
			name:        "Empty schema name",
			schemaName:  "",
			fields:      []Field{{Name: "id", Type: Int64}},
			shouldError: true,
			errorMsg:    "schema name cannot be empty",
		},
		{
			name:        "Empty fields array",
			schemaName:  "users",
			fields:      []Field{},
			shouldError: true,
			errorMsg:    "schema must have at least one field",
		},
		{
			name:        "Nil fields array",
			schemaName:  "users",
			fields:      nil,
			shouldError: true,
			errorMsg:    "schema must have at least one field",
		},
		{
			name:       "Empty field name at index 0",
			schemaName: "users",
			fields: []Field{
				{Name: "", Type: Int64},
			},
			shouldError: true,
			errorMsg:    "field at index 0 has empty name",
		},
		{
			name:       "Empty field name at index 1",
			schemaName: "users",
			fields: []Field{
				{Name: "id", Type: Int64},
				{Name: "", Type: String},
			},
			shouldError: true,
			errorMsg:    "field at index 1 has empty name",
		},
		{
			name:       "Duplicate field name",
			schemaName: "users",
			fields: []Field{
				{Name: "id", Type: Int64},
				{Name: "name", Type: String},
				{Name: "id", Type: String}, // Duplicate
			},
			shouldError: true,
			errorMsg:    "duplicate field name: id",
		},
		{
			name:       "Valid schema with single field",
			schemaName: "logs",
			fields: []Field{
				{Name: "message", Type: String},
			},
			shouldError: false,
		},
		{
			name:       "Valid schema with indexed field",
			schemaName: "users",
			fields: []Field{
				{Name: "id", Type: Int64, Indexed: true},
				{Name: "email", Type: String, Indexed: true},
				{Name: "age", Type: Int64},
			},
			shouldError: false,
		},
		{
			name:       "Valid schema with comments",
			schemaName: "products",
			fields: []Field{
				{Name: "id", Type: Int64, Comment: "产品ID"},
				{Name: "name", Type: String, Comment: "产品名称"},
				{Name: "price", Type: Float64, Comment: "价格"},
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := NewSchema(tt.schemaName, tt.fields)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}

				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}

				// 验证错误码是 ErrCodeSchemaInvalid
				if GetErrorCode(err) != ErrCodeSchemaInvalid {
					t.Errorf("Expected error code %d, got %d", ErrCodeSchemaInvalid, GetErrorCode(err))
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
					return
				}

				// 验证返回的 schema 是正确的
				if schema == nil {
					t.Errorf("Expected schema, got nil")
					return
				}
				if schema.Name != tt.schemaName {
					t.Errorf("Expected schema name %q, got %q", tt.schemaName, schema.Name)
				}
				if len(schema.Fields) != len(tt.fields) {
					t.Errorf("Expected %d fields, got %d", len(tt.fields), len(schema.Fields))
				}
			}
		})
	}
}

// TestNewSchemaFieldValidation 测试字段级别的验证
func TestNewSchemaFieldValidation(t *testing.T) {
	t.Run("Multiple duplicate field names", func(t *testing.T) {
		schema, err := NewSchema("test", []Field{
			{Name: "id", Type: Int64},
			{Name: "name", Type: String},
			{Name: "id", Type: String},   // First duplicate
			{Name: "name", Type: String}, // Second duplicate
		})

		if err == nil {
			t.Errorf("Expected error for duplicate field names")
			return
		}

		// 应该在第一个重复处就停止
		if !strings.Contains(err.Error(), "duplicate field name: id") {
			t.Errorf("Expected error about duplicate field 'id', got: %v", err)
		}

		if schema != nil {
			t.Errorf("Expected nil schema on error, got %+v", schema)
		}
	})

	t.Run("Case sensitive field names", func(t *testing.T) {
		// 大小写敏感，ID 和 id 应该是不同的字段
		schema, err := NewSchema("test", []Field{
			{Name: "id", Type: Int64},
			{Name: "ID", Type: Int64},
			{Name: "Id", Type: Int64},
		})

		if err != nil {
			t.Errorf("Expected no error for case-sensitive field names, got: %v", err)
			return
		}

		if len(schema.Fields) != 3 {
			t.Errorf("Expected 3 fields (case sensitive), got %d", len(schema.Fields))
		}
	})

	t.Run("Fields with all types", func(t *testing.T) {
		schema, err := NewSchema("test", []Field{
			{Name: "int_field", Type: Int64},
			{Name: "string_field", Type: String},
			{Name: "float_field", Type: Float64},
			{Name: "bool_field", Type: Bool},
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
			return
		}

		if len(schema.Fields) != 4 {
			t.Errorf("Expected 4 fields, got %d", len(schema.Fields))
		}

		// 验证每个字段的类型
		expectedTypes := map[string]FieldType{
			"int_field":    Int64,
			"string_field": String,
			"float_field":  Float64,
			"bool_field":   Bool,
		}

		for _, field := range schema.Fields {
			expectedType, exists := expectedTypes[field.Name]
			if !exists {
				t.Errorf("Unexpected field name: %s", field.Name)
				continue
			}
			if field.Type != expectedType {
				t.Errorf("Field %s: expected type %v, got %v", field.Name, expectedType, field.Type)
			}
		}
	})
}

// TestNewSchemaEdgeCases 测试边界情况
func TestNewSchemaEdgeCases(t *testing.T) {
	t.Run("Very long schema name", func(t *testing.T) {
		longName := strings.Repeat("a", 1000)
		schema, err := NewSchema(longName, []Field{
			{Name: "id", Type: Int64},
		})

		if err != nil {
			t.Errorf("Expected no error for long schema name, got: %v", err)
			return
		}

		if schema.Name != longName {
			t.Errorf("Expected schema name to be preserved")
		}
	})

	t.Run("Very long field name", func(t *testing.T) {
		longFieldName := strings.Repeat("b", 1000)
		schema, err := NewSchema("test", []Field{
			{Name: longFieldName, Type: Int64},
		})

		if err != nil {
			t.Errorf("Expected no error for long field name, got: %v", err)
			return
		}

		if schema.Fields[0].Name != longFieldName {
			t.Errorf("Expected field name to be preserved")
		}
	})

	t.Run("Many fields", func(t *testing.T) {
		fields := make([]Field, 100)
		for i := 0; i < 100; i++ {
			fields[i] = Field{
				Name: strings.Repeat("field", 1) + string(rune('a'+i)),
				Type: Int64,
			}
		}

		schema, err := NewSchema("test", fields)

		if err != nil {
			t.Errorf("Expected no error for many fields, got: %v", err)
			return
		}

		if len(schema.Fields) != 100 {
			t.Errorf("Expected 100 fields, got %d", len(schema.Fields))
		}
	})

	t.Run("Field with special characters", func(t *testing.T) {
		schema, err := NewSchema("test", []Field{
			{Name: "field_with_underscore", Type: Int64},
			{Name: "field123", Type: Int64},
			{Name: "字段名", Type: String}, // 中文字段名
		})

		if err != nil {
			t.Errorf("Expected no error for special characters, got: %v", err)
			return
		}

		if len(schema.Fields) != 3 {
			t.Errorf("Expected 3 fields with special characters, got %d", len(schema.Fields))
		}
	})
}

// TestNewSchemaConsistency 测试创建后的一致性
func TestNewSchemaConsistency(t *testing.T) {
	t.Run("Field order preserved", func(t *testing.T) {
		fields := []Field{
			{Name: "zebra", Type: String},
			{Name: "alpha", Type: Int64},
			{Name: "beta", Type: Float64},
		}

		schema, err := NewSchema("test", fields)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
			return
		}

		// 字段顺序应该保持不变
		for i, field := range schema.Fields {
			if field.Name != fields[i].Name {
				t.Errorf("Field order not preserved at index %d: expected %s, got %s",
					i, fields[i].Name, field.Name)
			}
		}
	})

	t.Run("Field properties preserved", func(t *testing.T) {
		fields := []Field{
			{Name: "id", Type: Int64, Indexed: true, Comment: "Primary key"},
			{Name: "name", Type: String, Indexed: false, Comment: "User name"},
		}

		schema, err := NewSchema("users", fields)

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
			return
		}

		// 验证所有属性都被保留
		if schema.Fields[0].Indexed != true {
			t.Errorf("Expected field 0 to be indexed")
		}
		if schema.Fields[1].Indexed != false {
			t.Errorf("Expected field 1 to not be indexed")
		}
		if schema.Fields[0].Comment != "Primary key" {
			t.Errorf("Expected field 0 comment to be preserved")
		}
		if schema.Fields[1].Comment != "User name" {
			t.Errorf("Expected field 1 comment to be preserved")
		}
	})
}

func TestExtractIndexValue(t *testing.T) {
	schema, err := NewSchema("test", []Field{
		{Name: "name", Type: String, Indexed: true, Comment: "名称"},
		{Name: "age", Type: Int64, Indexed: true, Comment: "年龄"},
	})
	if err != nil {
		t.Fatal(err)
	}

	data := map[string]any{
		"name": "David",
		"age":  float64(35), // JSON 解析后是 float64
	}

	// 提取 name
	name, err := schema.ExtractIndexValue("name", data)
	if err != nil {
		t.Errorf("Failed to extract name: %v", err)
	}
	if name != "David" {
		t.Errorf("Expected 'David', got %v", name)
	}

	// 提取 age (float64 → int64)
	age, err := schema.ExtractIndexValue("age", data)
	if err != nil {
		t.Errorf("Failed to extract age: %v", err)
	}
	if age != int64(35) {
		t.Errorf("Expected 35, got %v", age)
	}

	t.Log("Extract index value test passed!")
}

func TestPredefinedSchemas(t *testing.T) {
	// 测试 UserSchema
	userData := map[string]any{
		"name":        "Alice",
		"age":         25,
		"email":       "alice@example.com",
		"description": "Test user",
	}

	err := UserSchema.Validate(userData)
	if err != nil {
		t.Errorf("UserSchema validation failed: %v", err)
	}

	// 测试 LogSchema
	logData := map[string]any{
		"level":      "ERROR",
		"message":    "Something went wrong",
		"source":     "api",
		"error_code": 500,
	}

	err = LogSchema.Validate(logData)
	if err != nil {
		t.Errorf("LogSchema validation failed: %v", err)
	}

	t.Log("Predefined schemas test passed!")
}

// TestChecksumDeterminism 测试 checksum 的确定性
func TestChecksumDeterminism(t *testing.T) {
	// 创建相同的 Schema 多次
	for i := range 10 {
		s1, err := NewSchema("users", []Field{
			{Name: "name", Type: String, Indexed: true, Comment: "用户名"},
			{Name: "age", Type: Int64, Indexed: false, Comment: "年龄"},
		})
		if err != nil {
			t.Fatal(err)
		}

		s2, err := NewSchema("users", []Field{
			{Name: "name", Type: String, Indexed: true, Comment: "用户名"},
			{Name: "age", Type: Int64, Indexed: false, Comment: "年龄"},
		})
		if err != nil {
			t.Fatal(err)
		}

		checksum1, err := s1.ComputeChecksum()
		if err != nil {
			t.Fatal(err)
		}

		checksum2, err := s2.ComputeChecksum()
		if err != nil {
			t.Fatal(err)
		}

		if checksum1 != checksum2 {
			t.Errorf("Iteration %d: checksums should be equal, got %s and %s", i, checksum1, checksum2)
		}
	}

	t.Log("✅ Checksum is deterministic")
}

// TestChecksumFieldOrderIndependent 测试字段顺序不影响 checksum
func TestChecksumFieldOrderIndependent(t *testing.T) {
	s1, err := NewSchema("users", []Field{
		{Name: "name", Type: String, Indexed: true, Comment: "用户名"},
		{Name: "age", Type: Int64, Indexed: false, Comment: "年龄"},
	})
	if err != nil {
		t.Fatal(err)
	}

	s2, err := NewSchema("users", []Field{
		{Name: "age", Type: Int64, Indexed: false, Comment: "年龄"},
		{Name: "name", Type: String, Indexed: true, Comment: "用户名"},
	})
	if err != nil {
		t.Fatal(err)
	}

	checksum1, _ := s1.ComputeChecksum()
	checksum2, _ := s2.ComputeChecksum()

	if checksum1 != checksum2 {
		t.Errorf("Checksums should be equal regardless of field order, got %s and %s", checksum1, checksum2)
	} else {
		t.Logf("✅ Field order does not affect checksum (expected behavior)")
		t.Logf("   checksum: %s", checksum1)
	}
}

// TestChecksumDifferentData 测试不同 Schema 的 checksum 应该不同
func TestChecksumDifferentData(t *testing.T) {
	s1, err := NewSchema("users", []Field{
		{Name: "name", Type: String, Indexed: true, Comment: "用户名"},
	})
	if err != nil {
		t.Fatal(err)
	}

	s2, err := NewSchema("users", []Field{
		{Name: "name", Type: String, Indexed: false, Comment: "用户名"}, // Indexed 不同
	})
	if err != nil {
		t.Fatal(err)
	}

	checksum1, _ := s1.ComputeChecksum()
	checksum2, _ := s2.ComputeChecksum()

	if checksum1 == checksum2 {
		t.Error("Different schemas should have different checksums")
	} else {
		t.Log("✅ Different schemas have different checksums")
	}
}

// TestChecksumMultipleFieldOrders 测试多个字段的各种排列组合都产生相同 checksum
func TestChecksumMultipleFieldOrders(t *testing.T) {
	// 定义 4 个字段
	fieldA := Field{Name: "id", Type: Int64, Indexed: true, Comment: "ID"}
	fieldB := Field{Name: "name", Type: String, Indexed: false, Comment: "名称"}
	fieldC := Field{Name: "age", Type: Int64, Indexed: false, Comment: "年龄"}
	fieldD := Field{Name: "email", Type: String, Indexed: true, Comment: "邮箱"}

	// 创建不同顺序的 Schema
	mustNewSchema := func(name string, fields []Field) *Schema {
		s, err := NewSchema(name, fields)
		if err != nil {
			t.Fatalf("Failed to create schema: %v", err)
		}
		return s
	}

	schemas := []*Schema{
		mustNewSchema("test", []Field{fieldA, fieldB, fieldC, fieldD}), // 原始顺序
		mustNewSchema("test", []Field{fieldD, fieldC, fieldB, fieldA}), // 完全反转
		mustNewSchema("test", []Field{fieldB, fieldD, fieldA, fieldC}), // 随机顺序 1
		mustNewSchema("test", []Field{fieldC, fieldA, fieldD, fieldB}), // 随机顺序 2
		mustNewSchema("test", []Field{fieldD, fieldA, fieldC, fieldB}), // 随机顺序 3
	}

	// 计算所有 checksum
	checksums := make([]string, len(schemas))
	for i, s := range schemas {
		checksum, err := s.ComputeChecksum()
		if err != nil {
			t.Fatalf("Failed to compute checksum for schema %d: %v", i, err)
		}
		checksums[i] = checksum
	}

	// 验证所有 checksum 都相同
	expectedChecksum := checksums[0]
	for i := 1; i < len(checksums); i++ {
		if checksums[i] != expectedChecksum {
			t.Errorf("Schema %d has different checksum: expected %s, got %s", i, expectedChecksum, checksums[i])
		}
	}

	t.Logf("✅ All %d field permutations produce the same checksum", len(schemas))
	t.Logf("   checksum: %s", expectedChecksum)
}

// TestStructToFields 测试从结构体生成 Field 列表
func TestStructToFields(t *testing.T) {
	// 定义测试结构体
	type User struct {
		Name   string  `srdb:"name;indexed;comment:用户名"`
		Age    int64   `srdb:"age;comment:年龄"`
		Email  string  `srdb:"email;indexed;comment:邮箱"`
		Score  float64 `srdb:"score;comment:分数"`
		Active bool    `srdb:"active;comment:是否激活"`
	}

	// 生成 Field 列表
	fields, err := StructToFields(User{})
	if err != nil {
		t.Fatalf("StructToFields failed: %v", err)
	}

	// 验证字段数量
	if len(fields) != 5 {
		t.Errorf("Expected 5 fields, got %d", len(fields))
	}

	// 验证每个字段
	expectedFields := map[string]struct {
		Type    FieldType
		Indexed bool
		Comment string
	}{
		"name":   {String, true, "用户名"},
		"age":    {Int64, false, "年龄"},
		"email":  {String, true, "邮箱"},
		"score":  {Float64, false, "分数"},
		"active": {Bool, false, "是否激活"},
	}

	for _, field := range fields {
		expected, exists := expectedFields[field.Name]
		if !exists {
			t.Errorf("Unexpected field: %s", field.Name)
			continue
		}

		if field.Type != expected.Type {
			t.Errorf("Field %s: expected type %v, got %v", field.Name, expected.Type, field.Type)
		}

		if field.Indexed != expected.Indexed {
			t.Errorf("Field %s: expected indexed=%v, got %v", field.Name, expected.Indexed, field.Indexed)
		}

		if field.Comment != expected.Comment {
			t.Errorf("Field %s: expected comment=%s, got %s", field.Name, expected.Comment, field.Comment)
		}
	}

	t.Log("✓ StructToFields basic test passed")
}

// TestStructToFieldsDefaultName 测试默认字段名 (snake_case)
func TestStructToFieldsDefaultName(t *testing.T) {
	type Product struct {
		ProductName string // 没有 tag，应该使用 snake_case: product_name
		Price       int64  // 没有 tag，应该使用 snake_case: price
	}

	fields, err := StructToFields(Product{})
	if err != nil {
		t.Fatalf("StructToFields failed: %v", err)
	}

	if len(fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(fields))
	}

	// 验证默认字段名（snake_case）
	if fields[0].Name != "product_name" {
		t.Errorf("Expected field name 'product_name', got '%s'", fields[0].Name)
	}

	if fields[1].Name != "price" {
		t.Errorf("Expected field name 'price', got '%s'", fields[1].Name)
	}

	t.Log("✓ Default field name (snake_case) test passed")
}

// TestStructToFieldsIgnore 测试忽略字段
func TestStructToFieldsIgnore(t *testing.T) {
	type Order struct {
		OrderID   string `srdb:"order_id;comment:订单ID"`
		Internal  string `srdb:"-"` // 应该被忽略
		CreatedAt int64  `srdb:"created_at;comment:创建时间"`
	}

	fields, err := StructToFields(Order{})
	if err != nil {
		t.Fatalf("StructToFields failed: %v", err)
	}

	// 应该只有 2 个字段（Internal 被忽略）
	if len(fields) != 2 {
		t.Errorf("Expected 2 fields (excluding ignored field), got %d", len(fields))
	}

	// 验证没有 Internal 字段
	for _, field := range fields {
		if field.Name == "internal" || field.Name == "Internal" {
			t.Errorf("Field 'Internal' should have been ignored")
		}
	}

	t.Log("✓ Ignore field test passed")
}

// TestStructToFieldsPointer 测试指针类型
func TestStructToFieldsPointer(t *testing.T) {
	type Item struct {
		Name string `srdb:"name;comment:名称"`
	}

	// 使用指针
	fields, err := StructToFields(&Item{})
	if err != nil {
		t.Fatalf("StructToFields with pointer failed: %v", err)
	}

	if len(fields) != 1 {
		t.Errorf("Expected 1 field, got %d", len(fields))
	}

	if fields[0].Name != "name" {
		t.Errorf("Expected field name 'name', got '%s'", fields[0].Name)
	}

	t.Log("✓ Pointer type test passed")
}

// TestStructToFieldsAllTypes 测试所有支持的类型
func TestStructToFieldsAllTypes(t *testing.T) {
	type AllTypes struct {
		Int     int     `srdb:"int"`
		Int64   int64   `srdb:"int64"`
		Int32   int32   `srdb:"int32"`
		Int16   int16   `srdb:"int16"`
		Int8    int8    `srdb:"int8"`
		Uint    uint    `srdb:"uint"`
		Uint64  uint64  `srdb:"uint64"`
		Uint32  uint32  `srdb:"uint32"`
		Uint16  uint16  `srdb:"uint16"`
		Uint8   uint8   `srdb:"uint8"`
		String  string  `srdb:"string"`
		Float64 float64 `srdb:"float64"`
		Float32 float32 `srdb:"float32"`
		Bool    bool    `srdb:"bool"`
	}

	fields, err := StructToFields(AllTypes{})
	if err != nil {
		t.Fatalf("StructToFields failed: %v", err)
	}

	if len(fields) != 14 {
		t.Errorf("Expected 14 fields, got %d", len(fields))
	}

	// 验证所有类型都精确映射到对应的 FieldType
	expectedTypes := map[string]FieldType{
		"int":     Int,
		"int64":   Int64,
		"int32":   Int32,
		"int16":   Int16,
		"int8":    Int8,
		"uint":    Uint,
		"uint64":  Uint64,
		"uint32":  Uint32,
		"uint16":  Uint16,
		"uint8":   Uint8,
		"string":  String,
		"float64": Float64,
		"float32": Float32,
		"bool":    Bool,
	}

	for _, field := range fields {
		expectedType, exists := expectedTypes[field.Name]
		if !exists {
			t.Errorf("Unexpected field: %s", field.Name)
			continue
		}
		if field.Type != expectedType {
			t.Errorf("Field %s: expected %v, got %v", field.Name, expectedType, field.Type)
		}
	}

	t.Log("✓ All types test passed")
}

// TestStructToFieldsWithSchema 测试完整的使用流程
func TestStructToFieldsWithSchema(t *testing.T) {
	// 定义结构体
	type Customer struct {
		CustomerID string `srdb:"customer_id;indexed;comment:客户ID"`
		Name       string `srdb:"name;comment:客户名称"`
		Email      string `srdb:"email;indexed;comment:邮箱"`
		Balance    int64  `srdb:"balance;comment:余额"`
	}

	// 生成 Field 列表
	fields, err := StructToFields(Customer{})
	if err != nil {
		t.Fatalf("StructToFields failed: %v", err)
	}

	// 创建 Schema
	schema, err := NewSchema("customers", fields)
	if err != nil {
		t.Fatal(err)
	}

	// 验证 Schema
	if schema.Name != "customers" {
		t.Errorf("Expected schema name 'customers', got '%s'", schema.Name)
	}

	if len(schema.Fields) != 4 {
		t.Errorf("Expected 4 fields in schema, got %d", len(schema.Fields))
	}

	// 验证索引字段
	indexedFields := schema.GetIndexedFields()
	if len(indexedFields) != 2 {
		t.Errorf("Expected 2 indexed fields, got %d", len(indexedFields))
	}

	// 测试数据验证
	validData := map[string]any{
		"customer_id": "C001",
		"name":        "张三",
		"email":       "zhangsan@example.com",
		"balance":     int64(1000),
	}

	err = schema.Validate(validData)
	if err != nil {
		t.Errorf("Valid data should pass validation: %v", err)
	}

	// 测试无效数据
	invalidData := map[string]any{
		"customer_id": "C002",
		"name":        "李四",
		"email":       123, // 错误类型
		"balance":     int64(2000),
	}

	err = schema.Validate(invalidData)
	if err == nil {
		t.Error("Invalid data should fail validation")
	}

	t.Log("✓ Complete workflow test passed")
}

// TestStructToFieldsTagVariations 测试各种 tag 组合
func TestStructToFieldsTagVariations(t *testing.T) {
	type TestStruct struct {
		// 只有字段名
		Field1 string `srdb:"field1"`
		// 字段名 + indexed
		Field2 string `srdb:"field2;indexed"`
		// 字段名 + comment
		Field3 string `srdb:"field3;comment:字段3"`
		// 完整格式
		Field4 string `srdb:"field4;indexed;comment:字段4"`
		// 只有 indexed（使用默认字段名）
		Field5 string `srdb:";indexed"`
		// 只有 comment（使用默认字段名）
		Field6 string `srdb:";comment:字段6"`
		// 空 tag（使用默认字段名）
		Field7 string
		// indexed + comment（使用默认字段名）
		Field8 string `srdb:";indexed;comment:字段8"`
	}

	fields, err := StructToFields(TestStruct{})
	if err != nil {
		t.Fatalf("StructToFields failed: %v", err)
	}

	if len(fields) != 8 {
		t.Errorf("Expected 8 fields, got %d", len(fields))
	}

	// 验证各个字段
	tests := []struct {
		name    string
		indexed bool
		comment string
	}{
		{"field1", false, ""},
		{"field2", true, ""},
		{"field3", false, "字段3"},
		{"field4", true, "字段4"},
		{"field5", true, ""},
		{"field6", false, "字段6"},
		{"field7", false, ""},
		{"field8", true, "字段8"},
	}

	for i, test := range tests {
		if fields[i].Name != test.name {
			t.Errorf("Field %d: expected name %s, got %s", i+1, test.name, fields[i].Name)
		}
		if fields[i].Indexed != test.indexed {
			t.Errorf("Field %s: expected indexed=%v, got %v", test.name, test.indexed, fields[i].Indexed)
		}
		if fields[i].Comment != test.comment {
			t.Errorf("Field %s: expected comment=%s, got %s", test.name, test.comment, fields[i].Comment)
		}
	}

	t.Log("✓ Tag variations test passed")
}

// TestStructToFieldsErrors 测试错误情况
func TestStructToFieldsErrors(t *testing.T) {
	// 测试非结构体类型
	_, err := StructToFields("not a struct")
	if err == nil {
		t.Error("Expected error for non-struct type")
	}

	// 测试 nil
	_, err = StructToFields(nil)
	if err == nil {
		t.Error("Expected error for nil")
	}

	// 测试没有导出字段的结构体
	type Empty struct {
		private string // 未导出
	}
	_, err = StructToFields(Empty{})
	if err == nil {
		t.Error("Expected error for struct with no exported fields")
	}

	t.Log("✓ Error handling test passed")
}

// TestCamelToSnake 测试驼峰命名转 snake_case
func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// 基本测试
		{"UserName", "user_name"},
		{"EmailAddress", "email_address"},
		{"IsActive", "is_active"},

		// 单个单词
		{"Name", "name"},
		{"ID", "id"},

		// 连续大写字母
		{"HTTPServer", "http_server"},
		{"XMLParser", "xml_parser"},
		{"HTMLContent", "html_content"},
		{"URLPath", "url_path"},

		// 带数字
		{"User2Name", "user2_name"},
		{"Address1", "address1"},

		// 全小写
		{"username", "username"},

		// 全大写
		{"HTTP", "http"},
		{"API", "api"},

		// 混合情况
		{"getUserByID", "get_user_by_id"},
		{"HTTPSConnection", "https_connection"},
		{"createHTMLFile", "create_html_file"},

		// 边界情况
		{"A", "a"},
		{"AB", "ab"},
		{"AbC", "ab_c"},
	}

	for _, test := range tests {
		result := camelToSnake(test.input)
		if result != test.expected {
			t.Errorf("camelToSnake(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}

	t.Log("✓ camelToSnake test passed")
}

// TestStructToFieldsSnakeCase 测试默认使用 snake_case
func TestStructToFieldsSnakeCase(t *testing.T) {
	type User struct {
		UserName     string // 应该转为 user_name
		EmailAddress string // 应该转为 email_address
		IsActive     bool   // 应该转为 is_active
		HTTPEndpoint string // 应该转为 http_endpoint
		ID           int64  // 应该转为 id
	}

	fields, err := StructToFields(User{})
	if err != nil {
		t.Fatalf("StructToFields failed: %v", err)
	}

	expected := []string{"user_name", "email_address", "is_active", "http_endpoint", "id"}
	if len(fields) != len(expected) {
		t.Fatalf("Expected %d fields, got %d", len(expected), len(fields))
	}

	for i, exp := range expected {
		if fields[i].Name != exp {
			t.Errorf("Field %d: expected name %s, got %s", i, exp, fields[i].Name)
		}
	}

	t.Log("✓ Default snake_case test passed")
}

// TestStructToFieldsOverrideSnakeCase 测试可以覆盖默认 snake_case
func TestStructToFieldsOverrideSnakeCase(t *testing.T) {
	type User struct {
		UserName string `srdb:"username"`          // 覆盖默认的 user_name
		IsActive bool   `srdb:"active;comment:激活"` // 覆盖默认的 is_active
	}

	fields, err := StructToFields(User{})
	if err != nil {
		t.Fatalf("StructToFields failed: %v", err)
	}

	if len(fields) != 2 {
		t.Fatalf("Expected 2 fields, got %d", len(fields))
	}

	// 验证覆盖成功
	if fields[0].Name != "username" {
		t.Errorf("Expected field name 'username', got '%s'", fields[0].Name)
	}

	if fields[1].Name != "active" {
		t.Errorf("Expected field name 'active', got '%s'", fields[1].Name)
	}

	t.Log("✓ Override snake_case test passed")
}
