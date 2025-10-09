# Struct Tags 示例

本示例展示如何使用 Go struct tags 来定义 SRDB Schema，包括完整的 nullable 支持。

## 功能特性

### Struct Tag 格式

SRDB 支持以下 struct tag 格式：

```go
type User struct {
    // 基本格式
    Name string `srdb:"name"`

    // 指定索引
    Email string `srdb:"email;indexed"`

    // 标记为可空
    Bio string `srdb:"bio;nullable"`

    // 可空 + 索引
    Phone string `srdb:"phone;nullable;indexed"`

    // 完整格式
    Age int64 `srdb:"age;indexed;nullable;comment:用户年龄"`

    // 忽略字段
    TempData string `srdb:"-"`
}
```

### Tag 说明

- **字段名**: 第一部分指定数据库字段名（可选，默认自动转换为 snake_case）
- **indexed**: 标记该字段需要建立索引
- **nullable**: 标记该字段允许 NULL 值
- **comment**: 指定字段注释
- **-**: 忽略该字段（不包含在 Schema 中）

## 运行示例

```bash
cd examples/struct_tags
go run main.go
```

## 示例输出

```
=== SRDB Struct Tags Example ===

1. 从结构体生成 Schema
Schema 名称: users
字段数量: 8

字段详情:
  - username: Type=string, Indexed=true, Nullable=false, Comment="用户名（索引）"
  - age: Type=int64, Indexed=false, Nullable=false, Comment="年龄"
  - email: Type=string, Indexed=false, Nullable=true, Comment="邮箱（可选）"
  - phone_number: Type=string, Indexed=true, Nullable=true, Comment="手机号（可空且索引）"
  - bio: Type=string, Indexed=false, Nullable=true, Comment="个人简介（可选）"
  - avatar: Type=string, Indexed=false, Nullable=true, Comment="头像 URL（可选）"
  - balance: Type=decimal, Indexed=false, Nullable=true, Comment="账户余额（可空）"
  - is_active: Type=bool, Indexed=false, Nullable=false, Comment="是否激活"

2. 创建数据库和表
✓ 表创建成功

3. 插入完整数据
✓ 插入用户 alice（完整数据）

4. 插入部分数据（可选字段为 NULL）
✓ 插入用户 bob（email、bio、balance 为 NULL）

5. 测试必填字段不能为 NULL
✓ 符合预期的错误: field username: NULL value not allowed (field is not nullable)

6. 查询所有用户
  用户: alice, 邮箱: alice@example.com, 余额: 1000.5
  用户: bob, 邮箱: <NULL>, 余额: <NULL>

7. 按索引字段查询（username='alice'）
  找到用户: alice, 年龄: 25

✅ 所有操作完成!
```

## 自动字段名转换

如果不指定字段名，会自动将结构体字段名转换为 snake_case：

```go
type User struct {
    UserName      string  // -> user_name
    EmailAddress  string  // -> email_address
    IsActive      bool    // -> is_active
    HTTPServer    string  // -> http_server
}
```

## Nullable 支持说明

1. **必填字段**（默认）：不能插入 NULL 值，会返回错误
2. **可选字段**（nullable=true）：可以插入 NULL 值
3. **查询结果**：NULL 值会以 `nil` 形式返回
4. **验证时机**：在 `Insert()` 时自动验证

## 最佳实践

1. **对可选字段使用 nullable**
   ```go
   Email    string `srdb:"email;nullable"`        // ✓ 推荐
   Email    string `srdb:"email"`                 // ✗ 如果是可选的，应该标记 nullable
   ```

2. **对查询频繁的字段建立索引**
   ```go
   Username string `srdb:"username;indexed"`      // ✓ 查询键
   Bio      string `srdb:"bio"`                   // ✓ 不常查询的字段
   ```

3. **组合使用 nullable 和 indexed**
   ```go
   Phone string `srdb:"phone;nullable;indexed"`  // ✓ 可选但需要索引查询
   ```

4. **为可选字段标记 nullable**
   ```go
   Avatar   string `srdb:"avatar;nullable"`      // ✓ 值类型 + nullable
   Balance  decimal.Decimal `srdb:"balance;nullable"` // ✓ 所有类型都支持 nullable
   ```
