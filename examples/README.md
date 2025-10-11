# SRDB Examples

本目录包含 SRDB 数据库的示例程序和工具。

## 目录结构

```
examples/
├── complex/            # 复杂类型系统示例（21 种类型全覆盖）
│   ├── main.go         # 主程序
│   ├── README.md       # 详细文档
│   └── .gitignore      # 忽略数据目录
└── webui/              # Web UI 和命令行工具集
    ├── main.go         # 主入口点
    ├── commands/       # 命令实现
    │   ├── webui.go            # Web UI 服务器
    │   ├── check_data.go       # 数据检查工具
    │   ├── check_seq.go        # 序列号检查工具
    │   ├── dump_manifest.go    # Manifest 导出工具
    │   ├── inspect_all_sst.go  # SST 文件批量检查
    │   ├── inspect_sst.go      # SST 文件检查工具
    │   ├── test_fix.go         # 修复测试工具
    │   └── test_keys.go        # 键存在性测试工具
    └── README.md       # WebUI 详细文档
```

---

## Complex - 完整类型系统演示

一个展示 SRDB 所有 **21 种数据类型**的完整示例，包括结构体 Schema 生成、边界值测试、索引查询和分页等核心功能。

### 🎯 涵盖的类型

| 分类 | 数量 | 包含类型 |
|------|------|----------|
| **字符串** | 1 种 | String |
| **有符号整数** | 5 种 | Int, Int8, Int16, Int32, Int64 |
| **无符号整数** | 5 种 | Uint, Uint8, Uint16, Uint32, Uint64 |
| **浮点数** | 2 种 | Float32, Float64 |
| **布尔** | 1 种 | Bool |
| **特殊类型** | 5 种 | Byte, Rune, Decimal, Time, Duration |
| **复杂类型** | 2 种 | Object, Array |

### 快速开始

```bash
cd examples/complex

# 运行示例
go run main.go

# 清理并重新生成
go run main.go --clean

# 指定数据目录
go run main.go --dir ./mydata --clean
```

### 示例输出

```
╔═══════════════ 设备记录 #1 (seq=1) ═══════════════╗
║ ID: IOT-2025-0001                                   ║
║ 名称: 智能环境监测站                                 ║
╟─────────────────── 整数类型 ────────────────────────╢
║ Signal(int):    -55                                 ║
║ ErrorCode(i8):  0                                   ║
║ DeltaTemp(i16): 150                                 ║
║ RecordNum(i32): 12345                               ║
║ TotalBytes(i64):1073741824                          ║
...
```

### 功能演示

✅ **结构体自动生成 Schema**
```go
fields, _ := srdb.StructToFields(DeviceRecord{})
```

✅ **边界值测试**
- int8 最大值 (127)
- int16 最小值 (-32768)
- uint64 最大值 (18446744073709551615)

✅ **索引查询优化**
```go
table.Query().Eq("device_id", "IOT-2025-0001").Rows()
```

✅ **分页查询（返回总数）**
```go
rows, total, err := table.Query().Paginate(1, 10)
```

✅ **复杂类型序列化**
- Object: map[string]any → JSON
- Array: []string → JSON

详细文档：[complex/README.md](complex/README.md)

---

## WebUI - 数据库管理工具

一个集成了 Web 界面和命令行工具的 SRDB 数据库管理工具。

### 功能特性

#### 🌐 Web UI
- **表列表展示** - 可视化查看所有表及其 Schema
- **数据分页浏览** - 表格形式展示数据，支持分页和列选择
- **Manifest 查看** - 查看 LSM-Tree 结构和 Compaction 状态
- **响应式设计** - 基于 HTMX 的现代化界面
- **大数据优化** - 自动截断显示，点击查看完整内容

#### 🛠️ 命令行工具
- **数据检查** - 检查表和数据完整性
- **序列号验证** - 验证特定序列号的数据
- **Manifest 导出** - 导出 LSM-Tree 层级信息
- **SST 文件检查** - 检查和诊断 SST 文件问题

### 快速开始

#### 1. 启动 Web UI

```bash
cd examples/webui

# 使用默认配置（数据库：./data，端口：8080）
go run main.go serve

# 或指定自定义配置
go run main.go serve -db ./mydb -addr :3000
```

然后打开浏览器访问 `http://localhost:8080`

#### 2. 查看帮助

```bash
go run main.go help
```

输出：
```
SRDB WebUI - Database management tool

Usage:
  webui <command> [flags]

Commands:
  webui, serve       Start WebUI server (default: :8080)
  check-data         Check database tables and row counts
  check-seq          Check specific sequence numbers
  dump-manifest      Dump manifest information
  inspect-all-sst    Inspect all SST files
  inspect-sst        Inspect a specific SST file
  test-fix           Test fix for data retrieval
  test-keys          Test key existence
  help               Show this help message

Examples:
  webui serve -db ./mydb -addr :3000
  webui check-data -db ./mydb
  webui inspect-sst -file ./data/logs/sst/000046.sst
```

---

## 命令详解

### serve / webui - 启动 Web 服务器

启动 Web UI 服务器，提供数据可视化界面。

```bash
# 基本用法
go run main.go serve

# 指定数据库路径和端口
go run main.go webui -db ./mydb -addr :3000
```

**参数**：
- `-db` - 数据库目录路径（默认：`./data`）
- `-addr` - 服务器地址（默认：`:8080`）

**功能**：
- 自动创建示例表（users, products, logs）
- 后台自动插入测试数据（每秒一条）
- 提供 Web UI 和 HTTP API

---

### check-data - 检查数据

检查数据库中所有表的记录数。

```bash
go run main.go check-data -db ./data
```

**输出示例**：
```
Found 3 tables: [users products logs]
Table 'users': 5 rows
Table 'products': 6 rows
Table 'logs': 1234 rows
```

---

### check-seq - 检查序列号

验证特定序列号的数据是否存在。

```bash
go run main.go check-seq -db ./data
```

**功能**：
- 检查 seq=1, 100, 729 等特定序列号
- 显示总记录数
- 验证数据完整性

---

### dump-manifest - 导出 Manifest

导出数据库的 Manifest 信息，检查文件重复。

```bash
go run main.go dump-manifest -db ./data
```

**输出示例**：
```
Level 0: 5 files
Level 1: 3 files
Level 2: 1 files
```

---

### inspect-all-sst - 批量检查 SST 文件

检查所有 SST 文件的完整性。

```bash
go run main.go inspect-all-sst -dir ./data/logs/sst
```

**输出示例**：
```
Found 10 SST files

File #1 (000001.sst):
  Header: MinKey=1 MaxKey=100 RowCount=100
  Actual: 100 keys [1 ... 100]

File #2 (000002.sst):
  Header: MinKey=101 MaxKey=200 RowCount=100
  Actual: 100 keys [101 ... 200]
  *** MISMATCH: Header says 101-200 but file has 105-200 ***
```

---

### inspect-sst - 检查单个 SST 文件

详细检查特定 SST 文件。

```bash
go run main.go inspect-sst -file ./data/logs/sst/000046.sst
```

**输出示例**：
```
File: ./data/logs/sst/000046.sst
Size: 524288 bytes

Header:
  RowCount: 100
  MinKey: 332
  MaxKey: 354
  DataSize: 512000 bytes

Actual keys in file: 100 keys
  First key: 332
  Last key: 354
  All keys: [332 333 334 ... 354]

Trying to get key 332:
  FOUND: seq=332, time=1234567890
```

---

### test-fix - 测试修复

测试数据检索的修复功能。

```bash
go run main.go test-fix -db ./data
```

**功能**：
- 测试首部、中部、尾部记录
- 验证 Get() 操作的正确性
- 显示修复状态

---

### test-keys - 测试键存在性

测试特定键是否存在。

```bash
go run main.go test-keys -db ./data
```

**功能**：
- 测试预定义的键列表
- 统计找到的键数量
- 显示首尾记录

---

## 编译安装

### 编译二进制

```bash
cd examples/webui
go build -o webui main.go
```

### 全局安装

```bash
go install ./examples/webui@latest
```

然后可以在任何地方使用：

```bash
webui serve -db ./mydb
webui check-data -db ./mydb
```

---

## Web UI 使用

### 界面布局

访问 `http://localhost:8080` 后，你会看到：

**左侧边栏**：
- 表列表，显示每个表的字段数
- 点击展开查看 Schema 详情
- 点击表名切换到该表

**右侧主区域**：
- **Data 视图**：数据表格，支持分页和列选择
- **Manifest 视图**：LSM-Tree 结构和 Compaction 状态

### HTTP API 端点

#### 获取表列表
```
GET /api/tables-html
```

#### 获取表数据
```
GET /api/tables-view/{table_name}?page=1&pageSize=20
```

#### 获取 Manifest
```
GET /api/tables-view/{table_name}/manifest
```

#### 获取 Schema
```
GET /api/tables/{table_name}/schema
```

#### 获取单条数据
```
GET /api/tables/{table_name}/data/{seq}
```

详细 API 文档请参考：[webui/README.md](webui/README.md)

---

## 在你的应用中集成

### 方式 1：使用 WebUI 包

```go
package main

import (
    "net/http"
    "code.tczkiot.com/wlw/srdb"
    "code.tczkiot.com/wlw/srdb/webui"
)

func main() {
    db, _ := srdb.Open("./mydb")
    defer db.Close()

    // 创建 WebUI handler
    handler := webui.NewWebUI(db)

    // 启动服务器
    http.ListenAndServe(":8080", handler)
}
```

### 方式 2：挂载到现有应用

```go
mux := http.NewServeMux()

// 你的其他路由
mux.HandleFunc("/api/myapp", myHandler)

// 挂载 SRDB Web UI 到 /admin/db 路径
mux.Handle("/admin/db/", http.StripPrefix("/admin/db", webui.NewWebUI(db)))

http.ListenAndServe(":8080", mux)
```

### 方式 3：使用命令工具

将 webui 工具的命令集成到你的应用：

```go
import "code.tczkiot.com/wlw/srdb/examples/webui/commands"

// 检查数据
commands.CheckData("./mydb")

// 导出 manifest
commands.DumpManifest("./mydb")

// 启动服务器
commands.StartWebUI("./mydb", ":8080")
```

---

## 开发和调试

### 开发模式

在开发时，使用 `go run` 可以快速测试：

```bash
# 启动服务器
go run main.go serve

# 在另一个终端检查数据
go run main.go check-data

# 检查 SST 文件
go run main.go inspect-all-sst
```

### 清理数据

```bash
# 删除数据目录
rm -rf ./data

# 重新运行
go run main.go serve
```

---

## 注意事项

1. **数据目录**：默认在当前目录创建 `./data` 目录
2. **端口占用**：确保端口未被占用
3. **并发访问**：Web UI 支持多用户并发访问
4. **只读模式**：Web UI 仅用于查看，不提供数据修改功能
5. **生产环境**：建议添加身份验证和访问控制
6. **性能考虑**：大表分页查询性能取决于数据分布

---

## 技术栈

- **后端**：Go 标准库（net/http）
- **前端**：HTMX + 原生 JavaScript + CSS
- **渲染**：服务端 HTML 渲染（Go）
- **数据库**：SRDB (LSM-Tree)
- **部署**：所有静态资源通过 embed 嵌入，无需单独部署

---

## 故障排除

### 常见问题

**1. 启动失败 - 端口被占用**
```bash
Error: listen tcp :8080: bind: address already in use
```
解决：使用 `-addr` 指定其他端口
```bash
go run main.go serve -addr :3000
```

**2. 数据库打开失败**
```bash
Error: failed to open database: invalid header
```
解决：删除损坏的数据目录
```bash
rm -rf ./data
```

**3. SST 文件损坏**
使用 `inspect-sst` 或 `inspect-all-sst` 命令诊断：
```bash
go run main.go inspect-all-sst -dir ./data/logs/sst
```

---

## 更多信息

- **WebUI 详细文档**：[webui/README.md](webui/README.md)
- **SRDB 主文档**：[../README.md](../README.md)
- **Compaction 说明**：[../COMPACTION.md](../COMPACTION.md)
- **压力测试报告**：[../STRESS_TEST_RESULTS.md](../STRESS_TEST_RESULTS.md)

---

## 贡献

欢迎贡献新的示例和工具！请遵循以下规范：

1. 在 `examples/` 下创建新的子目录
2. 提供清晰的 README 文档
3. 添加示例代码和使用说明
4. 更新本文件

---

## 许可证

与 SRDB 项目相同的许可证。
