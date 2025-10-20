# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

SRDB 是一个用 Go 编写的高性能 Append-Only 时序数据库引擎。它使用简化的 LSM-tree 架构，结合 WAL + MemTable + mmap B+Tree SST 文件，针对高并发写入（200K+ 写/秒）和快速查询（1-5ms）进行了优化。

**模块**: `github.com/hupeh/srdb`

## 构建和测试

```bash
# 运行所有测试
go test -v ./...

# 运行单个测试
go test -v -run TestSSTable
go test -v -run TestTable

# 运行性能测试
go test -bench=. -benchmem

# 运行带超时的测试（某些 compaction 测试需要较长时间）
go test -v -timeout 30s

# 构建 WebUI 工具
cd examples/webui
go build -o webui main.go
./webui serve --db ./data
```

## 架构

### 文件结构（扁平化设计）

所有核心代码都在根目录下，采用扁平化结构：

```
srdb/
├── database.go          # 多表数据库管理
├── table.go             # 表管理（带 Schema）
├── errors.go            # 错误定义和处理（统一错误码系统）
├── wal.go               # WAL 实现（Write-Ahead Log）
├── memtable.go          # MemTable（map + sorted slice，~130 行）
├── sstable.go           # SSTable 文件（读写器、管理器、二进制编码）
├── btree.go             # B+Tree 索引（构建器、读取器，4KB 节点）
├── version.go           # 版本控制（MANIFEST 管理）
├── compaction.go        # Compaction 压缩合并
├── schema.go            # Schema 定义与验证
├── index.go             # 二级索引管理器
├── index_btree.go       # 索引 B+Tree 实现
└── query.go             # 查询构建器和表达式求值
```

**运行时数据目录**:
```
database_dir/
├── database.meta        # 数据库元数据（JSON）
├── MANIFEST             # 全局版本控制
└── table_name/          # 每表一个目录
    ├── schema.json      # 表 Schema 定义
    ├── MANIFEST         # 表级版本控制
    ├── 000001.wal       # WAL 文件
    ├── 000001.sst       # SST 文件（B+Tree 索引 + 二进制数据）
    └── idx_field.sst    # 二级索引文件（可选）
```

### 核心架构：简化的两层模型

与传统的多层 LSM 树不同，SRDB 使用简化的两层架构：

1. **内存层**: WAL + MemTable (Active + Immutable)
2. **磁盘层**: 带 B+Tree 索引的 SST 文件，分为 L0-L4+ 层级

### 核心数据流

**写入路径**:
1. Schema 验证（强制要求，如果表有 Schema）
2. 生成序列号 (`_seq`，原子递增的 int64）
3. 追加写入 WAL（顺序写）
4. 插入到 Active MemTable（map + 有序 slice）
5. 当 MemTable 超过阈值（默认 64MB）时，切换到新的 Active MemTable 并异步刷新 Immutable 到 SST
6. 更新二级索引（如果字段标记为 Indexed）

**读取路径**:
1. 检查 Active MemTable（O(1) map 查找）
2. 按顺序检查 Immutable MemTables（从最新到最旧）
3. 使用 mmap + B+Tree 索引扫描 SST 文件（从最新到最旧）
4. 第一个匹配的记录获胜（新数据覆盖旧数据）

**查询路径**（带条件）:
1. 如果是带 `=` 操作符的索引字段：使用二级索引 → 通过 seq 获取
2. 否则：带过滤条件的全表扫描（MemTable + SST）

### 关键设计决策

**MemTable: `map[int64][]byte + sorted []int64`**
- 为什么不用 SkipList？实现更简单（~130 行），Put 和 Get 都是 O(1) vs O(log N)
- 权衡：插入新 key 时需要重新排序 keys slice（但实际上仍然更快）
- Active MemTable + 多个 Immutable MemTables（正在刷新中）

**SST 格式: 4KB 节点的 B+Tree**
- 固定大小的节点，与 OS 页面大小对齐
- 支持高效的 mmap 访问和零拷贝读取
- 内部节点：keys + 子节点指针
- 叶子节点：keys + 数据偏移量/大小
- 数据块：二进制编码（使用 Schema 时）或 JSON（无 Schema 时）

**二进制编码格式**:
- Magic Number: `0x524F5731` ("ROW1")
- 格式：`[Magic: 4B][Seq: 8B][Time: 8B][FieldCount: 2B][FieldOffsetTable][FieldData]`
- 按字段分别编码，支持部分字段读取（`GetPartial`）
- 无压缩（优先查询性能，保持 mmap 零拷贝）

**mmap 而非 read() 系统调用**
- 对 SST 文件的零拷贝访问
- OS 自动管理页面缓存
- 应用程序内存占用 < 150MB，无论数据大小

**Append-only（无更新/删除）**
- 简化并发控制
- 相同 seq 的新记录覆盖旧记录
- Compaction 合并文件并按 seq 去重（保留最新的，按时间戳）

## 常见开发模式

### Schema 系统（强制要求）

从最近的重构开始，Schema 是**强制**的，不再支持无 Schema 模式：

```go
schema := NewSchema("users", []Field{
    {Name: "name", Type: String, Indexed: false, Comment: "用户名"},
    {Name: "age", Type: Int64, Indexed: false, Comment: "年龄"},
    {Name: "email", Type: String, Indexed: true, Comment: "邮箱（索引）"},
})

table, _ := db.CreateTable("users", schema)
```

- Schema 在 `Insert()` 时强制验证类型和必填字段
- 索引字段（`Indexed: true`）自动创建二级索引
- Schema 持久化到 `table_dir/schema.json`，包含校验和防篡改
- **支持的类型** (21 种，精确映射到 Go 基础类型):
  - **有符号整数** (5种): `Int`, `Int8`, `Int16`, `Int32`, `Int64`
  - **无符号整数** (5种): `Uint`, `Uint8`, `Uint16`, `Uint32`, `Uint64`
  - **浮点数** (2种): `Float32`, `Float64`
  - **字符串** (1种): `String`
  - **布尔** (1种): `Bool`
  - **特殊类型** (5种): `Byte` (独立类型，底层=uint8), `Rune` (独立类型，底层=int32), `Decimal` (高精度十进制，使用 shopspring/decimal), `Time` (time.Time), `Duration` (time.Duration)
  - **复杂类型** (2种): `Object` (map[string]xxx、struct{}、*struct{}，使用 JSON 编码), `Array` ([]xxx 切片，使用 JSON 编码)
- **Nullable 支持**: 字段可标记为 `Nullable: true`，允许 NULL 值

### 类型系统详解

**精确类型映射**:
从 v1.x 开始，SRDB 采用精确类型映射策略，每个 Go 基础类型都有对应的 FieldType。这带来以下优势：

1. **存储优化**: 使用 `uint8` (1 字节) 存储百分比，而不是 `int64` (8 字节)
2. **语义明确**: `uint32` 表示设备ID，`float32` 表示传感器读数
3. **类型安全**: 编译期和运行期双重类型检查

**类型转换规则**:

```go
// 1. 相同类型：直接接受
{Name: "age", Type: Int32}
Insert(map[string]any{"age": int32(25)})  // ✓

// 2. 兼容类型：自动转换（有符号 ↔ 无符号，需非负）
{Name: "count", Type: Int64}
Insert(map[string]any{"count": uint32(100)})  // ✓

// 3. 类型提升：整数 → 浮点
{Name: "ratio", Type: Float32}
Insert(map[string]any{"ratio": int32(42)})  // ✓ 转为 42.0

// 4. JSON 兼容：float64 → 整数（需为整数值）
{Name: "id", Type: Int64}
Insert(map[string]any{"id": float64(123.0)})  // ✓ JSON 反序列化场景

// 5. 负数 → 无符号：拒绝
{Name: "index", Type: Uint32}
Insert(map[string]any{"index": int32(-1)})  // ✗ 错误
```

**最佳实践**:

```go
// 推荐：根据数据范围选择合适的类型
schema, _ := NewSchema("sensors", []Field{
    {Name: "device_id", Type: Uint32},      // 0 ~ 42亿
    {Name: "temperature", Type: Float32},   // 单精度足够
    {Name: "humidity", Type: Uint8},        // 0-100
    {Name: "status", Type: Bool},           // 布尔状态
})

// 避免：盲目使用 int64 和 float64
schema, _ := NewSchema("sensors_bad", []Field{
    {Name: "device_id", Type: Int64},       // 浪费 4 字节
    {Name: "temperature", Type: Float64},   // 浪费 4 字节
    {Name: "humidity", Type: Int64},        // 浪费 7 字节！
    {Name: "status", Type: Int64},          // 浪费 7 字节！
})
```

**新增类型的使用场景**:

```go
// Byte 类型 - 状态码、标志位
{Name: "status_code", Type: Byte, Comment: "HTTP 状态码 (0-255)"}
Insert(map[string]any{"status_code": uint8(200)})  // byte 和 uint8 底层相同

// Rune 类型 - 单个字符
{Name: "grade", Type: Rune, Comment: "等级 (S/A/B/C)"}
Insert(map[string]any{"grade": rune('A')})  // rune 和 int32 底层相同

// Decimal 类型 - 金融计算
{Name: "amount", Type: Decimal, Comment: "交易金额"}
import "github.com/shopspring/decimal"
Insert(map[string]any{"amount": decimal.NewFromFloat(123.456)})

// Nullable 支持 - 可选字段
{Name: "email", Type: String, Nullable: true, Comment: "邮箱（可选）"}
Insert(map[string]any{"email": nil})  // 允许 NULL
Insert(map[string]any{"email": "user@example.com"})  // 或有值
```

**从结构体自动生成 Schema**:

```go
type Sensor struct {
    DeviceID    uint32  `srdb:"device_id;indexed;comment:设备ID"`
    Temperature float32 `srdb:"temperature;comment:温度"`
    Humidity    uint8   `srdb:"humidity;comment:湿度 0-100"`
    Online      bool    `srdb:"online;comment:是否在线"`
}

// 自动映射：
//   uint32 → Uint32
//   float32 → Float32
//   uint8 → Uint8 (也可用 byte)
//   bool → Bool
fields, _ := StructToFields(Sensor{})
schema, _ := NewSchema("sensors", fields)
```

**参考示例**:
- `examples/all_types/` - 展示所有 17 种类型的基本使用
- `examples/new_types/` - 展示新增的 Byte、Rune、Decimal 类型和 Nullable 支持的实际应用场景

### Query Builder

对于带条件的查询，使用链式 API：

```go
// 简单查询
rows, _ := table.Query().Eq("name", "Alice").Rows()

// 复合条件
rows, _ := table.Query().
    Eq("status", "active").
    Gte("age", 18).
    Rows()

// 字段选择（性能优化）
rows, _ := table.Query().
    Select("id", "name", "email").
    Eq("status", "active").
    Rows()

// 游标模式
rows, _ := table.Query().Rows()
defer rows.Close()
for rows.Next() {
    row := rows.Row()
    fmt.Println(row.Data())
}
```

支持的操作符：`Eq`, `NotEq`, `Lt`, `Gt`, `Lte`, `Gte`, `In`, `NotIn`, `Between`, `Contains`, `StartsWith`, `EndsWith`, `IsNull`, `NotNull`

### Compaction

Compaction 在后台自动运行：

- **触发条件**: L0 文件数 > 阈值（默认 4-10，根据层级）
- **策略**: 合并重叠文件，从 L0 → L1、L1 → L2 等
- **Score 计算**: `size / max_size` 或 `file_count / max_files`
- **安全性**: 删除前验证文件是否存在，以防止数据丢失
- **去重**: 对于重复的 seq，保留最新记录（按时间戳）
- **文件大小**: L0=2MB、L1=10MB、L2=50MB、L3=100MB、L4+=200MB

修改 compaction 逻辑时，注意 `compaction.go` 中的文件选择和合并逻辑。

### 版本控制（MANIFEST）

MANIFEST 跟踪跨版本的 SST 文件元数据：

- `VersionEdit`: 记录原子变更（AddFile/DeleteFile）
- `VersionSet`: 管理当前和历史版本
- `LogAndApply()`: 原子地应用编辑并持久化到 MANIFEST

添加/删除 SST 文件时：
1. 分配文件编号：`versionSet.AllocateFileNumber()`
2. 创建带变更的 `VersionEdit`
3. 应用：`versionSet.LogAndApply(edit)`
4. 清理旧文件（通过 GC 机制）

### 错误处理

使用统一的错误码系统（`errors.go`）：

```go
// 创建错误
err := NewError(ErrCodeTableNotFound, nil)

// 带上下文包装错误
err := WrapError(baseErr, "failed to get table %s", "users")

// 错误判断
if IsNotFound(err) { ... }
if IsCorrupted(err) { ... }
if IsClosed(err) { ... }

// 获取错误码
code := GetErrorCode(err)
```

- 错误码范围：1000-1999（通用）、2000-2999（数据库）、3000-3999（表）、4000-4999（Schema）等
- 所有 panic 已替换为错误返回
- 使用 `fmt.Errorf` 和 `%w` 进行错误链包装

### 错误恢复

- **WAL 重放**: 启动时，所有 `*.wal` 文件被重放到 Active MemTable
- **孤儿文件清理**: 不在 MANIFEST 中的文件在启动时删除（有年龄保护，避免误删最近写入的文件）
- **索引修复**: 自动验证和重建损坏的索引
- **优雅降级**: 表恢复失败会被记录但不会使数据库崩溃

## 重要实现细节

### 序列号系统

- `_seq` 是单调递增的 int64（原子操作）
- 充当主键和时间戳排序
- 永不重用（append-only）
- Compaction 期间，相同 seq 值的较新记录优先（按 `_time` 排序）

### 并发控制

- `Table.mu`: 保护表级元数据
- `SSTableManager.mu`: RWMutex，保护 SST reader 列表
- `MemTable.mu`: RWMutex，支持并发读、独占写
- `VersionSet.mu`: 保护版本状态
- 无全局锁，细粒度锁设计

### 文件格式

**WAL 条目**:
```
CRC32 (4B) | Length (4B) | Type (1B) | Seq (8B) | DataLen (4B) | Data (N bytes)
```

**SST 文件**:
```
Header (256B) | B+Tree Index (4KB nodes) | Data Blocks (Binary format)
```

**B+Tree 节点**（4KB 固定）:
```
Header (32B) | Keys (8B each) | Pointers/Offsets (8B each) | Padding
```

**二进制行格式** (ROW1):
```
Magic (4B) | Seq (8B) | Time (8B) | FieldCount (2B) |
[FieldOffset, FieldSize] × N | FieldData × N
```

## 性能特性

- **写入吞吐量**: 200K+ 写/秒（多线程），50K 写/秒（单线程）
- **写入延迟**: < 1ms（p99）
- **查询延迟**: < 0.1ms（MemTable），1-5ms（SST 热数据），3-5ms（冷数据）
- **内存使用**: < 150MB（64MB MemTable + 开销）
- **压缩**: 未使用（优先查询性能）

优化建议：
- 批量写入以减少 WAL 同步开销
- 对经常查询的字段创建索引
- 使用 `Select()` 只查询需要的字段
- 监控 MemTable 刷新频率（不应太频繁）
- 根据写入模式调整 Compaction 阈值

## 常见陷阱

- **Schema 是强制的**: 所有表必须定义 Schema，不再支持无 Schema 模式
- **索引非自动创建**: 需要在 Schema 中显式标记 `Indexed: true`
- **类型名称简化**:
  - ⚠️ **重要变更**: 从 v2.0 开始，类型名称已简化，使用简短形式（如 `String` 而非 `FieldTypeString`）
  - 每个 Go 类型有对应的简短常量（如 `int32` → `Int32`，`string` → `String`）
  - 插入时类型会自动转换（如 `int` → `int32`），但需要注意负数不能转为无符号类型
- **新增类型的使用**:
  - **Byte**: 虽然底层是 `uint8`，但在 Schema 中作为独立类型，语义更清晰（用于状态码、标志位等）
  - **Rune**: 虽然底层是 `int32`，但在 Schema 中作为独立类型，用于存储单个 Unicode 字符
  - **Decimal**: 必须使用 `github.com/shopspring/decimal` 包，用于金融计算等需要精确数值的场景
- **Nullable 支持**:
  - 需要显式标记 `Nullable: true`，默认字段不允许 NULL
  - NULL 值在 Go 中表示为 `nil`
  - 读取时需要检查值是否存在且不为 nil
- **选择合适的类型大小**:
  - 避免盲目使用 `Int64`/`Float64`，根据数据范围选择（如百分比用 `Uint8`，状态码用 `Byte`）
  - 过大的类型浪费存储和内存，影响性能
- **Compaction 磁盘占用**: 合并期间旧文件和新文件共存，会暂时增加磁盘使用
- **MemTable flush 异步**: 关闭时需要等待 immutable flush 完成
- **mmap 虚拟内存**: 可能显示较大的虚拟内存使用（正常，OS 管理，不是实际 RAM）
- **无 panic**: 所有 panic 已替换为错误返回，需要正确处理错误

## Web UI

项目包含功能完善的 Web 管理界面：

```bash
cd examples/webui
go run main.go serve --db /path/to/database --port 8080
```

功能：
- 表管理和数据浏览
- Manifest 可视化（LSM-Tree 结构）
- 实时 Compaction 监控
- 深色/浅色主题

详见 `examples/webui/README.md`
