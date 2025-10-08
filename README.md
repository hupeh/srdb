# SRDB - Simple Row Database

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

一个基于 LSM-Tree 的高性能嵌入式数据库，专为时序数据和日志存储设计。

## 🎯 特性

### 核心功能
- **LSM-Tree 架构** - 高效的写入性能和空间利用率
- **MVCC 并发控制** - 支持多版本并发读写
- **WAL 持久化** - 写前日志保证数据安全
- **自动 Compaction** - 智能的多层级数据合并策略
- **索引支持** - 快速的字段查询能力
- **Schema 管理** - 灵活的表结构定义

### 查询能力
- **链式查询 API** - 流畅的查询构建器
- **丰富的操作符** - 支持 `=`, `!=`, `<`, `>`, `IN`, `BETWEEN`, `CONTAINS` 等
- **复合条件** - `AND`, `OR`, `NOT` 逻辑组合
- **字段选择** - 按需加载指定字段，优化性能
- **游标模式** - 惰性加载，支持大数据集遍历

### 管理工具
- **Web UI** - 现代化的数据库管理界面
- **命令行工具** - 丰富的诊断和维护工具
- **实时监控** - LSM-Tree 结构和 Compaction 状态可视化

---

## 📋 目录

- [快速开始](#快速开始)
- [基本用法](#基本用法)
- [查询 API](#查询-api)
- [Web UI](#web-ui)
- [架构设计](#架构设计)
- [性能特点](#性能特点)
- [开发指南](#开发指南)

---

## 🚀 快速开始

### 安装

```bash
go get code.tczkiot.com/srdb
```

### 基本示例

```go
package main

import (
    "fmt"
    "log"
    "code.tczkiot.com/srdb"
)

func main() {
    // 1. 打开数据库
    db, err := srdb.Open("./data")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // 2. 定义 Schema
    schema := srdb.NewSchema("users", []srdb.Field{
        {Name: "id", Type: srdb.FieldTypeInt64, Indexed: true, Comment: "用户ID"},
        {Name: "name", Type: srdb.FieldTypeString, Indexed: false, Comment: "用户名"},
        {Name: "email", Type: srdb.FieldTypeString, Indexed: true, Comment: "邮箱"},
        {Name: "age", Type: srdb.FieldTypeInt64, Indexed: false, Comment: "年龄"},
    })

    // 3. 创建表
    table, err := db.CreateTable("users", schema)
    if err != nil {
        log.Fatal(err)
    }

    // 4. 插入数据
    err = table.Insert(map[string]any{
        "id":    1,
        "name":  "Alice",
        "email": "alice@example.com",
        "age":   25,
    })
    if err != nil {
        log.Fatal(err)
    }

    // 5. 查询数据
    rows, err := table.Query().
        Eq("name", "Alice").
        Gte("age", 18).
        Rows()
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()

    // 6. 遍历结果
    for rows.Next() {
        row := rows.Row()
        fmt.Printf("User: %v\n", row.Data())
    }
}
```

---

## 📖 基本用法

### 数据库操作

```go
// 打开数据库
db, err := srdb.Open("./data")

// 列出所有表
tables := db.ListTables()

// 获取表
table, err := db.GetTable("users")

// 删除表
err = db.DropTable("users")

// 关闭数据库
db.Close()
```

### 表操作

```go
// 插入数据
err := table.Insert(map[string]any{
    "name": "Bob",
    "age":  30,
})

// 获取单条数据（通过序列号）
row, err := table.Get(seq)

// 删除数据
err := table.Delete(seq)

// 更新数据
err := table.Update(seq, map[string]any{
    "age": 31,
})
```

### Schema 定义

```go
schema := srdb.NewSchema("logs", []srdb.Field{
    {
        Name:    "group",
        Type:    srdb.FieldTypeString,
        Indexed: true,
        Comment: "日志分组",
    },
    {
        Name:    "message",
        Type:    srdb.FieldTypeString,
        Indexed: false,
        Comment: "日志内容",
    },
    {
        Name:    "timestamp",
        Type:    srdb.FieldTypeInt64,
        Indexed: true,
        Comment: "时间戳",
    },
})
```

**支持的字段类型**：
- `FieldTypeString` - 字符串
- `FieldTypeInt64` - 64位整数
- `FieldTypeBool` - 布尔值
- `FieldTypeFloat64` - 64位浮点数

---

## 🔍 查询 API

### 基本查询

```go
// 等值查询
rows, err := table.Query().Eq("name", "Alice").Rows()

// 范围查询
rows, err := table.Query().
    Gte("age", 18).
    Lt("age", 60).
    Rows()

// IN 查询
rows, err := table.Query().
    In("status", []any{"active", "pending"}).
    Rows()

// BETWEEN 查询
rows, err := table.Query().
    Between("age", 18, 60).
    Rows()
```

### 字符串查询

```go
// 包含
rows, err := table.Query().Contains("message", "error").Rows()

// 前缀匹配
rows, err := table.Query().StartsWith("email", "admin@").Rows()

// 后缀匹配
rows, err := table.Query().EndsWith("filename", ".log").Rows()
```

### 复合条件

```go
// AND 条件
rows, err := table.Query().
    Eq("status", "active").
    Gte("age", 18).
    Rows()

// OR 条件
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
```

### 结果处理

```go
// 游标模式（惰性加载）
rows, err := table.Query().Rows()
defer rows.Close()

for rows.Next() {
    row := rows.Row()
    fmt.Println(row.Data())
}

// 获取第一条
row, err := table.Query().First()

// 获取最后一条
row, err := table.Query().Last()

// 收集所有结果
data := rows.Collect()

// 获取总数
count := rows.Count()

// 扫描到结构体
var users []User
err := rows.Scan(&users)
```

### 完整的操作符列表

| 操作符 | 方法 | 说明 |
|--------|------|------|
| `=` | `Eq(field, value)` | 等于 |
| `!=` | `NotEq(field, value)` | 不等于 |
| `<` | `Lt(field, value)` | 小于 |
| `>` | `Gt(field, value)` | 大于 |
| `<=` | `Lte(field, value)` | 小于等于 |
| `>=` | `Gte(field, value)` | 大于等于 |
| `IN` | `In(field, values)` | 在列表中 |
| `NOT IN` | `NotIn(field, values)` | 不在列表中 |
| `BETWEEN` | `Between(field, min, max)` | 在范围内 |
| `NOT BETWEEN` | `NotBetween(field, min, max)` | 不在范围内 |
| `CONTAINS` | `Contains(field, pattern)` | 包含子串 |
| `NOT CONTAINS` | `NotContains(field, pattern)` | 不包含子串 |
| `STARTS WITH` | `StartsWith(field, prefix)` | 以...开头 |
| `NOT STARTS WITH` | `NotStartsWith(field, prefix)` | 不以...开头 |
| `ENDS WITH` | `EndsWith(field, suffix)` | 以...结尾 |
| `NOT ENDS WITH` | `NotEndsWith(field, suffix)` | 不以...结尾 |
| `IS NULL` | `IsNull(field)` | 为空 |
| `IS NOT NULL` | `NotNull(field)` | 不为空 |

---

## 🌐 Web UI

SRDB 提供了一个功能强大的 Web 管理界面。

### 启动 Web UI

```bash
cd examples/webui

# 基本启动
go run main.go serve

# 自定义配置
go run main.go serve --db /path/to/database --port 3000

# 启用自动数据插入（演示模式）
go run main.go serve --auto-insert
```

访问：http://localhost:8080

### 功能特性

- **表管理** - 查看所有表及其 Schema
- **数据浏览** - 分页浏览表数据，支持列选择
- **Manifest 查看** - 可视化 LSM-Tree 结构
- **实时监控** - Compaction 状态和统计
- **主题切换** - 深色/浅色主题
- **响应式设计** - 完美适配移动设备

详细文档：[examples/webui/README.md](examples/webui/README.md)

---

## 🏗️ 架构设计

### LSM-Tree 结构

```
写入流程：
  数据
   ↓
  WAL（持久化）
   ↓
  MemTable（内存）
   ↓
  Immutable MemTable
   ↓
  Level 0 SST（磁盘）
   ↓
  Level 1-6 SST（Compaction）
```

### 组件架构

```
Database
├── Table
│   ├── Schema（表结构）
│   └── Engine（存储引擎）
│       ├── MemTable Manager
│       │   ├── Active MemTable
│       │   └── Immutable MemTables
│       ├── SSTable Manager
│       │   └── SST Files (Level 0-6)
│       ├── WAL Manager
│       │   └── Write-Ahead Log
│       ├── Version Manager
│       │   └── MVCC Versions
│       └── Compaction Manager
│           ├── Picker（选择策略）
│           └── Worker（执行合并）
└── Query Builder
    └── Expression Engine
```

### 数据流

**写入路径**：
```
Insert → WAL → MemTable → Flush → SST Level 0 → Compaction → SST Level 1-6
```

**读取路径**：
```
Query → MemTable → Immutable MemTables → SST Files (Level 0-6)
```

**Compaction 触发**：
- Level 0：文件数量 ≥ 4
- Level 1-6：总大小超过阈值
- Score 计算：`size / max_size` 或 `file_count / max_files`

---

## ⚡ 性能特点

### 写入性能
- **顺序写入** - WAL 和 MemTable 顺序写入，性能极高
- **批量刷盘** - MemTable 达到阈值后批量刷盘
- **异步 Compaction** - 后台异步执行，不阻塞写入

### 读取性能
- **内存优先** - 优先从 MemTable 读取
- **Bloom Filter** - 快速判断 key 是否存在（TODO）
- **索引加速** - 索引字段快速定位
- **按需加载** - 游标模式惰性加载，节省内存

### 空间优化
- **Snappy 压缩** - SST 文件自动压缩
- **增量合并** - Compaction 只合并必要的文件
- **垃圾回收** - 自动清理过期版本

### 性能指标（参考）

| 操作 | 性能 |
|------|------|
| 顺序写入 | ~100K ops/s |
| 随机写入 | ~50K ops/s |
| 点查询 | ~10K ops/s |
| 范围扫描 | ~1M rows/s |

*注：实际性能取决于硬件配置和数据特征*

---

## 🛠️ 开发指南

### 项目结构

```
srdb/
├── btree.go              # B-Tree 索引实现
├── compaction.go         # Compaction 管理器
├── database.go           # 数据库管理
├── engine.go             # 存储引擎核心
├── index.go              # 索引管理
├── memtable.go           # 内存表
├── query.go              # 查询构建器
├── schema.go             # Schema 定义
├── sstable.go            # SSTable 文件
├── table.go              # 表管理
├── version.go            # 版本管理（MVCC）
├── wal.go                # Write-Ahead Log
├── webui/                # Web UI
│   ├── webui.go          # HTTP 服务器
│   └── static/           # 前端资源
└── examples/             # 示例程序
    └── webui/            # Web UI 工具
```

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定测试
go test -v -run TestEngine

# 性能测试
go test -bench=. -benchmem
```

### 构建示例

```bash
# 构建 WebUI
cd examples/webui
go build -o webui main.go

# 运行
./webui serve --db ./data
```

---

## 📚 文档

- [设计文档](DESIGN.md) - 详细的架构设计和实现原理
- [WebUI 文档](examples/webui/README.md) - Web 管理界面使用指南
- [API 文档](https://pkg.go.dev/code.tczkiot.com/srdb) - Go API 参考

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

### 开发流程

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 提交 Pull Request

### 代码规范

- 遵循 Go 官方代码风格
- 添加必要的注释和文档
- 编写单元测试
- 确保所有测试通过

---

## 📝 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

---

## 🙏 致谢

- [LevelDB](https://github.com/google/leveldb) - LSM-Tree 设计灵感
- [RocksDB](https://github.com/facebook/rocksdb) - Compaction 策略参考
- [Lit](https://lit.dev/) - Web Components 框架

---

## 📧 联系方式

- 项目主页：https://code.tczkiot.com/srdb
- Issue 跟踪：https://code.tczkiot.com/srdb/issues

---

**SRDB** - 简单、高效、可靠的嵌入式数据库 🚀
