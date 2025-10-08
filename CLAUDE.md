# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

SRDB 是一个用 Go 编写的高性能 Append-Only 时序数据库引擎。它使用简化的 LSM-tree 架构，结合 WAL + MemTable + mmap B+Tree SST 文件，针对高并发写入（200K+ 写/秒）和快速查询（1-5ms）进行了优化。

**模块**: `code.tczkiot.com/wlw/srdb`

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
    {Name: "name", Type: FieldTypeString, Indexed: false, Comment: "用户名"},
    {Name: "age", Type: FieldTypeInt64, Indexed: false, Comment: "年龄"},
    {Name: "email", Type: FieldTypeString, Indexed: true, Comment: "邮箱（索引）"},
})

table, _ := db.CreateTable("users", schema)
```

- Schema 在 `Insert()` 时强制验证类型和必填字段
- 索引字段（`Indexed: true`）自动创建二级索引
- Schema 持久化到 `table_dir/schema.json`，包含校验和防篡改
- 支持的类型：`FieldTypeString`, `FieldTypeInt64`, `FieldTypeBool`, `FieldTypeFloat`

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
- **类型严格**: Schema 验证严格，int 和 int64 需要正确匹配
- **Compaction 磁盘占用**: 合并期间旧文件和新文件共存，会暂时增加磁盘使用
- **MemTable flush 异步**: 关闭时需要等待 immutable flush 完成
- **mmap 虚拟内存**: 可能显示较大的虚拟内存使用（正常，OS 管理，不是实际 RAM）
- **无 panic**: 所有 panic 已替换为错误返回，需要正确处理错误
- **废弃代码**: `SSTableCompressionNone` 等常量已删除

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
