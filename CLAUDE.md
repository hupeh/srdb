# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 提供在本仓库中工作的指导。

## 项目概述

SRDB 是一个用 Go 编写的高性能 Append-Only 时序数据库引擎。它使用简化的 LSM-tree 架构，结合 WAL + MemTable + mmap B+Tree SST 文件，针对高并发写入（200K+ 写/秒）和快速查询（1-5ms）进行了优化。

**模块**: `code.tczkiot.com/srdb`

## 构建和测试

```bash
# 运行所有测试
go test -v ./...

# 运行指定包的测试
go test -v ./engine
go test -v ./compaction
go test -v ./query

# 运行指定的测试
go test -v ./engine -run TestEngineBasic

# 构建示例程序
go build ./examples/basic
go build ./examples/with_schema
```

## 架构

### 两层存储模型

与传统的多层 LSM 树不同，SRDB 使用简化的两层架构：

1. **内存层**: WAL + MemTable (Active + Immutable)
2. **磁盘层**: 带 B+Tree 索引的 SST 文件，分为 L0-L4+ 层级

### 核心数据流

**写入路径**:
1. Schema 验证（如果定义了）
2. 生成序列号 (`_seq`)
3. 追加写入 WAL（顺序写）
4. 插入到 Active MemTable（map + 有序 slice）
5. 当 MemTable 超过阈值（默认 64MB）时，切换到新的 Active MemTable 并异步将 Immutable 刷新到 SST
6. 更新二级索引（如果已创建）

**读取路径**:
1. 检查 Active MemTable（O(1) map 查找）
2. 按顺序检查 Immutable MemTables（从最新到最旧）
3. 使用 mmap + B+Tree 索引扫描 SST 文件（从最新到最旧）
4. 第一个匹配的记录获胜（新数据覆盖旧数据）

**查询路径**（带条件）:
1. 如果是带 `=` 操作符的索引字段：使用二级索引 → 通过 seq 获取
2. 否则：带过滤条件的全表扫描（MemTable + SST）

### 关键设计选择

**MemTable: `map[int64][]byte + sorted []int64`**
- 为什么不用 SkipList？实现更简单（130 行），Put 和 Get 都是 O(1) vs O(log N)
- 权衡：插入时需要重新排序 keys slice（但实际上仍然更快）
- Active MemTable + 多个 Immutable MemTables（正在刷新中）

**SST 格式: 4KB 节点的 B+Tree**
- 固定大小的节点，与 OS 页面大小对齐
- 支持高效的 mmap 访问和零拷贝读取
- 内部节点：keys + 子节点指针
- 叶子节点：keys + 数据偏移量/大小
- 数据块：Snappy 压缩的 JSON 行

**mmap 而非 read() 系统调用**
- 对 SST 文件的零拷贝访问
- OS 自动管理页面缓存
- 应用程序内存占用 < 150MB，无论数据大小

**Append-only（无更新/删除）**
- 简化并发控制
- 相同 seq 的新记录覆盖旧记录
- Compaction 合并文件并按 seq 去重（保留最新的，按时间戳）

## 目录结构

```
srdb/
├── database.go           # 多表数据库管理
├── table.go              # 带 schema 的表
├── engine/               # 核心存储引擎（583 行）
│   └── engine.go
├── wal/                  # 预写日志
│   ├── wal.go           # WAL 实现（208 行）
│   └── manager.go       # 多 WAL 管理
├── memtable/            # 内存表
│   ├── memtable.go      # MemTable（130 行）
│   └── manager.go       # Active + Immutable 管理
├── sst/                 # SSTable 文件
│   ├── format.go        # 文件格式定义
│   ├── writer.go        # SST 写入器
│   ├── reader.go        # mmap 读取器（147 行）
│   ├── manager.go       # SST 文件管理
│   └── encoding.go      # Snappy 压缩
├── btree/               # B+Tree 索引
│   ├── node.go          # 4KB 节点结构
│   ├── builder.go       # B+Tree 构建器（125 行）
│   └── reader.go        # B+Tree 读取器
├── manifest/            # 版本控制
│   ├── version_set.go   # 版本管理
│   ├── version_edit.go  # 原子更新
│   ├── version.go       # 文件元数据
│   ├── manifest_writer.go
│   └── manifest_reader.go
├── compaction/          # 后台压缩
│   ├── manager.go       # Compaction 调度器
│   ├── compactor.go     # 合并执行器
│   └── picker.go        # 文件选择策略
├── index/               # 二级索引
│   ├── index.go         # 字段级索引
│   └── manager.go       # 索引生命周期
├── query/               # 查询系统
│   ├── builder.go       # 流式查询 API
│   └── expr.go          # 表达式求值
└── schema/              # Schema 验证
    ├── schema.go        # 类型定义和验证
    └── examples.go      # Schema 示例
```

**运行时数据目录**（例如 `./mydb/`）:
```
database_dir/
├── database.meta        # 数据库元数据（JSON）
├── MANIFEST             # 全局版本控制
└── table_name/          # 每表目录
    ├── schema.json      # 表 schema
    ├── MANIFEST         # 表级版本控制
    ├── wal/             # WAL 文件（*.wal）
    ├── sst/             # SST 文件（*.sst）
    └── index/           # 二级索引（idx_*.sst）
```

## 常见模式

### 使用 Engine

`Engine` 是核心存储层。修改引擎行为时：

- 所有写入都经过 `Insert()` → WAL → MemTable → （异步刷新到 SST）
- 读取经过 `Get(seq)` → 检查 MemTable → 检查 SST 文件
- `switchMemTable()` 创建新的 Active MemTable 并异步刷新旧的
- `flushImmutable()` 将 MemTable 写入 SST 并更新 MANIFEST
- 后台 compaction 通过 `compactionManager` 运行

### Schema 和验证

Schema 是可选的，但建议在生产环境使用：

```go
schema := schema.NewSchema("users").
    AddField("name", schema.FieldTypeString, false, "用户名").
    AddField("age", schema.FieldTypeInt64, false, "用户年龄").
    AddField("email", schema.FieldTypeString, true, "邮箱（索引）")

table, _ := db.CreateTable("users", schema)
```

- Schema 在 `Insert()` 时验证类型和必填字段
- 索引字段（`Indexed: true`）自动创建二级索引
- Schema 持久化到 `table_dir/schema.json`

### Query Builder

对于带条件的查询，始终使用 `QueryBuilder`：

```go
qb := query.NewQueryBuilder()
qb.Where("age", query.OpGreater, 18).
   Where("city", query.OpEqual, "Beijing")
rows, _ := table.Query(qb)
```

- 支持操作符：`OpEqual`、`OpNotEqual`、`OpGreater`、`OpLess`、`OpPrefix`、`OpSuffix`、`OpContains`
- 支持 `WhereNot()` 进行否定
- 支持 `And()` 和 `Or()` 逻辑
- 当可用时自动使用二级索引（对于 `=` 条件）
- 如果没有索引，则回退到全表扫描

### Compaction

Compaction 在后台自动运行：

- **触发条件**: L0 文件数 > 阈值（默认 10）
- **策略**: 合并重叠文件，从 L0 → L1、L1 → L2 等
- **安全性**: 删除前验证文件是否存在，以防止数据丢失
- **去重**: 对于重复的 seq，保留最新记录（按时间戳）
- **文件大小**: L0=2MB、L1=10MB、L2=50MB、L3=100MB、L4+=200MB

修改 compaction 逻辑时：
- `picker.go`: 选择要压缩的文件
- `compactor.go`: 执行合并操作
- `manager.go`: 调度和协调 compaction
- 删除前始终验证输入/输出文件是否存在（参见 `DoCompaction`）

### 版本控制（MANIFEST）

MANIFEST 跟踪跨版本的 SST 文件元数据：

- `VersionEdit`: 记录原子变更（AddFile/DeleteFile）
- `VersionSet`: 管理当前和历史版本
- `LogAndApply()`: 原子地应用编辑并持久化到 MANIFEST

添加/删除 SST 文件时：
1. 分配文件编号：`versionSet.AllocateFileNumber()`
2. 创建带变更的 `VersionEdit`
3. 应用：`versionSet.LogAndApply(edit)`
4. 清理旧文件：`compactionManager.CleanupOrphanFiles()`

### 错误恢复

- **WAL 重放**: 启动时，所有 `*.wal` 文件被重放到 Active MemTable
- **孤儿文件清理**: 不在 MANIFEST 中的文件在启动时删除
- **索引修复**: `verifyAndRepairIndexes()` 重建损坏的索引
- **优雅降级**: 表恢复失败会被记录但不会使数据库崩溃

## 测试模式

测试按组件组织：

- `engine/engine_test.go`: 基本引擎操作
- `engine/engine_compaction_test.go`: Compaction 场景
- `engine/engine_stress_test.go`: 并发压力测试
- `compaction/compaction_test.go`: Compaction 正确性
- `query/builder_test.go`: Query builder 功能
- `schema/schema_test.go`: Schema 验证

为多线程操作编写测试时，使用 `sync.WaitGroup` 并用多个 goroutine 测试（参见 `engine_stress_test.go`）。

## 性能特性

- **写入吞吐量**: 200K+ 写/秒（多线程），50K 写/秒（单线程）
- **写入延迟**: < 1ms（p99）
- **查询延迟**: < 0.1ms（MemTable），1-5ms（SST 热数据），3-5ms（冷数据）
- **内存使用**: < 150MB（64MB MemTable + 开销）
- **压缩率**: Snappy 约 50%

优化时：
- 批量写入以减少 WAL 同步开销
- 对经常查询的字段创建索引
- 监控 MemTable 刷新频率（不应太频繁）
- 根据写入模式调整 compaction 阈值

## 重要实现细节

### 序列号

- `_seq` 是单调递增的 int64（原子操作）
- 充当主键和时间戳排序
- 永不重用（append-only）
- compaction 期间，相同 seq 值的较新记录优先

### 并发

- `Engine.mu`: 保护元数据和 SST reader 列表
- `Engine.flushMu`: 确保一次只有一个 flush
- `MemTable.mu`: RWMutex，支持并发读、独占写
- `VersionSet.mu`: 保护版本状态

### 文件格式

**WAL 条目**:
```
CRC32 (4B) | Length (4B) | Type (1B) | Seq (8B) | DataLen (4B) | Data (N bytes)
```

**SST 文件**:
```
Header (256B) | B+Tree Index | Data Blocks (Snappy compressed)
```

**B+Tree 节点**（4KB 固定）:
```
Header (32B) | Keys (8B each) | Pointers/Offsets (8B each) | Padding
```

## 常见陷阱

- Schema 验证仅在向 `Engine.Open()` 提供 schema 时才应用
- 索引必须通过 `CreateIndex(field)` 显式创建（非自动）
- 带 schema 的 QueryBuilder 需要调用 `WithSchema()` 或让引擎设置它
- Compaction 可能会暂时增加磁盘使用（合并期间旧文件和新文件共存）
- MemTable flush 是异步的；关闭时可能需要等待 immutable flush 完成
- mmap 文件可能显示较大的虚拟内存使用（这是正常的，不是实际 RAM）
