package srdb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// FieldType 字段类型
type FieldType int

const (
	FieldTypeInt64  FieldType = 1
	FieldTypeString FieldType = 2
	FieldTypeFloat  FieldType = 3
	FieldTypeBool   FieldType = 4
)

func (t FieldType) String() string {
	switch t {
	case FieldTypeInt64:
		return "int64"
	case FieldTypeString:
		return "string"
	case FieldTypeFloat:
		return "float64"
	case FieldTypeBool:
		return "bool"
	default:
		return "unknown"
	}
}

// Field 字段定义
type Field struct {
	Name    string    // 字段名
	Type    FieldType // 字段类型
	Indexed bool      // 是否建立索引
	Comment string    // 注释
}

// Schema 表结构定义
type Schema struct {
	Name   string  // Schema 名称
	Fields []Field // 字段列表
}

// NewSchema 创建 Schema
// 参数：
//   - name: Schema 名称，不能为空
//   - fields: 字段列表，至少需要 1 个字段
//
// 返回：
//   - *Schema: Schema 实例
//   - error: 错误信息
func NewSchema(name string, fields []Field) (*Schema, error) {
	// 验证 name
	if name == "" {
		return nil, NewError(ErrCodeSchemaInvalid, fmt.Errorf("schema name cannot be empty"))
	}

	// 验证 fields 数量
	if len(fields) == 0 {
		return nil, NewError(ErrCodeSchemaInvalid, fmt.Errorf("schema must have at least one field"))
	}

	// 验证字段名不能为空且不能重复
	fieldNames := make(map[string]bool)
	for i, field := range fields {
		if field.Name == "" {
			return nil, NewError(ErrCodeSchemaInvalid, fmt.Errorf("field at index %d has empty name", i))
		}
		if fieldNames[field.Name] {
			return nil, NewError(ErrCodeSchemaInvalid, fmt.Errorf("duplicate field name: %s", field.Name))
		}
		fieldNames[field.Name] = true
	}

	return &Schema{
		Name:   name,
		Fields: fields,
	}, nil
}

// StructToFields 从 Go 结构体生成 Field 列表
//
// 支持的 struct tag 格式：
//   - `srdb:"name"` - 指定字段名（默认使用 snake_case 转换）
//   - `srdb:"name;indexed"` - 指定字段名并标记为索引
//   - `srdb:"name;indexed;comment:用户名"` - 完整格式（字段名;索引标记;注释）
//   - `srdb:"-"` - 忽略该字段
//
// Tag 格式说明：
//   - 使用分号 `;` 分隔不同的部分
//   - 第一部分是字段名（可选，默认使用 snake_case 转换结构体字段名）
//   - `indexed` 标记该字段需要索引
//   - `comment:注释内容` 指定字段注释
//
// 默认字段名转换示例：
//   - UserName -> user_name
//   - EmailAddress -> email_address
//   - IsActive -> is_active
//
// 类型映射：
//   - int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8 -> FieldTypeInt64
//   - string -> FieldTypeString
//   - float64, float32 -> FieldTypeFloat
//   - bool -> FieldTypeBool
//
// 示例：
//   type User struct {
//       Name  string `srdb:"name;indexed;comment:用户名"`
//       Age   int64  `srdb:"age;comment:年龄"`
//       Email string `srdb:"email;indexed;comment:邮箱"`
//   }
//   fields, err := StructToFields(User{})
//
// 参数：
//   - v: 结构体实例或指针
//
// 返回：
//   - []Field: 字段列表
//   - error: 错误信息
func StructToFields(v any) ([]Field, error) {
	// 获取类型
	typ := reflect.TypeOf(v)
	if typ == nil {
		return nil, fmt.Errorf("invalid type: nil")
	}

	// 如果是指针，获取其指向的类型
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// 必须是结构体
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", typ.Kind())
	}

	var fields []Field

	// 遍历结构体字段
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// 跳过未导出的字段
		if !field.IsExported() {
			continue
		}

		// 解析 srdb tag
		tag := field.Tag.Get("srdb")
		if tag == "-" {
			// 忽略该字段
			continue
		}

		// 解析字段名、索引标记和注释
		fieldName := camelToSnake(field.Name) // 默认使用 snake_case 字段名
		indexed := false
		comment := ""

		if tag != "" {
			// 使用分号分隔各部分
			parts := strings.Split(tag, ";")

			for idx, part := range parts {
				part = strings.TrimSpace(part)

				if idx == 0 && part != "" {
					// 第一部分是字段名
					fieldName = part
				} else if part == "indexed" {
					// indexed 标记
					indexed = true
				} else if strings.HasPrefix(part, "comment:") {
					// comment:注释内容
					comment = strings.TrimPrefix(part, "comment:")
				}
			}
		}

		// 映射 Go 类型到 FieldType
		fieldType, err := goTypeToFieldType(field.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}

		fields = append(fields, Field{
			Name:    fieldName,
			Type:    fieldType,
			Indexed: indexed,
			Comment: comment,
		})
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("no exported fields found in struct")
	}

	return fields, nil
}

// goTypeToFieldType 将 Go 类型映射到 FieldType
func goTypeToFieldType(typ reflect.Type) (FieldType, error) {
	switch typ.Kind() {
	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8,
		reflect.Uint, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8:
		return FieldTypeInt64, nil
	case reflect.String:
		return FieldTypeString, nil
	case reflect.Float64, reflect.Float32:
		return FieldTypeFloat, nil
	case reflect.Bool:
		return FieldTypeBool, nil
	default:
		return 0, fmt.Errorf("unsupported type: %s", typ.Kind())
	}
}

// camelToSnake 将驼峰命名转换为 snake_case
//
// 示例：
//   - UserName -> user_name
//   - EmailAddress -> email_address
//   - IsActive -> is_active
//   - HTTPServer -> http_server
//   - ID -> id
func camelToSnake(s string) string {
	var result strings.Builder
	result.Grow(len(s) + 5) // 预分配空间

	for i, r := range s {
		// 如果是大写字母
		if r >= 'A' && r <= 'Z' {
			// 不是第一个字符，并且前一个字符不是大写，需要添加下划线
			if i > 0 {
				// 检查是否需要添加下划线
				// 规则：
				// 1. 前一个字符是小写字母 -> 添加下划线
				// 2. 当前是大写，后一个是小写（处理 HTTPServer -> http_server）-> 添加下划线
				prevChar := rune(s[i-1])
				needUnderscore := false

				if prevChar >= 'a' && prevChar <= 'z' {
					// 前一个是小写字母
					needUnderscore = true
				} else if prevChar >= 'A' && prevChar <= 'Z' {
					// 前一个是大写字母，检查后一个
					if i+1 < len(s) {
						nextChar := rune(s[i+1])
						if nextChar >= 'a' && nextChar <= 'z' {
							// 后一个是小写字母，说明是新单词开始
							needUnderscore = true
						}
					}
				} else {
					// 前一个是数字或其他字符
					needUnderscore = true
				}

				if needUnderscore {
					result.WriteRune('_')
				}
			}
			// 转换为小写
			result.WriteRune(r + 32) // 'A' -> 'a' 的 ASCII 差值是 32
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// GetField 获取字段定义
func (s *Schema) GetField(name string) (*Field, error) {
	for i := range s.Fields {
		if s.Fields[i].Name == name {
			return &s.Fields[i], nil
		}
	}
	return nil, NewErrorf(ErrCodeFieldNotFound, "field %s not found", name)
}

// GetIndexedFields 获取所有需要索引的字段
func (s *Schema) GetIndexedFields() []Field {
	var fields []Field
	for _, field := range s.Fields {
		if field.Indexed {
			fields = append(fields, field)
		}
	}
	return fields
}

// Validate 验证数据是否符合 Schema
func (s *Schema) Validate(data map[string]any) error {
	for _, field := range s.Fields {
		value, exists := data[field.Name]
		if !exists {
			// 字段不存在，允许（可选字段）
			continue
		}

		// 验证类型
		if err := s.validateType(field.Type, value); err != nil {
			return fmt.Errorf("field %s: %v", field.Name, err)
		}
	}
	return nil
}

// ValidateType 验证值的类型（导出方法）
func (s *Schema) ValidateType(typ FieldType, value any) error {
	return s.validateType(typ, value)
}

// validateType 验证值的类型
func (s *Schema) validateType(typ FieldType, value any) error {
	switch typ {
	case FieldTypeInt64:
		switch value.(type) {
		case int, int64, int32, int16, int8:
			return nil
		case float64:
			// JSON 解析后数字都是 float64
			return nil
		default:
			return fmt.Errorf("expected int64, got %T", value)
		}
	case FieldTypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case FieldTypeFloat:
		switch value.(type) {
		case float64, float32:
			return nil
		default:
			return fmt.Errorf("expected float, got %T", value)
		}
	case FieldTypeBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected bool, got %T", value)
		}
	}
	return nil
}

// ExtractIndexValue 提取索引值
func (s *Schema) ExtractIndexValue(field string, data map[string]any) (any, error) {
	fieldDef, err := s.GetField(field)
	if err != nil {
		return nil, err
	}

	value, exists := data[field]
	if !exists {
		return nil, NewErrorf(ErrCodeFieldNotFound, "field %s not found in data", field)
	}

	// 类型转换
	switch fieldDef.Type {
	case FieldTypeInt64:
		switch v := value.(type) {
		case int:
			return int64(v), nil
		case int64:
			return v, nil
		case float64:
			return int64(v), nil
		default:
			return nil, fmt.Errorf("cannot convert %T to int64", value)
		}
	case FieldTypeString:
		if v, ok := value.(string); ok {
			return v, nil
		}
		return nil, fmt.Errorf("cannot convert %T to string", value)
	case FieldTypeFloat:
		if v, ok := value.(float64); ok {
			return v, nil
		}
		return nil, fmt.Errorf("cannot convert %T to float64", value)
	case FieldTypeBool:
		if v, ok := value.(bool); ok {
			return v, nil
		}
		return nil, fmt.Errorf("cannot convert %T to bool", value)
	}

	return nil, fmt.Errorf("unsupported type: %v", fieldDef.Type)
}

// ComputeChecksum 计算 Schema 的 SHA256 校验和
// 使用确定性的字符串拼接算法，不依赖 json.Marshal
// 这样即使 Schema struct 添加新字段，只要核心内容（Name、Fields）不变，checksum 就不会变
// 重要：字段顺序不影响 checksum，会先按字段名排序
// 格式: "name:<name>;fields:<field1_name>:<field1_type>:<field1_indexed>:<field1_comment>,<field2>..."
func (s *Schema) ComputeChecksum() (string, error) {
	var builder strings.Builder

	// 1. Schema 名称
	builder.WriteString("name:")
	builder.WriteString(s.Name)
	builder.WriteString(";")

	// 2. 复制字段列表并按字段名排序（保证顺序无关性）
	sortedFields := make([]Field, len(s.Fields))
	copy(sortedFields, s.Fields)
	sort.Slice(sortedFields, func(i, j int) bool {
		return sortedFields[i].Name < sortedFields[j].Name
	})

	// 3. 拼接排序后的字段列表
	builder.WriteString("fields:")
	for i, field := range sortedFields {
		if i > 0 {
			builder.WriteString(",")
		}
		// 字段格式: name:type:indexed:comment
		builder.WriteString(field.Name)
		builder.WriteString(":")
		builder.WriteString(field.Type.String())
		builder.WriteString(":")
		if field.Indexed {
			builder.WriteString("1")
		} else {
			builder.WriteString("0")
		}
		builder.WriteString(":")
		builder.WriteString(field.Comment)
	}

	// 计算 SHA256
	hash := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(hash[:]), nil
}

// SchemaFile Schema 文件格式（带校验）
type SchemaFile struct {
	Version   int     `json:"version"`   // 文件格式版本
	Timestamp int64   `json:"timestamp"` // 保存时间戳
	Checksum  string  `json:"checksum"`  // Schema 内容的 SHA256 校验和
	Schema    *Schema `json:"schema"`    // Schema 内容
}

// NewSchemaFile 创建带校验和的 Schema 文件
func NewSchemaFile(schema *Schema) (*SchemaFile, error) {
	checksum, err := schema.ComputeChecksum()
	if err != nil {
		return nil, fmt.Errorf("compute checksum: %w", err)
	}

	return &SchemaFile{
		Version:   1, // 当前文件格式版本
		Timestamp: time.Now().Unix(),
		Checksum:  checksum,
		Schema:    schema,
	}, nil
}

// Verify 验证 Schema 文件的完整性
func (sf *SchemaFile) Verify() error {
	if sf.Schema == nil {
		return fmt.Errorf("schema is nil")
	}

	// 重新计算 checksum
	actualChecksum, err := sf.Schema.ComputeChecksum()
	if err != nil {
		return fmt.Errorf("compute checksum: %w", err)
	}

	// 对比 checksum
	if actualChecksum != sf.Checksum {
		return fmt.Errorf("schema checksum mismatch: expected %s, got %s (schema may have been tampered with)", sf.Checksum, actualChecksum)
	}

	return nil
}
