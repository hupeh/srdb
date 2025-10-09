package srdb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// FieldType 字段类型（对应 Go 基础类型）
type FieldType int

const (
	_ FieldType = iota

	// 有符号整数类型
	Int
	Int8
	Int16
	Int32
	Int64

	// 无符号整数类型
	Uint
	Uint8
	Uint16
	Uint32
	Uint64

	// 浮点类型
	Float32
	Float64

	// 字符串类型
	String

	// 布尔类型
	Bool

	// Byte 和 Rune 类型（独立类型，语义上对应 Go 的 byte 和 rune）
	Byte // byte 类型（底层为 uint8）
	Rune // rune 类型（底层为 int32）

	// Decimal 类型（高精度十进制，用于金融计算）
	Decimal

	// 时间类型
	Time     // time.Time 时间戳
	Duration // time.Duration 时间间隔
)

func (t FieldType) String() string {
	switch t {
	case Int:
		return "int"
	case Int8:
		return "int8"
	case Int16:
		return "int16"
	case Int32:
		return "int32"
	case Int64:
		return "int64"
	case Uint:
		return "uint"
	case Uint8:
		return "uint8"
	case Uint16:
		return "uint16"
	case Uint32:
		return "uint32"
	case Uint64:
		return "uint64"
	case Float32:
		return "float32"
	case Float64:
		return "float64"
	case String:
		return "string"
	case Bool:
		return "bool"
	case Byte:
		return "byte"
	case Rune:
		return "rune"
	case Decimal:
		return "decimal"
	case Time:
		return "time"
	case Duration:
		return "duration"
	default:
		return "unknown"
	}
}

// Field 字段定义
type Field struct {
	Name     string    // 字段名
	Type     FieldType // 字段类型
	Indexed  bool      // 是否建立索引
	Nullable bool      // 是否允许 NULL 值
	Comment  string    // 注释
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
//   - `srdb:"name;nullable"` - 指定字段名并标记为可空
//   - `srdb:"name;indexed;nullable;comment:用户名"` - 完整格式
//   - `srdb:"-"` - 忽略该字段
//
// Tag 格式说明：
//   - 使用分号 `;` 分隔不同的部分
//   - 第一部分是字段名（可选，默认使用 snake_case 转换结构体字段名）
//   - `indexed` 标记该字段需要索引
//   - `nullable` 标记该字段允许 NULL 值
//   - `comment:注释内容` 指定字段注释
//
// 默认字段名转换示例：
//   - UserName -> user_name
//   - EmailAddress -> email_address
//   - IsActive -> is_active
//
// 类型映射（精确映射到 Go 基础类型）：
//   - int -> FieldTypeInt
//   - int8 -> Int8
//   - int16 -> Int16
//   - int32 -> Int32
//   - int64 -> Int64
//   - uint -> Uint
//   - uint8 (byte) -> Uint8 或 Byte
//   - uint16 -> Uint16
//   - uint32 -> Uint32
//   - uint64 -> Uint64
//   - float32 -> Float32
//   - float64 -> Float64
//   - string -> String
//   - bool -> Bool
//   - rune -> Rune
//   - decimal.Decimal -> Decimal
//
// 示例：
//
//	type User struct {
//	    Name  string  `srdb:"name;indexed;comment:用户名"`
//	    Age   int64   `srdb:"age;comment:年龄"`
//	    Email *string `srdb:"email;nullable;comment:邮箱（可选）"`
//	}
//	fields, err := StructToFields(User{})
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
	if typ.Kind() == reflect.Pointer {
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

		// 解析字段名、索引标记、nullable 和注释
		fieldName := camelToSnake(field.Name) // 默认使用 snake_case 字段名
		indexed := false
		nullable := false
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
				} else if part == "nullable" {
					// nullable 标记
					nullable = true
				} else if after, ok := strings.CutPrefix(part, "comment:"); ok {
					// comment:注释内容
					comment = after
				}
			}
		}

		// 映射 Go 类型到 FieldType
		fieldType, err := goTypeToFieldType(field.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name, err)
		}

		fields = append(fields, Field{
			Name:     fieldName,
			Type:     fieldType,
			Indexed:  indexed,
			Nullable: nullable,
			Comment:  comment,
		})
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("no exported fields found in struct")
	}

	return fields, nil
}

// goTypeToFieldType 将 Go 类型精确映射到 FieldType
func goTypeToFieldType(typ reflect.Type) (FieldType, error) {
	// 特殊处理：decimal.Decimal
	if typ.PkgPath() == "github.com/shopspring/decimal" && typ.Name() == "Decimal" {
		return Decimal, nil
	}

	// 特殊处理：time.Time
	if typ.PkgPath() == "time" && typ.Name() == "Time" {
		return Time, nil
	}

	// 特殊处理：time.Duration
	if typ.PkgPath() == "time" && typ.Name() == "Duration" {
		return Duration, nil
	}

	switch typ.Kind() {
	case reflect.Int:
		return Int, nil
	case reflect.Int8:
		// byte 在 Go 中是 uint8 的别名，但在反射中无法区分
		// 所以 int8 总是映射到 Int8
		return Int8, nil
	case reflect.Int16:
		return Int16, nil
	case reflect.Int32:
		// rune 在 Go 中是 int32 的别名
		// 如果 type 名称是 "rune"，则映射到 Rune
		if typ.Name() == "rune" {
			return Rune, nil
		}
		return Int32, nil
	case reflect.Int64:
		return Int64, nil
	case reflect.Uint:
		return Uint, nil
	case reflect.Uint8:
		// byte 是 uint8 的别名
		// 如果 type 名称是 "byte"，则映射到 Byte
		if typ.Name() == "byte" {
			return Byte, nil
		}
		return Uint8, nil
	case reflect.Uint16:
		return Uint16, nil
	case reflect.Uint32:
		return Uint32, nil
	case reflect.Uint64:
		return Uint64, nil
	case reflect.Float32:
		return Float32, nil
	case reflect.Float64:
		return Float64, nil
	case reflect.String:
		return String, nil
	case reflect.Bool:
		return Bool, nil
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

		// 检查 NULL 值
		if value == nil {
			if !field.Nullable {
				return fmt.Errorf("field %s: NULL value not allowed (field is not nullable)", field.Name)
			}
			// NULL 值且字段允许 NULL，跳过类型验证
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
	// 有符号整数类型
	case Int, Int8, Int16, Int32, Int64:
		switch v := value.(type) {
		case int, int8, int16, int32, int64:
			return nil
		case uint, uint8, uint16, uint32, uint64:
			return nil // 允许无符号整数，稍后转换
		case float64:
			// JSON 解析后数字都是 float64，检查是否为整数
			if v == float64(int64(v)) {
				return nil
			}
			return fmt.Errorf("expected integer, got float %v", v)
		case float32:
			if v == float32(int32(v)) {
				return nil
			}
			return fmt.Errorf("expected integer, got float %v", v)
		default:
			return fmt.Errorf("expected integer type (%s), got %T", typ.String(), value)
		}

	// 无符号整数类型
	case Uint, Uint8, Uint16, Uint32, Uint64:
		switch v := value.(type) {
		case uint, uint8, uint16, uint32, uint64:
			return nil
		case int, int8, int16, int32, int64:
			// 允许有符号整数，但必须非负
			if reflect.ValueOf(v).Int() < 0 {
				return fmt.Errorf("expected non-negative integer for %s, got %v", typ.String(), v)
			}
			return nil
		case float64:
			if v < 0 || v != float64(uint64(v)) {
				return fmt.Errorf("expected non-negative integer for %s, got %v", typ.String(), v)
			}
			return nil
		case float32:
			if v < 0 || v != float32(uint32(v)) {
				return fmt.Errorf("expected non-negative integer for %s, got %v", typ.String(), v)
			}
			return nil
		default:
			return fmt.Errorf("expected unsigned integer type (%s), got %T", typ.String(), value)
		}

	// Byte 类型（底层为 uint8）
	case Byte:
		switch v := value.(type) {
		case uint8: // byte 和 uint8 是同一类型，只需一个 case
			return nil
		case int, int8, int16, int32, int64:
			if reflect.ValueOf(v).Int() < 0 || reflect.ValueOf(v).Int() > 255 {
				return fmt.Errorf("expected byte value (0-255), got %v", v)
			}
			return nil
		case uint, uint16, uint32, uint64:
			if reflect.ValueOf(v).Uint() > 255 {
				return fmt.Errorf("expected byte value (0-255), got %v", v)
			}
			return nil
		case float64:
			if v < 0 || v > 255 || v != float64(uint8(v)) {
				return fmt.Errorf("expected byte value (0-255), got %v", v)
			}
			return nil
		default:
			return fmt.Errorf("expected byte type, got %T", value)
		}

	// Rune 类型（底层为 int32）
	case Rune:
		switch v := value.(type) {
		case int32: // rune 和 int32 是同一类型，只需一个 case
			return nil
		case int, int8, int16, int64:
			return nil
		case uint, uint8, uint16, uint32, uint64:
			return nil
		case float64:
			if v != float64(int32(v)) {
				return fmt.Errorf("expected rune (int32), got float %v", v)
			}
			return nil
		case string:
			// 允许单字符字符串转换为 rune
			if len([]rune(v)) == 1 {
				return nil
			}
			return fmt.Errorf("expected single character string for rune, got %q", v)
		default:
			return fmt.Errorf("expected rune type, got %T", value)
		}

	// 浮点类型
	case Float32, Float64:
		switch value.(type) {
		case float32, float64:
			return nil
		case int, int8, int16, int32, int64:
			return nil // 整数可以转换为浮点数
		case uint, uint8, uint16, uint32, uint64:
			return nil
		default:
			return fmt.Errorf("expected float type (%s), got %T", typ.String(), value)
		}

	// Decimal 类型
	case Decimal:
		switch v := value.(type) {
		case decimal.Decimal:
			return nil
		case string:
			// 允许字符串转换为 Decimal
			_, err := decimal.NewFromString(v)
			if err != nil {
				return fmt.Errorf("expected decimal value, got invalid string %q: %v", v, err)
			}
			return nil
		case float32, float64:
			return nil // 浮点数可以转换为 Decimal
		case int, int8, int16, int32, int64:
			return nil
		case uint, uint8, uint16, uint32, uint64:
			return nil
		default:
			return fmt.Errorf("expected decimal type, got %T", value)
		}

	// 字符串类型
	case String:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}

	// 布尔类型
	case Bool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected bool, got %T", value)
		}

	// 时间类型
	case Time:
		switch v := value.(type) {
		case time.Time:
			return nil
		case string:
			// 允许字符串转换为 Time (RFC3339 格式)
			_, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return fmt.Errorf("expected time value, got invalid string %q: %v", v, err)
			}
			return nil
		case int64:
			// 允许 Unix 时间戳（秒）
			return nil
		default:
			return fmt.Errorf("expected time type, got %T", value)
		}

	// 时间间隔类型
	case Duration:
		switch v := value.(type) {
		case time.Duration:
			return nil
		case int64:
			// 允许 int64 (纳秒)
			return nil
		case string:
			// 允许字符串转换为 Duration (如 "1h30m")
			_, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("expected duration value, got invalid string %q: %v", v, err)
			}
			return nil
		default:
			return fmt.Errorf("expected duration type, got %T", value)
		}

	default:
		return fmt.Errorf("unknown field type: %v", typ)
	}
	return nil
}

// ExtractIndexValue 提取索引值（支持类型转换）
func (s *Schema) ExtractIndexValue(field string, data map[string]any) (any, error) {
	fieldDef, err := s.GetField(field)
	if err != nil {
		return nil, err
	}

	value, exists := data[field]
	if !exists {
		return nil, NewErrorf(ErrCodeFieldNotFound, "field %s not found in data", field)
	}

	return convertValue(value, fieldDef.Type)
}

// convertValue 将值转换为目标类型
func convertValue(value any, targetType FieldType) (any, error) {
	switch targetType {
	// 有符号整数类型
	case Int:
		return convertToInt(value)
	case Int8:
		return convertToInt8(value)
	case Int16:
		return convertToInt16(value)
	case Int32:
		return convertToInt32(value)
	case Int64:
		return convertToInt64(value)

	// 无符号整数类型
	case Uint:
		return convertToUint(value)
	case Uint8:
		return convertToUint8(value)
	case Uint16:
		return convertToUint16(value)
	case Uint32:
		return convertToUint32(value)
	case Uint64:
		return convertToUint64(value)

	// Byte 和 Rune 类型
	case Byte:
		return convertToByte(value)
	case Rune:
		return convertToRune(value)

	// 浮点类型
	case Float32:
		return convertToFloat32(value)
	case Float64:
		return convertToFloat64(value)

	// Decimal 类型
	case Decimal:
		return convertToDecimal(value)

	// 字符串类型
	case String:
		if v, ok := value.(string); ok {
			return v, nil
		}
		return nil, fmt.Errorf("cannot convert %T to string", value)

	// 布尔类型
	case Bool:
		if v, ok := value.(bool); ok {
			return v, nil
		}
		return nil, fmt.Errorf("cannot convert %T to bool", value)

	// 时间类型
	case Time:
		return convertToTime(value)

	// 时间间隔类型
	case Duration:
		return convertToDuration(value)

	default:
		return nil, fmt.Errorf("unsupported type: %v", targetType)
	}
}

// 类型转换辅助函数
func convertToInt(v any) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case int8:
		return int(val), nil
	case int16:
		return int(val), nil
	case int32:
		return int(val), nil
	case int64:
		return int(val), nil
	case uint:
		return int(val), nil
	case uint8:
		return int(val), nil
	case uint16:
		return int(val), nil
	case uint32:
		return int(val), nil
	case uint64:
		return int(val), nil
	case float32:
		return int(val), nil
	case float64:
		return int(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

func convertToInt8(v any) (int8, error) {
	switch val := v.(type) {
	case int:
		return int8(val), nil
	case int8:
		return val, nil
	case int16:
		return int8(val), nil
	case int32:
		return int8(val), nil
	case int64:
		return int8(val), nil
	case uint:
		return int8(val), nil
	case uint8:
		return int8(val), nil
	case uint16:
		return int8(val), nil
	case uint32:
		return int8(val), nil
	case uint64:
		return int8(val), nil
	case float32:
		return int8(val), nil
	case float64:
		return int8(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int8", v)
	}
}

func convertToInt16(v any) (int16, error) {
	switch val := v.(type) {
	case int:
		return int16(val), nil
	case int8:
		return int16(val), nil
	case int16:
		return val, nil
	case int32:
		return int16(val), nil
	case int64:
		return int16(val), nil
	case uint:
		return int16(val), nil
	case uint8:
		return int16(val), nil
	case uint16:
		return int16(val), nil
	case uint32:
		return int16(val), nil
	case uint64:
		return int16(val), nil
	case float32:
		return int16(val), nil
	case float64:
		return int16(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int16", v)
	}
}

func convertToInt32(v any) (int32, error) {
	switch val := v.(type) {
	case int:
		return int32(val), nil
	case int8:
		return int32(val), nil
	case int16:
		return int32(val), nil
	case int32:
		return val, nil
	case int64:
		return int32(val), nil
	case uint:
		return int32(val), nil
	case uint8:
		return int32(val), nil
	case uint16:
		return int32(val), nil
	case uint32:
		return int32(val), nil
	case uint64:
		return int32(val), nil
	case float32:
		return int32(val), nil
	case float64:
		return int32(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int32", v)
	}
}

func convertToInt64(v any) (int64, error) {
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int8:
		return int64(val), nil
	case int16:
		return int64(val), nil
	case int32:
		return int64(val), nil
	case int64:
		return val, nil
	case uint:
		return int64(val), nil
	case uint8:
		return int64(val), nil
	case uint16:
		return int64(val), nil
	case uint32:
		return int64(val), nil
	case uint64:
		return int64(val), nil
	case float32:
		return int64(val), nil
	case float64:
		return int64(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

func convertToUint(v any) (uint, error) {
	switch val := v.(type) {
	case uint:
		return val, nil
	case uint8:
		return uint(val), nil
	case uint16:
		return uint(val), nil
	case uint32:
		return uint(val), nil
	case uint64:
		return uint(val), nil
	case int:
		if val < 0 {
			return 0, fmt.Errorf("cannot convert negative int %d to uint", val)
		}
		return uint(val), nil
	case int8:
		if val < 0 {
			return 0, fmt.Errorf("cannot convert negative int8 %d to uint", val)
		}
		return uint(val), nil
	case int16:
		if val < 0 {
			return 0, fmt.Errorf("cannot convert negative int16 %d to uint", val)
		}
		return uint(val), nil
	case int32:
		if val < 0 {
			return 0, fmt.Errorf("cannot convert negative int32 %d to uint", val)
		}
		return uint(val), nil
	case int64:
		if val < 0 {
			return 0, fmt.Errorf("cannot convert negative int64 %d to uint", val)
		}
		return uint(val), nil
	case float32:
		if val < 0 {
			return 0, fmt.Errorf("cannot convert negative float32 %v to uint", val)
		}
		return uint(val), nil
	case float64:
		if val < 0 {
			return 0, fmt.Errorf("cannot convert negative float64 %v to uint", val)
		}
		return uint(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to uint", v)
	}
}

func convertToUint8(v any) (uint8, error) {
	val, err := convertToUint(v)
	if err != nil {
		return 0, err
	}
	return uint8(val), nil
}

func convertToUint16(v any) (uint16, error) {
	val, err := convertToUint(v)
	if err != nil {
		return 0, err
	}
	return uint16(val), nil
}

func convertToUint32(v any) (uint32, error) {
	val, err := convertToUint(v)
	if err != nil {
		return 0, err
	}
	return uint32(val), nil
}

func convertToUint64(v any) (uint64, error) {
	val, err := convertToUint(v)
	if err != nil {
		return 0, err
	}
	return uint64(val), nil
}

func convertToFloat32(v any) (float32, error) {
	switch val := v.(type) {
	case float32:
		return val, nil
	case float64:
		return float32(val), nil
	case int:
		return float32(val), nil
	case int8:
		return float32(val), nil
	case int16:
		return float32(val), nil
	case int32:
		return float32(val), nil
	case int64:
		return float32(val), nil
	case uint:
		return float32(val), nil
	case uint8:
		return float32(val), nil
	case uint16:
		return float32(val), nil
	case uint32:
		return float32(val), nil
	case uint64:
		return float32(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float32", v)
	}
}

func convertToFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int8:
		return float64(val), nil
	case int16:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case uint:
		return float64(val), nil
	case uint8:
		return float64(val), nil
	case uint16:
		return float64(val), nil
	case uint32:
		return float64(val), nil
	case uint64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// convertToByte 将值转换为 byte (uint8)
func convertToByte(v any) (byte, error) {
	switch val := v.(type) {
	case uint8: // byte 和 uint8 是同一类型
		return val, nil
	case uint, uint16, uint32, uint64:
		uval := reflect.ValueOf(val).Uint()
		if uval > 255 {
			return 0, fmt.Errorf("value %d out of byte range (0-255)", uval)
		}
		return byte(uval), nil
	case int, int8, int16, int32, int64:
		ival := reflect.ValueOf(val).Int()
		if ival < 0 || ival > 255 {
			return 0, fmt.Errorf("value %d out of byte range (0-255)", ival)
		}
		return byte(ival), nil
	case float32, float64:
		fval := reflect.ValueOf(val).Float()
		if fval < 0 || fval > 255 {
			return 0, fmt.Errorf("value %f out of byte range (0-255)", fval)
		}
		return byte(fval), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to byte", v)
	}
}

// convertToRune 将值转换为 rune (int32)
func convertToRune(v any) (rune, error) {
	switch val := v.(type) {
	case int32: // rune 和 int32 是同一类型
		return val, nil
	case int, int8, int16, int64:
		return rune(reflect.ValueOf(val).Int()), nil
	case uint, uint8, uint16, uint32, uint64:
		return rune(reflect.ValueOf(val).Uint()), nil
	case float32, float64:
		return rune(reflect.ValueOf(val).Float()), nil
	case string:
		// 单字符字符串转换为 rune
		runes := []rune(val)
		if len(runes) == 1 {
			return runes[0], nil
		}
		return 0, fmt.Errorf("cannot convert multi-character string %q to rune", val)
	default:
		return 0, fmt.Errorf("cannot convert %T to rune", v)
	}
}

// convertToDecimal 将值转换为 decimal.Decimal
func convertToDecimal(v any) (decimal.Decimal, error) {
	switch val := v.(type) {
	case decimal.Decimal:
		return val, nil
	case string:
		d, err := decimal.NewFromString(val)
		if err != nil {
			return decimal.Decimal{}, fmt.Errorf("invalid decimal string %q: %w", val, err)
		}
		return d, nil
	case float32:
		return decimal.NewFromFloat32(val), nil
	case float64:
		return decimal.NewFromFloat(val), nil
	case int:
		return decimal.NewFromInt(int64(val)), nil
	case int8:
		return decimal.NewFromInt(int64(val)), nil
	case int16:
		return decimal.NewFromInt(int64(val)), nil
	case int32:
		return decimal.NewFromInt32(val), nil
	case int64:
		return decimal.NewFromInt(val), nil
	case uint:
		return decimal.NewFromInt(int64(val)), nil
	case uint8:
		return decimal.NewFromInt(int64(val)), nil
	case uint16:
		return decimal.NewFromInt(int64(val)), nil
	case uint32:
		return decimal.NewFromInt(int64(val)), nil
	case uint64:
		// uint64 可能超出 int64 范围，使用字符串转换
		return decimal.NewFromString(fmt.Sprintf("%d", val))
	default:
		return decimal.Decimal{}, fmt.Errorf("cannot convert %T to decimal", v)
	}
}

// convertToTime 将值转换为 time.Time
func convertToTime(v any) (time.Time, error) {
	switch val := v.(type) {
	case time.Time:
		return val, nil
	case string:
		// 尝试 RFC3339 格式
		t, err := time.Parse(time.RFC3339, val)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid time string %q: %w", val, err)
		}
		return t, nil
	case int64:
		// Unix 时间戳（秒）
		return time.Unix(val, 0), nil
	default:
		return time.Time{}, fmt.Errorf("cannot convert %T to time", v)
	}
}

// convertToDuration 将值转换为 time.Duration
func convertToDuration(v any) (time.Duration, error) {
	switch val := v.(type) {
	case time.Duration:
		return val, nil
	case int64:
		// 纳秒
		return time.Duration(val), nil
	case string:
		// 字符串格式 (如 "1h30m")
		d, err := time.ParseDuration(val)
		if err != nil {
			return 0, fmt.Errorf("invalid duration string %q: %w", val, err)
		}
		return d, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to duration", v)
	}
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
