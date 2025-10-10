# Nullable 字段支持

## 概述

SRDB 通过**指针类型**来声明 nullable 字段：

- **指针类型** (`*string`, `*int32`, ...) - 自动推断为 nullable
- **Tag 标记** (`nullable`) - 可选，仅用于指针类型（非指针类型会报错）

## 方式 1: 指针类型（推荐）

### 定义

```go
type User struct {
    ID    uint32  `srdb:"field:id"`
    Name  string  `srdb:"field:name"`
    Email *string `srdb:"field:email"`  // 自动推断为 nullable
    Age   *int32  `srdb:"field:age"`    // 自动推断为 nullable
}
```

### 使用

```go
// 插入数据
table.Insert(map[string]any{
    "id":    uint32(1),
    "name":  "Alice",
    "email": "alice@example.com",  // 有值
    "age":   int32(25),
})

table.Insert(map[string]any{
    "id":    uint32(2),
    "name":  "Bob",
    "email": nil,  // NULL
    "age":   nil,
})

// 查询数据
rows, _ := table.Query().Rows()
for rows.Next() {
    data := rows.Row().Data()

    if data["email"] != nil {
        fmt.Println("Email:", data["email"])
    } else {
        fmt.Println("Email: <NULL>")
    }
}
```

**优点**:
- ✓ Go 原生支持
- ✓ nil 天然表示 NULL
- ✓ 最符合 Go 习惯
- ✓ 无需额外依赖
- ✓ StructToFields 自动识别

**使用场景**:
- 大部分 nullable 字段场景
- 新项目

---

## Tag 显式标记（可选）

### 定义

```go
type User struct {
    ID    uint32  `srdb:"field:id"`
    Name  string  `srdb:"field:name"`
    Email *string `srdb:"field:email"`           // 指针类型，自动 nullable
    Phone *string `srdb:"field:phone;nullable"`  // 显式标记（冗余但允许）
}
```

⚠️ **重要**：`nullable` 标记**只能用于指针类型**。非指针类型标记 `nullable` 会报错：

```go
// ✗ 错误：非指针类型不能标记 nullable
type Wrong struct {
    Email string `srdb:"field:email;nullable"`  // 报错！
}

// ✓ 正确：必须是指针类型
type Correct struct {
    Email *string `srdb:"field:email;nullable"`  // ✓ 或省略 nullable
}
```

**为什么要这样设计？**
- 保持类型系统的一致性
- 避免 "string 类型但允许 NULL" 这种混乱的语义
- 强制使用指针类型来表示 nullable，语义更清晰

---

## 完整示例

```go
package main

import (
    "fmt"
    "time"
    "code.tczkiot.com/wlw/srdb"
)

// 用户表（使用指针）
type User struct {
    ID        uint32    `srdb:"field:id"`
    Name      string    `srdb:"field:name"`
    Email     *string   `srdb:"field:email;comment:邮箱（可选）"`
    Phone     *string   `srdb:"field:phone;comment:手机号（可选）"`
    Age       *int32    `srdb:"field:age;comment:年龄（可选）"`
    CreatedAt time.Time `srdb:"field:created_at"`
}

// 商品表（使用指针）
type Product struct {
    ID          uint32    `srdb:"field:id"`
    Name        string    `srdb:"field:name;indexed"`
    Price       *float64  `srdb:"field:price;comment:价格（可选）"`
    Stock       *int32    `srdb:"field:stock;comment:库存（可选）"`
    Description *string   `srdb:"field:description"`
    CreatedAt   time.Time `srdb:"field:created_at"`
}

func main() {
    db, _ := srdb.Open("./data")
    defer db.Close()

    // 创建用户表
    userFields, _ := srdb.StructToFields(User{})
    userSchema, _ := srdb.NewSchema("users", userFields)
    userTable, _ := db.CreateTable("users", userSchema)

    // 插入用户
    userTable.Insert(map[string]any{
        "id":         uint32(1),
        "name":       "Alice",
        "email":      "alice@example.com",
        "phone":      "13800138000",
        "age":        int32(25),
        "created_at": time.Now(),
    })

    userTable.Insert(map[string]any{
        "id":         uint32(2),
        "name":       "Bob",
        "email":      nil,  // NULL
        "phone":      nil,
        "age":        nil,
        "created_at": time.Now(),
    })

    // 查询用户
    rows, _ := userTable.Query().Rows()
    defer rows.Close()

    for rows.Next() {
        data := rows.Row().Data()

        fmt.Printf("%s:", data["name"])

        if data["email"] != nil {
            fmt.Printf(" email=%s", data["email"])
        } else {
            fmt.Print(" email=<NULL>")
        }

        if data["age"] != nil {
            fmt.Printf(", age=%d", data["age"])
        } else {
            fmt.Print(", age=<NULL>")
        }

        fmt.Println()
    }
}
```

---

## 最佳实践

### 1. 使用指针类型

```go
// ✓ 推荐：指针类型，无需 tag
type User struct {
    Email *string
    Phone *string
}

// ✓ 可以：显式标记（冗余但允许）
type User struct {
    Email *string `srdb:"nullable"`
    Phone *string `srdb:"nullable"`
}

// ✗ 错误：非指针类型不能标记 nullable
type User struct {
    Email string `srdb:"nullable"`  // 报错！
    Phone string `srdb:"nullable"`  // 报错！
}
```

### 2. 添加注释说明

```go
type User struct {
    Email *string `srdb:"field:email;comment:邮箱（可选）"`
    Phone *string `srdb:"field:phone;comment:手机号（可选）"`
}
```

### 3. 一致性

在同一个结构体中，尽量使用统一的方式：

```go
// ✓ 好：统一使用指针
type User struct {
    Email *string
    Phone *string
    Age   *int32
}

// ✗ 避免：混用
type User struct {
    Email *string
    Phone string `srdb:"nullable"`
}
```

---

## 当前限制

⚠️ **注意**：当前版本在二进制编码中，NULL 值会被存储为零值。这意味着：

- `0` 和 `NULL` 在 int 类型中无法区分
- `""` 和 `NULL` 在 string 类型中无法区分
- `false` 和 `NULL` 在 bool 类型中无法区分

**未来改进** (v2.1):
我们计划在二进制编码格式中添加 NULL 标志位，完全区分零值和 NULL。

**当前解决方案**:
- 对于整数类型，考虑使用特殊值（如 -1）表示未设置
- 对于字符串，考虑使用非空默认值
- 或等待 v2.1 版本的完整 NULL 支持

---

## FAQ

### Q: 为什么推荐指针类型？

**A**: 指针类型是 Go 语言表示 nullable 的标准方式：
- nil 天然表示 NULL
- 类型系统原生支持
- 无需额外学习成本
- StructToFields 自动识别

### Q: nullable tag 是必需的吗？

**A**: 不是。指针类型会自动推断为 nullable，无需显式标记。

```go
// 这两种写法等价
type User struct {
    Email *string                    // 自动 nullable
    Phone *string `srdb:"nullable"`  // 显式标记（冗余）
}
```

### Q: 非指针类型可以标记 nullable 吗？

**A**: 不可以！非指针类型标记 `nullable` 会报错：

```go
// ✗ 错误
type User struct {
    Email string `srdb:"nullable"`  // 报错！
}

// ✓ 正确
type User struct {
    Email *string  // 或 *string `srdb:"nullable"`
}
```

**原因**：保持类型系统的一致性，避免混乱的语义。

### Q: 插入时可以省略 nullable 字段吗？

**A**: 可以。如果 map 中不包含某个 nullable 字段，会被视为 NULL。

```go
// 这两种写法等价
table.Insert(map[string]any{
    "name":  "Alice",
    "email": nil,
})

table.Insert(map[string]any{
    "name": "Alice",
    // email 字段省略，自动为 NULL
})
```

### Q: NULL 和零值的问题何时解决？

**A**: 计划在 v2.1 版本中添加 NULL 标志位，完全区分 NULL 和零值。

---

## 运行示例

```bash
cd examples/nullable
go run main.go
```

输出：
```
=== Nullable 字段测试（指针类型） ===

【测试 1】用户表（指针类型）
─────────────────────────────
User Schema 字段:
  - id (uint32)
  - name (string)
  - email (string) [nullable] // 邮箱（可选）
  - phone (string) [nullable] // 手机号（可选）
  - age (int32) [nullable] // 年龄（可选）
  - created_at (time)

插入用户数据:
  ✓ Alice (所有字段都有值)
  ✓ Bob (email 和 age 为 NULL)
  ✓ Charlie (所有可选字段都为 NULL)

查询结果:
  - Alice: email=alice@example.com, phone=13800138000, age=25
  - Bob: email=<NULL>, phone=13900139000, age=<NULL>
  - Charlie: email=<NULL>, phone=<NULL>, age=<NULL>
```
