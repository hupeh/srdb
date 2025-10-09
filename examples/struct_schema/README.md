# StructToFields 示例

这个示例展示如何使用 `StructToFields` 方法从 Go 结构体自动生成 Schema。

## 功能特性

- ✅ 从结构体自动生成 Field 列表
- ✅ 支持 struct tags 定义字段属性
- ✅ 支持索引标记
- ✅ 支持字段注释
- ✅ 自动类型映射
- ✅ 支持忽略字段

## Struct Tag 格式

### srdb tag

所有配置都在 `srdb` tag 中，使用分号 `;` 分隔：

```go
type User struct {
    // 基本用法：指定字段名
    Name string `srdb:"name"`

    // 标记为索引字段
    Email string `srdb:"email;indexed"`

    // 完整格式：字段名;索引;注释
    Age int64 `srdb:"age;comment:年龄"`

    // 带索引和注释
    Phone string `srdb:"phone;indexed;comment:手机号"`

    // 忽略该字段
    Internal string `srdb:"-"`

    // 不使用 tag，默认使用 snake_case 转换
    Score float64  // 字段名: score
    UserID string  // 字段名: user_id

}
```

### Tag 格式说明

格式：`srdb:"字段名;选项1;选项2;..."`

- **字段名**（第一部分）：指定数据库中的字段名，省略则自动将结构体字段名转为 snake_case
- **indexed**：标记该字段需要建立索引
- **comment:注释内容**：字段注释说明

### 默认字段名转换（snake_case）

如果不指定字段名，会自动将驼峰命名转换为 snake_case：

- `UserName` → `user_name`
- `EmailAddress` → `email_address`
- `IsActive` → `is_active`
- `HTTPServer` → `http_server`
- `ID` → `id`

## 类型映射

| Go 类型 | FieldType |
|---------|-----------|
| int, int64, int32, int16, int8 | FieldTypeInt64 |
| uint, uint64, uint32, uint16, uint8 | FieldTypeInt64 |
| string | FieldTypeString |
| float64, float32 | FieldTypeFloat |
| bool | FieldTypeBool |

## 完整示例

```go
package main

import (
    "log"
    "code.tczkiot.com/wlw/srdb"
)

// 定义结构体
type User struct {
    Name     string  `srdb:"name;indexed;comment:用户名"`
    Age      int64   `srdb:"age;comment:年龄"`
    Email    string  `srdb:"email;indexed;comment:邮箱"`
    Score    float64 `srdb:"score;comment:分数"`
    IsActive bool    `srdb:"is_active;comment:是否激活"`
}

func main() {
    // 1. 从结构体生成 Field 列表
    fields, err := srdb.StructToFields(User{})
    if err != nil {
        log.Fatal(err)
    }

    // 2. 创建 Schema
    schema := srdb.NewSchema("users", fields)

    // 3. 创建表
    table, err := srdb.OpenTable(&srdb.TableOptions{
        Dir:    "./data/users",
        Name:   schema.Name,
        Fields: schema.Fields,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer table.Close()

    // 4. 插入数据
    err = table.Insert(map[string]any{
        "name":      "张三",
        "age":       int64(25),
        "email":     "zhangsan@example.com",
        "score":     95.5,
        "is_active": true,
    })

    // 5. 查询数据（自动使用索引）
    rows, _ := table.Query().Eq("email", "zhangsan@example.com").Rows()
    defer rows.Close()

    for rows.Next() {
        data := rows.Row().Data()
        // 处理数据...
    }
}
```

## 运行示例

```bash
cd examples/struct_schema
go run main.go
```

## 优势

1. **类型安全**: 使用结构体定义，编译时检查类型
2. **简洁**: 不需要手动创建 Field 列表
3. **可维护**: 结构体和 Schema 在一起，便于维护
4. **灵活**: 支持 tag 自定义字段属性
5. **自动索引**: 通过 `indexed` tag 自动创建索引

## 注意事项

1. 只有导出的字段（首字母大写）会被包含
2. 使用 `srdb:"-"` 可以忽略字段
3. 如果不指定字段名，默认使用小写的字段名
4. 不支持嵌套结构体（需要手动展开）
5. 不支持切片、map 等复杂类型
