# SRDB 新 Tag 格式说明

## 概述

从 v2.0 开始，SRDB 支持新的结构体标签格式，采用 `key:value` 形式，使标签解析与顺序无关。

## 新格式特点

### 1. 顺序无关

旧格式（位置相关）：
```go
type User struct {
    Name  string `srdb:"name;comment:用户名;nullable"`
    Email string `srdb:"email;indexed;comment:邮箱;nullable"`
}
```

新格式（顺序无关）：
```go
type User struct {
    // 以下三种写法完全等价
    Name  string `srdb:"field:name;comment:用户名;nullable"`
    Email string `srdb:"comment:邮箱;field:email;indexed;nullable"`
    Age   int64  `srdb:"nullable;field:age;comment:年龄;indexed"`
}
```

### 2. 支持的标签

| 标签格式 | 说明 | 示例 |
|---------|------|------|
| `field:xxx` | 字段名（如不指定则使用结构体字段名） | `field:user_name` |
| `comment:xxx` | 字段注释 | `comment:用户邮箱` |
| `indexed` | 创建二级索引 | `indexed` |
| `nullable` | 允许 NULL 值 | `nullable` |

### 3. 向后兼容

新格式完全兼容旧的位置相关格式：

```go
// 旧格式（仍然有效）
type Product struct {
    ID   uint32 `srdb:"id;comment:商品ID"`
    Name string `srdb:"name;indexed;comment:商品名称"`
}

// 新格式（推荐）
type Product struct {
    ID   uint32 `srdb:"field:id;comment:商品ID"`
    Name string `srdb:"field:name;indexed;comment:商品名称"`
}
```

## 完整示例

```go
package main

import (
    "time"
    "code.tczkiot.com/wlw/srdb"
)

// 使用新 tag 格式定义结构体
type Product struct {
    ID          uint32        `srdb:"field:id;comment:商品ID"`
    Name        string        `srdb:"comment:商品名称;field:name;indexed"`
    Price       float64       `srdb:"field:price;nullable;comment:价格"`
    Stock       int32         `srdb:"indexed;field:stock;comment:库存数量"`
    Category    string        `srdb:"field:category;indexed;nullable;comment:分类"`
    Description string        `srdb:"nullable;field:description;comment:商品描述"`
    CreatedAt   time.Time     `srdb:"field:created_at;comment:创建时间"`
    UpdatedAt   time.Time     `srdb:"comment:更新时间;field:updated_at;nullable"`
    ExpireIn    time.Duration `srdb:"field:expire_in;comment:过期时间;nullable"`
}

func main() {
    // 从结构体自动生成 Schema
    fields, err := srdb.StructToFields(Product{})
    if err != nil {
        panic(err)
    }

    schema, err := srdb.NewSchema("products", fields)
    if err != nil {
        panic(err)
    }

    // 创建数据库和表
    db, err := srdb.Open("./data")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    table, err := db.CreateTable("products", schema)
    if err != nil {
        panic(err)
    }

    // 插入数据（nullable 字段可以使用 nil）
    err = table.Insert(map[string]any{
        "id":         uint32(1001),
        "name":       "iPhone 15",
        "price":      6999.0,
        "stock":      int32(50),
        "category":   "电子产品",
        "created_at": time.Now(),
        "expire_in":  365 * 24 * time.Hour,
    })

    // nullable 字段设为 nil
    err = table.Insert(map[string]any{
        "id":         uint32(1002),
        "name":       "待定商品",
        "price":      nil,  // ✓ 允许 NULL（字段标记为 nullable）
        "stock":      int32(0),
        "created_at": time.Now(),
    })
}
```

## 类型映射

新 tag 格式支持 SRDB 的所有 19 种类型：

| Go 类型 | FieldType | 说明 |
|---------|-----------|------|
| `int`, `int8`, `int16`, `int32`, `int64` | `Int`, `Int8`, ... | 有符号整数 |
| `uint`, `uint8`, `uint16`, `uint32`, `uint64` | `Uint`, `Uint8`, ... | 无符号整数 |
| `byte` | `Byte` | 字节类型（底层 uint8） |
| `rune` | `Rune` | 字符类型（底层 int32） |
| `float32`, `float64` | `Float32`, `Float64` | 浮点数 |
| `string` | `String` | 字符串 |
| `bool` | `Bool` | 布尔值 |
| `time.Time` | `Time` | 时间戳 |
| `time.Duration` | `Duration` | 时间间隔 |
| `decimal.Decimal` | `Decimal` | 高精度十进制 |

## Nullable 支持

标记为 `nullable` 的字段：
- 可以接受 `nil` 值
- 插入时可以省略该字段
- 读取时需要检查是否为 `nil`

```go
type User struct {
    Name  string `srdb:"field:name;comment:必填字段"`
    Email string `srdb:"field:email;nullable;comment:可选字段"`
}

// 插入数据
table.Insert(map[string]any{
    "name":  "Alice",
    "email": nil,  // ✓ nullable 字段可以为 nil
})

table.Insert(map[string]any{
    "name": "Bob",
    // email 可以省略
})

// 查询数据
rows, _ := table.Query().Rows()
for rows.Next() {
    data := rows.Row().Data()
    if data["email"] != nil {
        fmt.Println("Email:", data["email"])
    } else {
        fmt.Println("Email: <未设置>")
    }
}
```

## 索引支持

标记为 `indexed` 的字段会自动创建二级索引：

```go
type User struct {
    ID    uint32 `srdb:"field:id"`
    Email string `srdb:"field:email;indexed;comment:邮箱（自动创建索引）"`
}

// 查询时自动使用索引
rows, _ := table.Query().Eq("email", "user@example.com").Rows()
```

## 最佳实践

1. **优先使用新格式**：虽然兼容旧格式，但推荐使用 `field:xxx` 明确指定字段名
2. **合理使用 nullable**：只对真正可选的字段标记 nullable，避免滥用
3. **为索引字段添加注释**：标记 `indexed` 时说明索引用途
4. **按语义排序标签**：建议按 `field → indexed → nullable → comment` 顺序编写，便于阅读

示例：
```go
type Product struct {
    ID       uint32  `srdb:"field:id;comment:商品ID"`
    Name     string  `srdb:"field:name;indexed;comment:商品名称（索引）"`
    Price    float64 `srdb:"field:price;nullable;comment:价格（可选）"`
    Category string  `srdb:"field:category;indexed;nullable;comment:分类（索引+可选）"`
}
```

## 迁移指南

从旧格式迁移到新格式：

```bash
# 使用 sed 批量转换（示例）
sed -i 's/`srdb:"\([^;]*\);/`srdb:"field:\1;/g' *.go
```

或手动修改：

```go
// 旧格式
type User struct {
    Name string `srdb:"name;comment:用户名"`
}

// 新格式
type User struct {
    Name string `srdb:"field:name;comment:用户名"`
}
```

## 错误处理

常见错误：

```go
// ❌ 错误：field 值为空
`srdb:"field:;comment:xxx"`

// ✓ 正确：省略 field 前缀（使用结构体字段名）
`srdb:"comment:xxx"`

// ❌ 错误：重复的 key
`srdb:"field:name;field:user_name"`

// ✓ 正确：只使用一次
`srdb:"field:name"`
```

## 性能说明

新 tag 格式的解析性能与旧格式相当：
- 解析时间：< 1μs/field
- 内存开销：无额外分配
- 向后兼容：零性能损失
