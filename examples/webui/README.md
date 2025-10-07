# SRDB Web UI Example

这个示例展示了如何使用 SRDB 的内置 Web UI 来可视化查看数据库中的表和数据。

## 功能特性

- 📊 **表列表展示** - 左侧显示所有表及其行数
- 🔍 **Schema 查看** - 点击箭头展开查看表的字段定义
- 📋 **数据分页浏览** - 右侧以表格形式展示数据，支持分页
- 🎨 **响应式设计** - 现代化的界面设计
- ⚡ **零构建** - 使用 HTMX 从 CDN 加载，无需构建步骤
- 💾 **大数据优化** - 自动截断显示，悬停查看，点击弹窗查看完整内容
- 📏 **数据大小显示** - 超过 1KB 的单元格自动显示大小标签
- 🔄 **后台数据插入** - 自动生成 2KB~512KB 的测试数据（每秒一条）

## 运行示例

```bash
# 进入示例目录
cd examples/webui

# 运行
go run main.go
```

程序会：
1. 创建/打开数据库目录 `./data`
2. 创建三个示例表：`users`、`products` 和 `logs`
3. 插入初始示例数据
4. **启动后台协程** - 每秒向 `logs` 表插入一条 2KB~512KB 的随机数据
5. 启动 Web 服务器在 `http://localhost:8080`

## 使用界面

打开浏览器访问 `http://localhost:8080`，你将看到：

### 左侧边栏
- 显示所有表的列表
- 显示每个表的字段数量
- 点击 ▶ 图标展开查看字段信息
- 点击表名选择要查看的表（蓝色高亮显示当前选中）

### 右侧主区域
- **Schema 区域**：显示表结构和字段定义
- **Data 区域**：以表格形式显示数据
  - 支持分页浏览（每页 20 条）
  - 显示系统字段（_seq, _time）和用户字段
  - **自动截断长数据**：超过 400px 的内容显示省略号
  - **鼠标悬停**：悬停在单元格上查看完整内容
  - **点击查看**：点击单元格在弹窗中查看完整内容
  - **大小指示**：超过 1KB 的数据显示大小标签

### 大数据查看
1. **表格截断**：单元格最大宽度 400px，超长显示 `...`
2. **悬停展开**：鼠标悬停自动展开，黄色背景高亮
3. **模态框**：点击单元格弹出窗口
   - 等宽字体显示（适合查看十六进制数据）
   - 显示数据大小
   - 支持滚动查看超长内容

## API 端点

Web UI 提供了以下 HTTP API：

### 获取所有表
```
GET /api/tables
```

返回示例：
```json
[
  {
    "name": "users",
    "rowCount": 5,
    "dir": "./data/users"
  }
]
```

### 获取表的 Schema
```
GET /api/tables/{name}/schema
```

返回示例：
```json
{
  "fields": [
    {"name": "name", "type": "string", "required": true},
    {"name": "email", "type": "string", "required": true},
    {"name": "age", "type": "int", "required": false}
  ]
}
```

### 获取表数据（分页）
```
GET /api/tables/{name}/data?page=1&pageSize=20
```

参数：
- `page` - 页码，从 1 开始（默认：1）
- `pageSize` - 每页行数，最大 100（默认：20）

返回示例：
```json
{
  "page": 1,
  "pageSize": 20,
  "totalRows": 5,
  "totalPages": 1,
  "rows": [
    {
      "_seq": 1,
      "_time": 1234567890,
      "name": "Alice",
      "email": "alice@example.com",
      "age": 30
    }
  ]
}
```

### 获取表基本信息
```
GET /api/tables/{name}
```

## 在你的应用中使用

你可以在自己的应用中轻松集成 Web UI：

```go
package main

import (
    "net/http"
    "code.tczkiot.com/srdb"
)

func main() {
    // 打开数据库
    db, _ := database.Open("./mydb")
    defer db.Close()

    // 获取 HTTP Handler
    handler := db.WebUI()

    // 启动服务器
    http.ListenAndServe(":8080", handler)
}
```

或者将其作为现有 Web 应用的一部分：

```go
mux := http.NewServeMux()

// 你的其他路由
mux.HandleFunc("/api/myapp", myHandler)

// 挂载 SRDB Web UI 到 /admin/db 路径
mux.Handle("/admin/db/", http.StripPrefix("/admin/db", db.WebUI()))

http.ListenAndServe(":8080", mux)
```

## 技术栈

- **后端**: Go + 标准库 `net/http`
- **前端**: [HTMX](https://htmx.org/) + 原生 JavaScript + CSS
- **渲染**: 服务端 HTML 渲染（Go 模板生成）
- **字体**: Google Fonts (Inter)
- **无构建**: 直接从 CDN 加载 HTMX，无需 npm、webpack 等工具
- **部署**: 所有静态资源通过 `embed.FS` 嵌入到二进制文件中

## 测试大数据

### logs 表自动生成

程序会在后台持续向 `logs` 表插入大数据：

- **频率**：每秒一条
- **大小**：2KB ~ 512KB 随机
- **格式**：十六进制字符串
- **字段**：
  - `timestamp` - 插入时间
  - `data` - 随机数据（十六进制）
  - `size_bytes` - 数据大小（字节）

你可以选择 `logs` 表来测试大数据的显示效果：
1. 单元格会显示数据大小标签（如 `245.12 KB`）
2. 内容被自动截断，显示省略号
3. 点击单元格在弹窗中查看完整数据

终端会实时输出插入日志：
```
Inserted record #1, size: 245.12 KB
Inserted record #2, size: 128.50 KB
Inserted record #3, size: 487.23 KB
```

## 注意事项

- Web UI 是只读的，不提供数据修改功能
- 适合用于开发、调试和数据查看
- 生产环境建议添加身份验证和访问控制
- 大数据量表的分页查询性能取决于数据分布
- `logs` 表会持续增长，可手动删除 `./data/logs` 目录重置

## Compaction 状态

由于后台持续插入大数据，会产生大量 SST 文件。SRDB 会自动运行 compaction 合并这些文件。

### 检查 Compaction 状态

```bash
# 查看 SST 文件分布
./check_sst.sh

# 观察 webui 日志中的 [Compaction] 信息
```

### Compaction 改进

- **触发阈值**: L0 文件数量 ≥2 就触发（之前是 4）
- **运行频率**: 每 10 秒自动检查
- **日志增强**: 显示详细的 compaction 状态和统计

详细说明请查看 [COMPACTION.md](./COMPACTION.md)

## 常见问题

### `invalid header` 错误

如果看到类似错误：
```
failed to open table logs: invalid header
```

**快速修复**：
```bash
./fix_corrupted_table.sh logs
```

详见：[QUICK_FIX.md](./QUICK_FIX.md) 或 [TROUBLESHOOTING.md](./TROUBLESHOOTING.md)

## 更多信息

- [FEATURES.md](./FEATURES.md) - 详细功能说明
- [COMPACTION.md](./COMPACTION.md) - Compaction 机制和诊断
- [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) - 故障排除指南
- [QUICK_FIX.md](./QUICK_FIX.md) - 快速修复常见错误
