# SRDB 完整文档

## 目录

- [概述](#概述)
- [安装](#安装)
- [快速开始](#快速开始)
- [类型系统](#类型系统)
- [Schema 管理](#schema-管理)
- [数据操作](#数据操作)
- [查询 API](#查询-api)
- [Scan 方法](#scan-方法)
- [Object 和 Array 类型](#object-和-array-类型)
- [索引](#索引)
- [并发控制](#并发控制)
- [性能优化](#性能优化)
- [错误处理](#错误处理)
- [最佳实践](#最佳实践)
- [架构细节](#架构细节)

---

## 概述

SRDB (Simple Row Database) 是一个用 Go 编写的高性能嵌入式数据库，采用 Append-Only 架构（参考 LSM-Tree 设计理念），专为时序数据和高并发写入场景设计。

### 核心特性

- **高性能写入** - 基于 WAL + MemTable，支持 200K+ 写入/秒
- **灵活的 Schema** - 支持 21 种数据类型，包括复杂类型（Object、Array）
- **强大的查询** - 链式 API，支持 18 种操作符和复合条件
- **智能 Scan** - 自动扫描到结构体，完整支持复杂类型
- **自动 Compaction** - 后台智能合并，优化存储空间
- **索引支持** - 二级索引加速查询
- **MVCC** - 多版本并发控制，无锁读

### 适用场景

- 时序数据存储（日志、指标、事件）
- 嵌入式数据库（单机应用）
- 高并发写入场景
- 需要复杂数据类型的场景（JSON 风格数据）

---

## 安装

```bash
go get code.tczkiot.com/wlw/srdb
```

**最低要求**：
- Go 1.21+
- 支持平台：Linux、macOS、Windows

---

## 快速开始

### 基本使用流程

```go
package main

import (
    "fmt"
    "log"
    "code.tczkiot.com/wlw/srdb"
)

func main() {
    // 1. 打开数据库
    db, err := srdb.Open("./data")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // 2. 定义 Schema
    schema, err := srdb.NewSchema("users", []srdb.Field{
        {Name: "id", Type: srdb.Uint32, Indexed: true, Comment: "用户ID"},
        {Name: "name", Type: srdb.String, Comment: "用户名"},
        {Name: "email", Type: srdb.String, Indexed: true, Comment: "邮箱"},
        {Name: "age", Type: srdb.Int32, Comment: "年龄"},
        {Name: "settings", Type: srdb.Object, Comment: "设置（map）"},
        {Name: "tags", Type: srdb.Array, Comment: "标签（slice）"},
    })
    if err != nil {
        log.Fatal(err)
    }

    // 3. 创建表
    table, err := db.CreateTable("users", schema)
    if err != nil {
        log.Fatal(err)
    }

    // 4. 插入数据
    err = table.Insert(map[string]any{
        "id":    uint32(1),
        "name":  "Alice",
        "email": "alice@example.com",
        "age":   int32(25),
        "settings": map[string]any{
            "theme": "dark",
            "lang":  "zh-CN",
        },
        "tags": []any{"golang", "database"},
    })
    if err != nil {
        log.Fatal(err)
    }

    // 5. 查询数据
    var users []User
    err = table.Query().
        Eq("name", "Alice").
        Gte("age", 18).
        Scan(&users)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d users\n", len(users))
}

type User struct {
    ID       uint32            `json:"id"`
    Name     string            `json:"name"`
    Email    string            `json:"email"`
    Age      int32             `json:"age"`
    Settings map[string]string `json:"settings"`
    Tags     []string          `json:"tags"`
}
```

---

## 类型系统

SRDB 支持 **21 种数据类型**，精确映射到 Go 的基础类型。

### 整数类型

#### 有符号整数（5 种）

| 类型 | Go 类型 | 范围 | 存储大小 |
|------|---------|------|----------|
| `Int` | `int` | 平台相关 | 4/8 字节 |
| `Int8` | `int8` | -128 ~ 127 | 1 字节 |
| `Int16` | `int16` | -32,768 ~ 32,767 | 2 字节 |
| `Int32` | `int32` | -2^31 ~ 2^31-1 | 4 字节 |
| `Int64` | `int64` | -2^63 ~ 2^63-1 | 8 字节 |

#### 无符号整数（5 种）

| 类型 | Go 类型 | 范围 | 存储大小 |
|------|---------|------|----------|
| `Uint` | `uint` | 平台相关 | 4/8 字节 |
| `Uint8` | `uint8` | 0 ~ 255 | 1 字节 |
| `Uint16` | `uint16` | 0 ~ 65,535 | 2 字节 |
| `Uint32` | `uint32` | 0 ~ 2^32-1 | 4 字节 |
| `Uint64` | `uint64` | 0 ~ 2^64-1 | 8 字节 |

### 浮点数类型（2 种）

| 类型 | Go 类型 | 精度 | 存储大小 |
|------|---------|------|----------|
| `Float32` | `float32` | 单精度 | 4 字节 |
| `Float64` | `float64` | 双精度 | 8 字节 |

### 基础类型（4 种）

| 类型 | Go 类型 | 说明 | 存储大小 |
|------|---------|------|----------|
| `String` | `string` | UTF-8 字符串 | 变长 |
| `Bool` | `bool` | 布尔值 | 1 字节 |
| `Byte` | `byte` | 字节（uint8 别名） | 1 字节 |
| `Rune` | `rune` | Unicode 字符（int32 别名） | 4 字节 |

### 特殊类型（2 种）

| 类型 | Go 类型 | 说明 | 依赖 |
|------|---------|------|------|
| `Time` | `time.Time` | 时间戳 | 标准库 |
| `Decimal` | `decimal.Decimal` | 高精度十进制 | shopspring/decimal |

### 复杂类型（2 种）

| 类型 | Go 类型 | 说明 | 编码 |
|------|---------|------|------|
| `Object` | `map[string]xxx`, `struct{}`, `*struct{}` | JSON 对象 | JSON |
| `Array` | `[]xxx` | 数组/切片 | JSON |

### 类型选择建议

```go
// ✓ 推荐：根据数据范围选择合适的类型
type Sensor struct {
    DeviceID    uint32  `srdb:"device_id"`      // 0 ~ 42亿
    Temperature float32 `srdb:"temperature"`    // 单精度足够
    Humidity    uint8   `srdb:"humidity"`       // 0-100
    Status      bool    `srdb:"status"`         // 布尔状态
}

// ✗ 避免：盲目使用大类型
type Sensor struct {
    DeviceID    int64   // 浪费 4 字节
    Temperature float64 // 浪费 4 字节
    Humidity    int64   // 浪费 7 字节！
    Status      int64   // 浪费 7 字节！
}
```

### 类型转换规则

SRDB 在插入数据时会进行智能类型转换：

1. **相同类型** - 直接接受
2. **兼容类型** - 自动转换（如 `int` → `int32`）
3. **类型提升** - 整数 → 浮点（如 `int32(42)` → `float64(42.0)`）
4. **JSON 兼容** - `float64` → 整数（需为整数值，用于 JSON 反序列化）
5. **负数检查** - 负数不能转为无符号类型

```go
// 示例：类型转换
schema, _ := srdb.NewSchema("test", []srdb.Field{
    {Name: "count", Type: srdb.Int64},
    {Name: "ratio", Type: srdb.Float32},
})

// ✓ 允许
table.Insert(map[string]any{
    "count": uint32(100),     // uint32 → int64
    "ratio": int32(42),       // int32 → float32 (42.0)
})

// ✗ 拒绝
table.Insert(map[string]any{
    "count": int32(-1),       // 负数不能转为 uint
})
```

---

## Schema 管理

### 创建 Schema

#### 方式 1：手动定义

```go
schema, err := srdb.NewSchema("users", []srdb.Field{
    {
        Name:     "id",
        Type:     srdb.Uint32,
        Indexed:  true,
        Nullable: false,
        Comment:  "用户ID",
    },
    {
        Name:     "name",
        Type:     srdb.String,
        Indexed:  false,
        Nullable: false,
        Comment:  "用户名",
    },
    {
        Name:     "email",
        Type:     srdb.String,
        Indexed:  true,
        Nullable: true,
        Comment:  "邮箱（可选）",
    },
})
```

#### 方式 2：从结构体自动生成

```go
type User struct {
    ID    uint32  `srdb:"field:id;indexed;comment:用户ID"`
    Name  string  `srdb:"field:name;comment:用户名"`
    Email *string `srdb:"field:email;indexed;comment:邮箱（可选）"`
    Age   *int32  `srdb:"field:age;comment:年龄（可选）"`
}

fields, err := srdb.StructToFields(User{})
if err != nil {
    log.Fatal(err)
}

schema, err := srdb.NewSchema("users", fields)
if err != nil {
    log.Fatal(err)
}
```

### Field 结构

```go
type Field struct {
    Name     string      // 字段名（必填）
    Type     FieldType   // 字段类型（必填）
    Indexed  bool        // 是否创建索引
    Nullable bool        // 是否允许 NULL（指针类型自动推断）
    Comment  string      // 字段注释
}
```

### Schema Tag 语法

```go
`srdb:"field:字段名;indexed;nullable;comment:注释"`
```

**支持的选项**：
- `field:name` - 指定字段名（默认使用 snake_case）
- `indexed` - 创建索引
- `nullable` - 允许 NULL（仅用于指针类型）
- `comment:文本` - 字段注释

**示例**：

```go
type User struct {
    // 基本字段
    ID   uint32 `srdb:"field:id;indexed;comment:用户ID"`
    Name string `srdb:"field:name;comment:用户名"`

    // Nullable 字段（使用指针）
    Email *string `srdb:"field:email;indexed;comment:邮箱（可选）"`
    Phone *string `srdb:"field:phone;comment:手机号（可选）"`

    // 复杂类型
    Settings map[string]string `srdb:"field:settings;comment:设置"`
    Tags     []string          `srdb:"field:tags;comment:标签"`

    // 忽略字段
    Internal string `srdb:"-"`
}
```

### Schema 验证

Schema 在创建时会进行严格验证：

1. **字段名唯一性** - 不能重复
2. **类型有效性** - 必须是支持的类型
3. **Nullable 规则** - 只有指针类型可以标记 nullable
4. **保留字段** - 不能使用 `_seq`, `_time` 等保留字段

```go
// ✗ 错误示例
schema, err := srdb.NewSchema("test", []srdb.Field{
    {Name: "id", Type: srdb.String},
    {Name: "id", Type: srdb.Int64},  // 错误：字段名重复
})

// ✗ 错误示例
schema, err := srdb.NewSchema("test", []srdb.Field{
    {Name: "email", Type: srdb.String, Nullable: true},  // 错误：非指针类型不能 nullable
})
```

---

## 数据操作

### 插入数据

```go
// 单条插入
err := table.Insert(map[string]any{
    "id":    uint32(1),
    "name":  "Alice",
    "email": "alice@example.com",
    "age":   int32(25),
})

// 批量插入
users := []map[string]any{
    {"id": uint32(1), "name": "Alice", "age": int32(25)},
    {"id": uint32(2), "name": "Bob", "age": int32(30)},
    {"id": uint32(3), "name": "Charlie", "age": int32(35)},
}

for _, user := range users {
    if err := table.Insert(user); err != nil {
        log.Printf("插入失败: %v", err)
    }
}
```

**注意事项**：
- 插入的数据会立即写入 WAL
- 字段类型会自动验证和转换
- 缺失的 nullable 字段会设为 NULL
- 缺失的非 nullable 字段会报错

### 获取数据

```go
// 通过序列号获取
row, err := table.Get(seq)
if err != nil {
    log.Fatal(err)
}

fmt.Println(row.Seq)   // 序列号
fmt.Println(row.Time)  // 时间戳
fmt.Println(row.Data)  // 数据 (map[string]any)
```

### 更新数据

SRDB 是 **append-only** 架构，更新操作会创建新版本：

```go
// 更新数据
err := table.Update(seq, map[string]any{
    "age": int32(26),
})

// 等价于：
newData := existingData
newData["age"] = int32(26)
table.Insert(newData)
```

### 删除数据

```go
// 标记删除（软删除）
err := table.Delete(seq)

// 物理删除在 Compaction 时进行
```

---

## 查询 API

SRDB 提供流畅的链式查询 API。

### 基本查询

```go
// 等值查询
rows, err := table.Query().Eq("name", "Alice").Rows()

// 不等于
rows, err := table.Query().NotEq("status", "deleted").Rows()

// 大于/小于
rows, err := table.Query().
    Gt("age", 18).
    Lt("age", 60).
    Rows()

// 大于等于/小于等于
rows, err := table.Query().
    Gte("score", 60).
    Lte("score", 100).
    Rows()
```

### 集合查询

```go
// IN
rows, err := table.Query().
    In("status", []any{"active", "pending", "processing"}).
    Rows()

// NOT IN
rows, err := table.Query().
    NotIn("role", []any{"banned", "suspended"}).
    Rows()

// BETWEEN
rows, err := table.Query().
    Between("age", 18, 60).
    Rows()

// NOT BETWEEN
rows, err := table.Query().
    NotBetween("price", 1000, 5000).
    Rows()
```

### 字符串查询

```go
// 包含子串
rows, err := table.Query().Contains("message", "error").Rows()

// 不包含
rows, err := table.Query().NotContains("message", "debug").Rows()

// 前缀匹配
rows, err := table.Query().StartsWith("email", "admin@").Rows()

// 后缀匹配
rows, err := table.Query().EndsWith("filename", ".log").Rows()
```

### NULL 查询

```go
// IS NULL
rows, err := table.Query().IsNull("email").Rows()

// IS NOT NULL
rows, err := table.Query().NotNull("phone").Rows()
```

### 复合条件

```go
// AND（默认）
rows, err := table.Query().
    Eq("status", "active").
    Gte("age", 18).
    NotNull("email").
    Rows()

// OR
rows, err := table.Query().
    Where(srdb.Or(
        srdb.Eq("role", "admin"),
        srdb.Eq("role", "moderator"),
    )).
    Rows()

// 复杂组合
rows, err := table.Query().
    Where(srdb.And(
        srdb.Eq("status", "active"),
        srdb.Or(
            srdb.Gte("age", 18),
            srdb.Eq("verified", true),
        ),
        srdb.Not(srdb.Eq("role", "banned")),
    )).
    Rows()
```

### 字段选择

```go
// 只查询指定字段（性能优化）
rows, err := table.Query().
    Select("id", "name", "email").
    Eq("status", "active").
    Rows()

// 遍历结果
for rows.Next() {
    row := rows.Row()
    data := row.Data()  // 只包含 id, name, email
    fmt.Println(data)
}
```

### 结果获取

```go
// 游标模式（惰性加载，推荐）
rows, err := table.Query().Rows()
defer rows.Close()

for rows.Next() {
    row := rows.Row()
    fmt.Println(row.Data())
}

// 检查错误
if err := rows.Err(); err != nil {
    log.Fatal(err)
}

// 获取第一条
row, err := table.Query().First()

// 获取最后一条
row, err := table.Query().Last()

// 收集所有结果（内存消耗大）
data := rows.Collect()

// 获取总数
count := rows.Count()
```

### 操作符完整列表

| 方法 | 操作符 | 说明 | 示例 |
|------|--------|------|------|
| `Eq(field, value)` | `=` | 等于 | `.Eq("status", "active")` |
| `NotEq(field, value)` | `!=` | 不等于 | `.NotEq("role", "guest")` |
| `Lt(field, value)` | `<` | 小于 | `.Lt("age", 18)` |
| `Gt(field, value)` | `>` | 大于 | `.Gt("score", 60)` |
| `Lte(field, value)` | `<=` | 小于等于 | `.Lte("price", 100)` |
| `Gte(field, value)` | `>=` | 大于等于 | `.Gte("count", 10)` |
| `In(field, values)` | `IN` | 在列表中 | `.In("status", []any{"a", "b"})` |
| `NotIn(field, values)` | `NOT IN` | 不在列表中 | `.NotIn("role", []any{"banned"})` |
| `Between(field, min, max)` | `BETWEEN` | 在范围内 | `.Between("age", 18, 60)` |
| `NotBetween(field, min, max)` | `NOT BETWEEN` | 不在范围内 | `.NotBetween("price", 0, 10)` |
| `Contains(field, pattern)` | `CONTAINS` | 包含子串 | `.Contains("message", "error")` |
| `NotContains(field, pattern)` | `NOT CONTAINS` | 不包含 | `.NotContains("log", "debug")` |
| `StartsWith(field, prefix)` | `STARTS WITH` | 以...开头 | `.StartsWith("email", "admin")` |
| `NotStartsWith(field, prefix)` | `NOT STARTS WITH` | 不以...开头 | `.NotStartsWith("name", "test")` |
| `EndsWith(field, suffix)` | `ENDS WITH` | 以...结尾 | `.EndsWith("file", ".log")` |
| `NotEndsWith(field, suffix)` | `NOT ENDS WITH` | 不以...结尾 | `.NotEndsWith("path", ".tmp")` |
| `IsNull(field)` | `IS NULL` | 为空 | `.IsNull("email")` |
| `NotNull(field)` | `IS NOT NULL` | 不为空 | `.NotNull("phone")` |

---

## Scan 方法

SRDB 提供智能的 Scan 方法，可以将查询结果直接扫描到 Go 结构体。

### Row.Scan() - 扫描单行

```go
row, err := table.Query().Eq("id", 1).First()
if err != nil {
    log.Fatal(err)
}

var user User
err = row.Scan(&user)
if err != nil {
    log.Fatal(err)
}

fmt.Println(user.Name)  // "Alice"
```

### Rows.Scan() - 智能扫描

**Rows.Scan 会自动判断目标类型**：
- 如果目标是**切片** → 扫描所有行
- 如果目标是**结构体** → 只扫描第一行

```go
// 扫描多行到切片
rows, _ := table.Query().Rows()
defer rows.Close()

var users []User
err := rows.Scan(&users)

// 扫描单行到结构体（智能判断）
rows2, _ := table.Query().Eq("id", 1).Rows()
defer rows2.Close()

var user User
err := rows2.Scan(&user)  // 自动只扫描第一行
```

### QueryBuilder.Scan() - 最简洁的方式

```go
// 扫描多行
var users []User
err := table.Query().Scan(&users)

// 扫描单行
var user User
err := table.Query().Eq("id", 1).Scan(&user)

// 带条件扫描
var activeUsers []User
err := table.Query().
    Eq("status", "active").
    Gte("age", 18).
    Scan(&activeUsers)
```

### 部分字段扫描

```go
// 定义简化的结构体
type UserBrief struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

// 只扫描指定字段
var briefs []UserBrief
err := table.Query().
    Select("name", "email").
    Scan(&briefs)

// 结果只包含 name 和 email 字段
```

### 复杂类型扫描

```go
type User struct {
    Name     string            `json:"name"`
    Email    string            `json:"email"`
    Settings map[string]string `json:"settings"`  // Object
    Tags     []string          `json:"tags"`      // Array
    Metadata map[string]any    `json:"metadata"`  // Object with any
    Scores   []int             `json:"scores"`    // Array of int
}

var user User
err := table.Query().Eq("name", "Alice").Scan(&user)

// 访问复杂类型
fmt.Println(user.Settings["theme"])      // "dark"
fmt.Println(user.Tags[0])                // "golang"
fmt.Println(user.Metadata["version"])   // "1.0"
fmt.Println(user.Scores[0])             // 95
```

### Scan 的工作原理

1. **Row.Scan**：
   - 使用 `json.Marshal` 将 row.Data() 转为 JSON
   - 使用 `json.Unmarshal` 解码到目标结构体
   - 应用字段过滤（如果调用了 Select）

2. **Rows.Scan**：
   - 使用 `reflect` 检查目标类型
   - 如果是切片：调用 Collect() 获取所有行，然后 JSON 转换
   - 如果是结构体：调用 First() 获取第一行，然后调用 Row.Scan

3. **QueryBuilder.Scan**：
   - 直接调用 Rows.Scan

---

## Object 和 Array 类型

SRDB 原生支持复杂的数据类型，可以存储 JSON 风格的对象和数组。

### Object 类型

Object 类型可以存储：
- `map[string]string`
- `map[string]any`
- `struct{}`
- `*struct{}`

#### 定义 Object 字段

```go
type User struct {
    Settings map[string]string `srdb:"field:settings"`
    Metadata map[string]any    `srdb:"field:metadata"`
}

// 或手动定义
schema, _ := srdb.NewSchema("users", []srdb.Field{
    {Name: "settings", Type: srdb.Object, Comment: "用户设置"},
    {Name: "metadata", Type: srdb.Object, Comment: "元数据"},
})
```

#### 插入 Object 数据

```go
err := table.Insert(map[string]any{
    "name": "Alice",
    "settings": map[string]any{
        "theme":    "dark",
        "language": "zh-CN",
        "fontSize": "14px",
    },
    "metadata": map[string]any{
        "version": "1.0",
        "author":  "Alice",
        "tags":    []string{"admin", "verified"},  // 嵌套数组
    },
})
```

#### 查询和使用 Object

```go
var user User
table.Query().Eq("name", "Alice").Scan(&user)

// 访问 Object 字段
theme := user.Settings["theme"]               // "dark"
version := user.Metadata["version"]           // "1.0"

// 类型断言（for map[string]any）
if tags, ok := user.Metadata["tags"].([]any); ok {
    fmt.Println(tags[0])  // "admin"
}
```

### Array 类型

Array 类型可以存储任意切片：
- `[]string`
- `[]int`
- `[]any`
- `[]struct{}`

#### 定义 Array 字段

```go
type User struct {
    Tags    []string `srdb:"field:tags"`
    Scores  []int    `srdb:"field:scores"`
    Items   []any    `srdb:"field:items"`
}

// 或手动定义
schema, _ := srdb.NewSchema("users", []srdb.Field{
    {Name: "tags", Type: srdb.Array, Comment: "标签"},
    {Name: "scores", Type: srdb.Array, Comment: "分数"},
})
```

#### 插入 Array 数据

```go
err := table.Insert(map[string]any{
    "name":   "Alice",
    "tags":   []any{"golang", "database", "lsm-tree"},
    "scores": []any{95, 88, 92},
    "items":  []any{
        "item1",
        123,
        true,
        map[string]any{"nested": "value"},  // 嵌套对象
    },
})
```

#### 查询和使用 Array

```go
var user User
table.Query().Eq("name", "Alice").Scan(&user)

// 访问 Array 字段
fmt.Println(len(user.Tags))      // 3
fmt.Println(user.Tags[0])        // "golang"
fmt.Println(user.Scores[1])      // 88

// 遍历
for _, tag := range user.Tags {
    fmt.Println(tag)
}

// 计算平均分
total := 0
for _, score := range user.Scores {
    total += score
}
avg := float64(total) / float64(len(user.Scores))
```

### 嵌套结构

Object 和 Array 可以任意嵌套：

```go
type Config struct {
    Server   string          `json:"server"`
    Port     int             `json:"port"`
    Features map[string]bool `json:"features"`  // 嵌套 Object
}

type Application struct {
    Name    string            `json:"name"`
    Config  Config            `json:"config"`     // 嵌套结构体
    Servers []string          `json:"servers"`    // Array
    Tags    []string          `json:"tags"`       // Array
    Meta    map[string]any    `json:"meta"`       // Object
}

// 插入嵌套数据
table.Insert(map[string]any{
    "name": "MyApp",
    "config": map[string]any{
        "server": "localhost",
        "port":   8080,
        "features": map[string]any{
            "cache":   true,
            "logging": false,
        },
    },
    "servers": []any{"server1", "server2", "server3"},
    "tags":    []any{"production", "v1.0"},
    "meta": map[string]any{
        "deployedAt": time.Now().Format(time.RFC3339),
        "region":     "us-west",
        "replicas":   3,
    },
})

// 查询和访问
var app Application
table.Query().Eq("name", "MyApp").Scan(&app)

fmt.Println(app.Config.Server)              // "localhost"
fmt.Println(app.Config.Features["cache"])   // true
fmt.Println(app.Servers[0])                 // "server1"
fmt.Println(app.Meta["region"])             // "us-west"
```

### 空值处理

```go
// 插入空 Object 和 Array
table.Insert(map[string]any{
    "name":     "Charlie",
    "settings": map[string]any{},  // 空 Object
    "tags":     []any{},            // 空 Array
})

// 查询
var user User
table.Query().Eq("name", "Charlie").Scan(&user)

// 安全检查
if len(user.Settings) == 0 {
    fmt.Println("设置为空")
}

if len(user.Tags) == 0 {
    fmt.Println("没有标签")
}
```

### 存储格式

- **编码方式**：JSON
- **存储格式**：`[length: uint32][JSON data]`
- **零值**：Object 为 `{}`，Array 为 `[]`
- **性能**：JSON 编码/解码有一定开销，但保证了灵活性

---

## 索引

SRDB 支持二级索引，可以显著加速查询性能。

### 创建索引

```go
// 在 Schema 中标记索引
schema, _ := srdb.NewSchema("users", []srdb.Field{
    {Name: "id", Type: srdb.Uint32, Indexed: true},     // 创建索引
    {Name: "email", Type: srdb.String, Indexed: true},  // 创建索引
    {Name: "name", Type: srdb.String, Indexed: false},  // 不创建索引
})
```

### 索引的工作原理

1. **自动创建**：创建表时，所有标记为 `Indexed: true` 的字段会自动创建索引
2. **自动更新**：插入/更新数据时，索引会自动更新
3. **查询优化**：使用 `Eq()` 查询索引字段时，会自动使用索引

```go
// 使用索引（快速）
rows, _ := table.Query().Eq("email", "alice@example.com").Rows()

// 不使用索引（全表扫描）
rows, _ := table.Query().Contains("name", "Alice").Rows()
```

### 索引适用场景

**适合创建索引**：
- ✅ 经常用于等值查询的字段（`Eq`）
- ✅ 高基数字段（unique 或接近 unique）
- ✅ 查询频繁的字段

**不适合创建索引**：
- ❌ 低基数字段（如性别、状态等）
- ❌ 很少查询的字段
- ❌ 频繁更新的字段
- ❌ Object 和 Array 类型字段

### 索引性能

| 操作 | 无索引 | 有索引 | 提升 |
|------|--------|--------|------|
| 等值查询 (Eq) | O(N) | O(log N) | ~1000x |
| 范围查询 (Gt/Lt) | O(N) | O(N) | 无提升 |
| 模糊查询 (Contains) | O(N) | O(N) | 无提升 |

---

## 并发控制

SRDB 使用 **MVCC (多版本并发控制)** 实现无锁并发读写：

- **写入**：追加到 WAL 和 MemTable，使用互斥锁保护
- **读取**：无锁读取，读取的是快照版本
- **Compaction**：后台异步执行，不阻塞读写

```go
// 多个 goroutine 并发写入
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        table.Insert(map[string]any{
            "id":   uint32(id),
            "name": fmt.Sprintf("user_%d", id),
        })
    }(i)
}
wg.Wait()

// 多个 goroutine 并发读取
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        rows, _ := table.Query().Rows()
        defer rows.Close()
        for rows.Next() {
            _ = rows.Row()
        }
    }()
}
wg.Wait()
```

---

## 性能优化

### 写入优化

**1. 批量写入**

```go
// ✓ 好：批量写入，减少 fsync 次数
for i := 0; i < 1000; i++ {
    table.Insert(data[i])
}

// ✗ 避免：每次都打开关闭数据库
for i := 0; i < 1000; i++ {
    db, _ := srdb.Open("./data")
    table.Insert(data[i])
    db.Close()
}
```

**2. 调整 MemTable 大小**

```go
// 默认 64MB，可以根据内存调整
// 更大的 MemTable = 更少的 flush，但占用更多内存
```

### 查询优化

**1. 使用索引**

```go
// ✓ 好：使用索引字段查询
rows, _ := table.Query().Eq("email", "alice@example.com").Rows()

// ✗ 避免：全表扫描
rows, _ := table.Query().Contains("name", "Alice").Rows()
```

**2. 字段选择**

```go
// ✓ 好：只查询需要的字段
rows, _ := table.Query().Select("id", "name").Rows()

// ✗ 避免：查询所有字段
rows, _ := table.Query().Rows()
```

**3. 使用游标模式**

```go
// ✓ 好：惰性加载，节省内存
rows, _ := table.Query().Rows()
defer rows.Close()
for rows.Next() {
    process(rows.Row())
}

// ✗ 避免：一次性加载所有数据
data := rows.Collect()  // 内存消耗大
```

### 存储优化

**1. 定期 Compaction**

Compaction 会自动触发，但可以手动触发：

```go
// 手动触发 Compaction（阻塞）
err := table.Compact()
```

**2. 选择合适的类型**

```go
// ✓ 好：根据数据范围选择类型
type Sensor struct {
    DeviceID uint32  // 0 ~ 42亿，4字节
    Value    float32 // 单精度，4字节
}

// ✗ 避免：使用过大的类型
type Sensor struct {
    DeviceID int64   // 8字节，浪费4字节
    Value    float64 // 8字节，浪费4字节
}
```

### 内存优化

**1. 及时关闭游标**

```go
// ✓ 好：使用 defer 确保关闭
rows, _ := table.Query().Rows()
defer rows.Close()

// ✗ 避免：忘记关闭
rows, _ := table.Query().Rows()
// ... 使用 rows
// 忘记调用 rows.Close()
```

**2. 避免大量缓存**

```go
// ✗ 避免：缓存大量数据
var cache []map[string]any
rows, _ := table.Query().Rows()
cache = rows.Collect()  // 内存消耗大

// ✓ 好：流式处理
rows, _ := table.Query().Rows()
defer rows.Close()
for rows.Next() {
    process(rows.Row())  // 逐条处理
}
```

---

## 错误处理

SRDB 使用统一的错误码系统。

### 错误类型

```go
// 创建错误
err := srdb.NewError(srdb.ErrCodeTableNotFound, nil)

// 包装错误
err := srdb.WrapError(baseErr, "failed to insert: %v", data)

// 判断错误类型
if srdb.IsNotFound(err) {
    // 处理未找到错误
}

if srdb.IsCorrupted(err) {
    // 处理数据损坏错误
}
```

### 常见错误码

| 错误码 | 说明 | 处理方式 |
|--------|------|----------|
| `ErrCodeNotFound` | 数据不存在 | 检查 key 是否正确 |
| `ErrCodeTableNotFound` | 表不存在 | 先创建表 |
| `ErrCodeSchemaValidation` | Schema 验证失败 | 检查字段定义 |
| `ErrCodeTypeConversion` | 类型转换失败 | 检查数据类型 |
| `ErrCodeCorrupted` | 数据损坏 | 恢复备份或重建 |
| `ErrCodeClosed` | 数据库已关闭 | 重新打开数据库 |

### 错误处理最佳实践

```go
// ✓ 好：检查并处理错误
if err := table.Insert(data); err != nil {
    if srdb.IsSchemaValidation(err) {
        log.Printf("数据验证失败: %v", err)
        return
    }
    log.Printf("插入失败: %v", err)
    return
}

// ✗ 避免：忽略错误
table.Insert(data)  // 错误未处理
```

---

## 最佳实践

### Schema 设计

1. **选择合适的类型**
   ```go
   // ✓ 根据数据范围选择
   DeviceID uint32  // 0 ~ 42亿
   Count    uint8   // 0 ~ 255
   ```

2. **合理使用索引**
   ```go
   // ✓ 高基数、频繁查询的字段
   Email string `srdb:"indexed"`

   // ✗ 低基数字段不需要索引
   Gender string  // 只有 2-3 个值
   ```

3. **Nullable 字段使用指针**
   ```go
   Email *string `srdb:"field:email"`
   Phone *string `srdb:"field:phone"`
   ```

### 数据插入

1. **批量插入**
   ```go
   for _, data := range batch {
       table.Insert(data)
   }
   ```

2. **验证数据**
   ```go
   if email == "" {
       return errors.New("email required")
   }
   table.Insert(data)
   ```

### 查询优化

1. **使用索引字段**
   ```go
   // ✓ 使用索引
   table.Query().Eq("email", "alice@example.com")

   // ✗ 避免全表扫描
   table.Query().Contains("email", "@example.com")
   ```

2. **字段选择**
   ```go
   table.Query().Select("id", "name").Rows()
   ```

3. **使用 Scan**
   ```go
   var users []User
   table.Query().Scan(&users)
   ```

### 并发访问

1. **读写分离**
   ```go
   // 多个 goroutine 可以安全并发读
   go func() {
       table.Query().Rows()
   }()
   ```

2. **写入控制**
   ```go
   // 写入使用队列控制并发
   ```

---

## 架构细节

### Append-Only 架构

SRDB 采用 Append-Only 架构（参考 LSM-Tree 设计理念），分为两层：

1. **内存层** - WAL + MemTable (Active + Immutable)
2. **磁盘层** - 带 B+Tree 索引的 SST 文件，分层存储（L0-L3）

```
写入流程：
数据 → WAL（持久化）→ MemTable → Flush → SST L0 → Compaction → SST L1-L3

读取流程：
查询 → MemTable（O(1)）→ Immutable MemTables → SST Files（B+Tree）
```

### 文件组织

```
database_dir/
├── database.meta        # 数据库元数据
└── table_name/          # 每表一个目录
    ├── schema.json      # 表 Schema 定义
    ├── MANIFEST-000001  # 表级版本控制
    ├── CURRENT          # 当前 MANIFEST 指针
    ├── wal/             # WAL 子目录
    │   ├── 000001.wal   # WAL 文件
    │   └── CURRENT      # 当前 WAL 指针
    ├── sst/             # SST 子目录（L0-L3 层级文件）
    │   └── 000001.sst   # SST 文件（B+Tree + 数据）
    └── idx/             # 索引子目录
        └── idx_email.sst # 二级索引文件
```

### 设计特点

- **Append-Only** - 无原地更新，简化并发控制
- **MemTable** - `map[int64][]byte + sorted slice`，O(1) 读写
- **SST 文件** - 4KB 节点的 B+Tree，mmap 零拷贝访问
- **二进制编码** - ROW1 格式，无压缩，优先查询性能
- **Compaction** - 后台异步合并，按层级管理文件大小

### Compaction 策略

- **Level 0-3**: 文件数量或总大小超过阈值时触发
- **Score 计算**: `size / max_size` 或 `file_count / max_files`
- **文件大小**: L0=2MB, L1=10MB, L2=50MB, L3=100MB

### 性能指标

| 操作 | 性能 |
|------|------|
| 顺序写入 | ~100K ops/s |
| 随机写入 | ~50K ops/s |
| 点查询 | ~10K ops/s |
| 范围扫描 | ~1M rows/s |
| 内存使用 | < 150MB (64MB MemTable + overhead) |

---

## 附录

### 参考链接

- [GitHub 仓库](https://code.tczkiot.com/wlw/srdb)
- [API 文档](https://pkg.go.dev/code.tczkiot.com/wlw/srdb)
- [设计文档](DESIGN.md)
- [开发者指南](CLAUDE.md)

### 示例项目

- [所有类型示例](examples/all_types/)
- [Scan 方法示例](examples/scan_demo/)
- [Nullable 示例](examples/nullable/)
- [Web UI](examples/webui/)

### 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

---

**SRDB** - 简单、高效、可靠的嵌入式数据库 🚀
