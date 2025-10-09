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
		"name":   {FieldTypeString, true, "用户名"},
		"age":    {FieldTypeInt64, false, "年龄"},
		"email":  {FieldTypeString, true, "邮箱"},
		"score":  {FieldTypeFloat, false, "分数"},
		"active": {FieldTypeBool, false, "是否激活"},
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

	// 验证所有整数类型都映射到 FieldTypeInt64
	intFields := []string{"int", "int64", "int32", "int16", "int8", "uint", "uint64", "uint32", "uint16", "uint8"}
	for _, name := range intFields {
		found := false
		for _, field := range fields {
			if field.Name == name {
				found = true
				if field.Type != FieldTypeInt64 {
					t.Errorf("Field %s: expected FieldTypeInt64, got %v", name, field.Type)
				}
				break
			}
		}
		if !found {
			t.Errorf("Field %s not found", name)
		}
	}

	// 验证浮点类型
	for _, field := range fields {
		if field.Name == "float64" || field.Name == "float32" {
			if field.Type != FieldTypeFloat {
				t.Errorf("Field %s: expected FieldTypeFloat, got %v", field.Name, field.Type)
			}
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
	schema := NewSchema("customers", fields)

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
