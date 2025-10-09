# SRDB 新类型系统示例

本示例展示 SRDB 最新的类型系统特性，包括新增的 **Byte**、**Rune**、**Decimal** 类型以及 **Nullable** 支持。

## 新增特性

### 1. Byte 类型 (FieldTypeByte)
- **用途**: 存储 0-255 范围的小整数
- **适用场景**: HTTP 状态码、标志位、小范围枚举值
- **优势**: 仅占 1 字节，相比 int64 节省 87.5% 空间

### 2. Rune 类型 (FieldTypeRune)
- **用途**: 存储单个 Unicode 字符
- **适用场景**: 等级标识（S/A/B/C）、单字符代码、Unicode 字符
- **优势**: 语义清晰，支持所有 Unicode 字符

### 3. Decimal 类型 (FieldTypeDecimal)
- **用途**: 高精度十进制数值
- **适用场景**: 金融计算、科学计算、需要精确数值的场景
- **优势**: 无精度损失，避免浮点数误差
- **实现**: 使用 `github.com/shopspring/decimal` 库

### 4. Nullable 支持
- **用途**: 允许字段值为 NULL
- **适用场景**: 可选字段、区分"未填写"和"空值"
- **使用**: 在 Field 定义中设置 `Nullable: true`

## 完整类型系统

SRDB 现在支持 **17 种**数据类型：

| 类别 | 类型 | 说明 |
|------|------|------|
| 有符号整数 | int, int8, int16, int32, int64 | 5 种 |
| 无符号整数 | uint, uint8, uint16, uint32, uint64 | 5 种 |
| 浮点 | float32, float64 | 2 种 |
| 字符串 | string | 1 种 |
| 布尔 | bool | 1 种 |
| 特殊类型 | byte, rune, decimal | 3 种 |

## 运行示例

```bash
cd examples/new_types
go run main.go
```

## 示例说明

### 示例 1: Byte 类型（API 日志）
演示使用 `byte` 类型存储 HTTP 状态码，节省存储空间。

```go
{Name: "status_code", Type: srdb.FieldTypeByte, Comment: "HTTP 状态码"}
```

### 示例 2: Rune 类型（用户等级）
演示使用 `rune` 类型存储等级字符（S/A/B/C/D）。

```go
{Name: "level", Type: srdb.FieldTypeRune, Comment: "等级字符"}
```

### 示例 3: Decimal 类型（金融交易）
演示使用 `decimal` 类型进行精确的金融计算。

```go
{Name: "amount", Type: srdb.FieldTypeDecimal, Comment: "交易金额（高精度）"}

// 使用示例
amount := decimal.NewFromFloat(1234.56789012345)
fee := decimal.NewFromFloat(1.23)
total := amount.Add(fee) // 精确加法，无误差
```

### 示例 4: Nullable 支持（用户资料）
演示可选字段的使用，允许某些字段为 NULL。

```go
{Name: "email", Type: srdb.FieldTypeString, Nullable: true, Comment: "邮箱（可选）"}

// 插入数据时可以为 nil
{"username": "Bob", "email": nil} // email 为 NULL
```

### 示例 5: 完整类型系统
演示所有 17 种类型在同一个表中的使用。

## 类型优势对比

| 场景 | 旧方案 | 新方案 | 优势 |
|------|--------|--------|------|
| HTTP 状态码 | int64 (8 字节) | byte (1 字节) | 节省 87.5% 空间 |
| 等级标识 | string ("S") | rune ('S') | 更精确的语义 |
| 金融金额 | float64 (有误差) | decimal (无误差) | 精确计算 |
| 可选字段 | 空字符串 "" | NULL | 区分未填写和空值 |

## 注意事项

1. **Byte 和 Rune 的底层类型**
   - `byte` 在 Go 中等同于 `uint8`
   - `rune` 在 Go 中等同于 `int32`
   - 但在 SRDB Schema 中作为独立类型，语义更清晰

2. **Decimal 的使用**
   - 需要导入 `github.com/shopspring/decimal`
   - 创建方式：`decimal.NewFromFloat()`, `decimal.NewFromString()`, `decimal.NewFromInt()`
   - 运算方法：`Add()`, `Sub()`, `Mul()`, `Div()` 等

3. **Nullable 的使用**
   - NULL 值在 Go 中表示为 `nil`
   - 读取时需要检查值是否存在且不为 nil
   - 非 Nullable 字段不允许为 NULL，会在验证时报错

## 最佳实践

1. **选择合适的类型**
   - 使用最小的整数类型来节省空间（如 uint8 而非 int64）
   - 金融计算必须使用 decimal，避免 float64
   - 可选字段使用 Nullable，而非空字符串或特殊值

2. **性能优化**
   - 小整数类型（byte, int8, uint16）可减少存储和传输开销
   - 索引字段选择合适的类型（如 byte 类型的索引比 string 更高效）

3. **数据完整性**
   - 必填字段设置 `Nullable: false`
   - 使用类型系统保证数据正确性
   - Decimal 类型保证金融数据精确性
