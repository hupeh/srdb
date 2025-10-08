package srdb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

// New 创建 Schema
func NewSchema(name string, fields []Field) *Schema {
	return &Schema{
		Name:   name,
		Fields: fields,
	}
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
