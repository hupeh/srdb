package srdb

import (
	"testing"
)

// UserSchema 用户表 Schema
var UserSchema = NewSchema("users", []Field{
	{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
	{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
	{Name: "email", Type: FieldTypeString, Indexed: true, Comment: "邮箱"},
	{Name: "description", Type: FieldTypeString, Indexed: false, Comment: "描述"},
})

// LogSchema 日志表 Schema
var LogSchema = NewSchema("logs", []Field{
	{Name: "level", Type: FieldTypeString, Indexed: true, Comment: "日志级别"},
	{Name: "message", Type: FieldTypeString, Indexed: false, Comment: "日志消息"},
	{Name: "source", Type: FieldTypeString, Indexed: true, Comment: "来源"},
	{Name: "error_code", Type: FieldTypeInt64, Indexed: true, Comment: "错误码"},
})

// OrderSchema 订单表 Schema
var OrderSchema = NewSchema("orders", []Field{
	{Name: "order_id", Type: FieldTypeString, Indexed: true, Comment: "订单ID"},
	{Name: "user_id", Type: FieldTypeInt64, Indexed: true, Comment: "用户ID"},
	{Name: "amount", Type: FieldTypeFloat, Indexed: true, Comment: "金额"},
	{Name: "status", Type: FieldTypeString, Indexed: true, Comment: "状态"},
	{Name: "paid", Type: FieldTypeBool, Indexed: true, Comment: "是否支付"},
})

func TestSchema(t *testing.T) {
	// 创建 Schema
	schema := NewSchema("test", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "名称"},
		{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
		{Name: "score", Type: FieldTypeFloat, Indexed: false, Comment: "分数"},
	})

	// 测试数据
	data := map[string]any{
		"name":  "Alice",
		"age":   25,
		"score": 95.5,
	}

	// 验证
	err := schema.Validate(data)
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
	schema := NewSchema("test", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "名称"},
		{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
	})

	// 正确的数据
	validData := map[string]any{
		"name": "Bob",
		"age":  30,
	}

	err := schema.Validate(validData)
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

func TestExtractIndexValue(t *testing.T) {
	schema := NewSchema("test", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "名称"},
		{Name: "age", Type: FieldTypeInt64, Indexed: true, Comment: "年龄"},
	})

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
		s1 := NewSchema("users", []Field{
			{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
			{Name: "age", Type: FieldTypeInt64, Indexed: false, Comment: "年龄"},
		})

		s2 := NewSchema("users", []Field{
			{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
			{Name: "age", Type: FieldTypeInt64, Indexed: false, Comment: "年龄"},
		})

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
	s1 := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: false, Comment: "年龄"},
	})

	s2 := NewSchema("users", []Field{
		{Name: "age", Type: FieldTypeInt64, Indexed: false, Comment: "年龄"},
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
	})

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
	s1 := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: true, Comment: "用户名"},
	})

	s2 := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "用户名"}, // Indexed 不同
	})

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
	fieldA := Field{Name: "id", Type: FieldTypeInt64, Indexed: true, Comment: "ID"}
	fieldB := Field{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "名称"}
	fieldC := Field{Name: "age", Type: FieldTypeInt64, Indexed: false, Comment: "年龄"}
	fieldD := Field{Name: "email", Type: FieldTypeString, Indexed: true, Comment: "邮箱"}

	// 创建不同顺序的 Schema
	schemas := []*Schema{
		NewSchema("test", []Field{fieldA, fieldB, fieldC, fieldD}), // 原始顺序
		NewSchema("test", []Field{fieldD, fieldC, fieldB, fieldA}), // 完全反转
		NewSchema("test", []Field{fieldB, fieldD, fieldA, fieldC}), // 随机顺序 1
		NewSchema("test", []Field{fieldC, fieldA, fieldD, fieldB}), // 随机顺序 2
		NewSchema("test", []Field{fieldD, fieldA, fieldC, fieldB}), // 随机顺序 3
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
