# 批量插入示例

这个示例展示了 SRDB 的批量插入功能，支持多种数据类型的插入。

## 功能特性

SRDB 的 `Insert` 方法支持以下输入类型：

1. **单个 map**: `map[string]any`
2. **map 切片**: `[]map[string]any`
3. **单个结构体**: `struct{}`
4. **结构体指针**: `*struct{}`
5. **结构体切片**: `[]struct{}`
6. **结构体指针切片**: `[]*struct{}`

## 运行示例

```bash
cd examples/batch_insert
go run main.go
```

## 示例说明

### 示例 1: 插入单个 map

```go
err = table.Insert(map[string]any{
    "name": "Alice",
    "age":  int64(25),
})
```

最基本的插入方式，适合动态数据。

### 示例 2: 批量插入 map 切片

```go
err = table.Insert([]map[string]any{
    {"name": "Alice", "age": int64(25), "email": "alice@example.com"},
    {"name": "Bob", "age": int64(30), "email": "bob@example.com"},
    {"name": "Charlie", "age": int64(35), "email": "charlie@example.com"},
})
```

批量插入多条数据，提高插入效率。

### 示例 3: 插入单个结构体

```go
type User struct {
    Name     string `srdb:"name;comment:用户名"`
    Age      int64  `srdb:"age;comment:年龄"`
    Email    string `srdb:"email;indexed;comment:邮箱"`
    IsActive bool   `srdb:"is_active;comment:是否激活"`
}

user := User{
    Name:     "Alice",
    Age:      25,
    Email:    "alice@example.com",
    IsActive: true,
}

err = table.Insert(user)
```

使用结构体插入，提供类型安全和代码可读性。

### 示例 4: 批量插入结构体切片

```go
users := []User{
    {Name: "Alice", Age: 25, Email: "alice@example.com", IsActive: true},
    {Name: "Bob", Age: 30, Email: "bob@example.com", IsActive: true},
    {Name: "Charlie", Age: 35, Email: "charlie@example.com", IsActive: false},
}

err = table.Insert(users)
```

批量插入结构体，适合需要插入大量数据的场景。

### 示例 5: 批量插入结构体指针切片

```go
users := []*User{
    {Name: "Alice", Age: 25, Email: "alice@example.com", IsActive: true},
    {Name: "Bob", Age: 30, Email: "bob@example.com", IsActive: true},
    nil, // nil 指针会被自动跳过
    {Name: "Charlie", Age: 35, Email: "charlie@example.com", IsActive: false},
}

err = table.Insert(users)
```

支持指针切片，nil 指针会被自动跳过。

### 示例 6: 使用 snake_case 自动转换

```go
type Product struct {
    ProductID   string  `srdb:";comment:产品ID"`  // 自动转为 product_id
    ProductName string  `srdb:";comment:产品名称"`  // 自动转为 product_name
    Price       float64 `srdb:";comment:价格"`    // 自动转为 price
    InStock     bool    `srdb:";comment:是否有货"` // 自动转为 in_stock
}

products := []Product{
    {ProductID: "P001", ProductName: "Laptop", Price: 999.99, InStock: true},
    {ProductID: "P002", ProductName: "Mouse", Price: 29.99, InStock: true},
}

err = table.Insert(products)
```

不指定字段名时，会自动将驼峰命名转换为 snake_case：

- `ProductID` → `product_id`
- `ProductName` → `product_name`
- `InStock` → `in_stock`

## Struct Tag 格式

```go
type User struct {
    // 完整格式：字段名;索引;注释
    Email string `srdb:"email;indexed;comment:邮箱地址"`

    // 使用默认字段名（snake_case）+ 注释
    UserName string `srdb:";comment:用户名"` // 自动转为 user_name

    // 不使用 tag，完全依赖 snake_case 转换
    PhoneNumber string // 自动转为 phone_number

    // 忽略字段
    Internal string `srdb:"-"`
}
```

## 性能优化

批量插入相比逐条插入：

- ✅ 减少函数调用开销
- ✅ 统一类型转换和验证
- ✅ 更清晰的代码逻辑
- ✅ 适合大批量数据导入

## 注意事项

1. **类型匹配**: 确保结构体字段类型与 Schema 定义一致
2. **Schema 验证**: 所有数据都会经过 Schema 验证
3. **nil 处理**: 结构体指针切片中的 nil 会被自动跳过
4. **字段名转换**: 未指定 tag 时自动使用 snake_case 转换
5. **索引更新**: 带索引的字段会自动更新索引

## 相关文档

- [STRUCT_TAG_GUIDE.md](../../STRUCT_TAG_GUIDE.md) - Struct Tag 完整指南
- [SNAKE_CASE_CONVERSION.md](../../SNAKE_CASE_CONVERSION.md) - snake_case 转换规则
- [examples/struct_schema](../struct_schema) - 结构体 Schema 示例
