# SRDB - Simple Row Database

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

一个用 Go 编写的高性能 Append-Only 时序数据库引擎，专为高并发写入和快速查询设计。

## 🎯 核心特性

- **Append-Only 架构** - WAL + MemTable + mmap B+Tree SST，简化并发控制
- **强类型 Schema** - 21 种数据类型，包括 Object（map）和 Array（slice）
- **高性能写入** - 200K+ 写/秒（多线程），<1ms 延迟（p99）
- **快速查询** - <0.1ms（内存），1-5ms（磁盘），支持二级索引
- **智能 Scan** - 自动扫描到结构体，完整支持复杂类型
- **链式查询 API** - 18 种操作符，支持复合条件
- **自动 Compaction** - 后台异步合并，优化存储空间
- **零拷贝读取** - mmap 访问 SST 文件，内存占用 <150MB
- **Web 管理界面** - 现代化的数据浏览和监控工具

## 📋 目录

- [快速开始](#快速开始)
- [核心概念](#核心概念)
- [文档](#文档)
- [开发](#开发)

---

## 🚀 快速开始

### 安装

```bash
go get code.tczkiot.com/wlw/srdb
```

**要求**：Go 1.21+

### 基本示例

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

    // 2. 定义 Schema（强类型，21 种类型）
    schema, err := srdb.NewSchema("users", []srdb.Field{
        {Name: "id", Type: srdb.Uint32, Indexed: true, Comment: "用户ID"},
        {Name: "name", Type: srdb.String, Comment: "用户名"},
        {Name: "email", Type: srdb.String, Indexed: true, Comment: "邮箱"},
        {Name: "age", Type: srdb.Int32, Comment: "年龄"},
        {Name: "tags", Type: srdb.Array, Comment: "标签"},          // Array 类型
        {Name: "settings", Type: srdb.Object, Comment: "设置"},     // Object 类型
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
        "tags":  []any{"golang", "database"},
        "settings": map[string]any{
            "theme": "dark",
            "lang":  "zh-CN",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // 5. 查询并扫描到结构体
    type User struct {
        ID       uint32            `json:"id"`
        Name     string            `json:"name"`
        Email    string            `json:"email"`
        Age      int32             `json:"age"`
        Tags     []string          `json:"tags"`
        Settings map[string]string `json:"settings"`
    }

    var users []User
    err = table.Query().
        Eq("name", "Alice").
        Gte("age", 18).
        Scan(&users)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found %d users\n", len(users))
    fmt.Printf("Tags: %v\n", users[0].Tags)
    fmt.Printf("Settings: %v\n", users[0].Settings)
}
```

---

## 💡 核心概念

### 架构

SRDB 使用 **Append-Only 架构**，分为两层：

1. **内存层** - WAL（Write-Ahead Log）+ MemTable（Active + Immutable）
2. **磁盘层** - SST 文件（带 B+Tree 索引），分层存储（L0-L3）

```
写入流程：
数据 → WAL（持久化）→ MemTable → Flush → SST L0 → Compaction → SST L1-L3

读取流程：
查询 → MemTable（O(1)）→ Immutable MemTables → SST Files（B+Tree）
```

### 数据文件

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

---

## 📚 文档

### 核心文档

- [DOCS.md](DOCS.md) - 完整 API 文档和使用指南
- [DESIGN.md](CLAUDE.md) - 数据库设计文档

### 示例教程

- [WebUI 工具](examples/webui/README.md) - Web 管理界面

---

## 🛠️ 开发

### 运行测试

```bash
# 所有测试
go test -v ./...

# 单个测试
go test -v -run TestTable

# 性能测试
go test -bench=. -benchmem
```

### 构建 WebUI

```bash
cd examples/webui
go build -o webui main.go
./webui serve --db ./data
```

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

- [LevelDB](https://github.com/google/leveldb) - 架构设计参考
- [RocksDB](https://github.com/facebook/rocksdb) - Compaction 策略参考

---

## 📧 联系

- 项目主页：https://code.tczkiot.com/wlw/srdb
- Issue 跟踪：https://code.tczkiot.com/wlw/srdb/issues

---

**SRDB** - 简单、高效、可靠的嵌入式数据库 🚀
