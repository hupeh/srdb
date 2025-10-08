package srdb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultMemTableSize = 64 * 1024 * 1024 // 64 MB
)

// Engine 存储引擎
type Engine struct {
	dir               string
	schema            *Schema
	indexManager      *IndexManager
	walManager        *WALManager        // WAL 管理器
	sstManager        *SSTableManager    // SST 管理器
	memtableManager   *MemTableManager   // MemTable 管理器
	versionSet        *VersionSet        // MANIFEST 管理器
	compactionManager *CompactionManager // Compaction 管理器
	seq               atomic.Int64
	mu                sync.RWMutex
	flushMu           sync.Mutex
}

// EngineOptions 配置选项
type EngineOptions struct {
	Dir          string
	MemTableSize int64
	Schema       *Schema // 可选的 Schema 定义
}

// OpenEngine 打开数据库
func OpenEngine(opts *EngineOptions) (*Engine, error) {
	if opts.MemTableSize == 0 {
		opts.MemTableSize = DefaultMemTableSize
	}

	// 创建主目录
	err := os.MkdirAll(opts.Dir, 0755)
	if err != nil {
		return nil, err
	}

	// 创建子目录
	walDir := filepath.Join(opts.Dir, "wal")
	sstDir := filepath.Join(opts.Dir, "sst")
	idxDir := filepath.Join(opts.Dir, "index")

	err = os.MkdirAll(walDir, 0755)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(sstDir, 0755)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(idxDir, 0755)
	if err != nil {
		return nil, err
	}

	// 尝试从磁盘恢复 Schema（如果 Options 中没有提供）
	var sch *Schema
	if opts.Schema != nil {
		// 使用提供的 Schema
		sch = opts.Schema
		// 保存到磁盘（带校验和）
		schemaPath := filepath.Join(opts.Dir, "schema.json")
		schemaFile, err := NewSchemaFile(sch)
		if err != nil {
			return nil, fmt.Errorf("create schema file: %w", err)
		}
		schemaData, err := json.MarshalIndent(schemaFile, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal schema: %w", err)
		}
		err = os.WriteFile(schemaPath, schemaData, 0644)
		if err != nil {
			return nil, fmt.Errorf("write schema: %w", err)
		}
	} else {
		// 尝试从磁盘恢复
		schemaPath := filepath.Join(opts.Dir, "schema.json")
		schemaData, err := os.ReadFile(schemaPath)
		if err == nil {
			// 文件存在，尝试解析
			schemaFile := &SchemaFile{}
			err = json.Unmarshal(schemaData, schemaFile)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal schema from %s: %w", schemaPath, err)
			}

			// 验证校验和
			err = schemaFile.Verify()
			if err != nil {
				return nil, fmt.Errorf("failed to verify schema from %s: %w", schemaPath, err)
			}

			sch = schemaFile.Schema
		} else if !os.IsNotExist(err) {
			// 其他读取错误
			return nil, fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
		}
		// 如果文件不存在，sch 保持为 nil（可选）
	}

	// 创建索引管理器
	var indexMgr *IndexManager
	if sch != nil {
		indexMgr = NewIndexManager(idxDir, sch)
	}

	// 创建 SST Manager
	sstMgr, err := NewSSTableManager(sstDir)
	if err != nil {
		return nil, err
	}

	// 设置 Schema（用于优化编解码）
	if sch != nil {
		sstMgr.SetSchema(sch)
	}

	// 创建 MemTable Manager
	memMgr := NewMemTableManager(opts.MemTableSize)

	// 创建/恢复 MANIFEST
	manifestDir := opts.Dir
	versionSet, err := NewVersionSet(manifestDir)
	if err != nil {
		return nil, fmt.Errorf("create version set: %w", err)
	}

	// 创建 Engine（暂时不设置 WAL Manager）
	engine := &Engine{
		dir:             opts.Dir,
		schema:          sch,
		indexManager:    indexMgr,
		walManager:      nil, // 先不设置，恢复后再创建
		sstManager:      sstMgr,
		memtableManager: memMgr,
		versionSet:      versionSet,
	}

	// 先恢复数据（包括从 WAL 恢复）
	err = engine.recover()
	if err != nil {
		return nil, err
	}

	// 恢复完成后，创建 WAL Manager 用于后续写入
	walMgr, err := NewWALManager(walDir)
	if err != nil {
		return nil, err
	}
	engine.walManager = walMgr
	engine.memtableManager.SetActiveWAL(walMgr.GetCurrentNumber())

	// 创建 Compaction Manager
	engine.compactionManager = NewCompactionManager(sstDir, versionSet, sstMgr)

	// 设置 Schema（如果有）
	if sch != nil {
		engine.compactionManager.SetSchema(sch)
	}

	// 启动时清理孤儿文件（崩溃恢复后的清理）
	engine.compactionManager.CleanupOrphanFiles()

	// 启动后台 Compaction 和垃圾回收
	engine.compactionManager.Start()

	// 验证并修复索引
	if engine.indexManager != nil {
		engine.verifyAndRepairIndexes()
	}

	return engine, nil
}

// Insert 插入数据
func (e *Engine) Insert(data map[string]any) error {
	// 1. 验证 Schema (如果定义了)
	if e.schema != nil {
		if err := e.schema.Validate(data); err != nil {
			return fmt.Errorf("schema validation failed: %v", err)
		}
	}

	// 2. 生成 _seq
	seq := e.seq.Add(1)

	// 2. 添加系统字段
	row := &SSTableRow{
		Seq:  seq,
		Time: time.Now().UnixNano(),
		Data: data,
	}

	// 3. 序列化
	rowData, err := json.Marshal(row)
	if err != nil {
		return err
	}

	// 4. 写入 WAL
	entry := &WALEntry{
		Type: WALEntryTypePut,
		Seq:  seq,
		Data: rowData,
	}
	err = e.walManager.Append(entry)
	if err != nil {
		return err
	}

	// 5. 写入 MemTable Manager
	e.memtableManager.Put(seq, rowData)

	// 6. 添加到索引
	if e.indexManager != nil {
		e.indexManager.AddToIndexes(data, seq)
	}

	// 7. 检查是否需要切换 MemTable
	if e.memtableManager.ShouldSwitch() {
		go e.switchMemTable()
	}

	return nil
}

// Get 查询数据
func (e *Engine) Get(seq int64) (*SSTableRow, error) {
	// 1. 先查 MemTable Manager (Active + Immutables)
	data, found := e.memtableManager.Get(seq)
	if found {
		var row SSTableRow
		err := json.Unmarshal(data, &row)
		if err != nil {
			return nil, err
		}
		return &row, nil
	}

	// 2. 查询 SST 文件
	return e.sstManager.Get(seq)
}

// GetPartial 按需查询数据（只读取指定字段）
func (e *Engine) GetPartial(seq int64, fields []string) (*SSTableRow, error) {
	// 1. 先查 MemTable Manager (Active + Immutables)
	data, found := e.memtableManager.Get(seq)
	if found {
		var row SSTableRow
		err := json.Unmarshal(data, &row)
		if err != nil {
			return nil, err
		}

		// MemTable 中的数据已经完全解析，需要手动过滤字段
		if len(fields) > 0 {
			filteredData := make(map[string]any)
			for _, field := range fields {
				if val, ok := row.Data[field]; ok {
					filteredData[field] = val
				}
			}
			row.Data = filteredData
		}

		return &row, nil
	}

	// 2. 查询 SST 文件（按需解码）
	return e.sstManager.GetPartial(seq, fields)
}

// switchMemTable 切换 MemTable
func (e *Engine) switchMemTable() error {
	e.flushMu.Lock()
	defer e.flushMu.Unlock()

	// 1. 切换到新的 WAL
	oldWALNumber, err := e.walManager.Rotate()
	if err != nil {
		return err
	}
	newWALNumber := e.walManager.GetCurrentNumber()

	// 2. 切换 MemTable (Active → Immutable)
	_, immutable := e.memtableManager.Switch(newWALNumber)

	// 3. 异步 Flush Immutable
	go e.flushImmutable(immutable, oldWALNumber)

	return nil
}

// flushImmutable 将 Immutable MemTable 刷新到 SST
func (e *Engine) flushImmutable(imm *ImmutableMemTable, walNumber int64) error {
	// 1. 收集所有行
	var rows []*SSTableRow
	iter := imm.NewIterator()
	for iter.Next() {
		var row SSTableRow
		err := json.Unmarshal(iter.Value(), &row)
		if err == nil {
			rows = append(rows, &row)
		}
	}

	if len(rows) == 0 {
		// 没有数据，直接清理
		e.walManager.Delete(walNumber)
		e.memtableManager.RemoveImmutable(imm)
		return nil
	}

	// 2. 从 VersionSet 分配文件编号
	fileNumber := e.versionSet.AllocateFileNumber()

	// 3. 创建 SST 文件到 L0
	reader, err := e.sstManager.CreateSST(fileNumber, rows)
	if err != nil {
		return err
	}

	// 4. 创建 FileMetadata
	header := reader.GetHeader()

	// 获取文件大小
	sstPath := reader.GetPath()
	fileInfo, err := os.Stat(sstPath)
	if err != nil {
		return fmt.Errorf("stat sst file: %w", err)
	}

	fileMeta := &FileMetadata{
		FileNumber: fileNumber,
		Level:      0, // Flush 到 L0
		FileSize:   fileInfo.Size(),
		MinKey:     header.MinKey,
		MaxKey:     header.MaxKey,
		RowCount:   header.RowCount,
	}

	// 5. 更新 MANIFEST
	edit := NewVersionEdit()
	edit.AddFile(fileMeta)

	// 持久化当前的文件编号计数器（关键修复：防止重启后文件编号重用）
	// 使用 fileNumber + 1 确保并发安全，避免竞态条件
	edit.SetNextFileNumber(fileNumber + 1)

	err = e.versionSet.LogAndApply(edit)
	if err != nil {
		return fmt.Errorf("log and apply version edit: %w", err)
	}

	// 6. 删除对应的 WAL
	e.walManager.Delete(walNumber)

	// 7. 从 Immutable 列表中移除
	e.memtableManager.RemoveImmutable(imm)

	// 8. Compaction 由后台线程负责，不在 flush 路径中触发
	// 避免同步 compaction 导致刚创建的文件立即被删除
	// e.compactionManager.MaybeCompact()

	return nil
}

// recover 恢复数据
func (e *Engine) recover() error {
	// 1. 恢复 SST 文件（SST Manager 已经在 NewManager 中恢复了）
	// 只需要获取最大 seq
	maxSeq := e.sstManager.GetMaxSeq()
	if maxSeq > e.seq.Load() {
		e.seq.Store(maxSeq)
	}

	// 2. 恢复所有 WAL 文件到 MemTable Manager
	walDir := filepath.Join(e.dir, "wal")
	pattern := filepath.Join(walDir, "*.wal")
	walFiles, err := filepath.Glob(pattern)
	if err == nil && len(walFiles) > 0 {
		// 按文件名排序
		sort.Strings(walFiles)

		// 依次读取每个 WAL
		for _, walPath := range walFiles {
			reader, err := NewWALReader(walPath)
			if err != nil {
				continue
			}

			entries, err := reader.Read()
			reader.Close()

			if err != nil {
				continue
			}

			// 重放 WAL 到 Active MemTable
			for _, entry := range entries {
				// 如果定义了 Schema，验证数据
				if e.schema != nil {
					var row SSTableRow
					if err := json.Unmarshal(entry.Data, &row); err != nil {
						return fmt.Errorf("failed to unmarshal row during recovery (seq=%d): %w", entry.Seq, err)
					}

					// 验证 Schema
					if err := e.schema.Validate(row.Data); err != nil {
						return fmt.Errorf("schema validation failed during recovery (seq=%d): %w", entry.Seq, err)
					}
				}

				e.memtableManager.Put(entry.Seq, entry.Data)
				if entry.Seq > e.seq.Load() {
					e.seq.Store(entry.Seq)
				}
			}
		}
	}

	return nil
}

// Close 关闭引擎
func (e *Engine) Close() error {
	// 1. 停止后台 Compaction
	if e.compactionManager != nil {
		e.compactionManager.Stop()
	}

	// 2. Flush Active MemTable
	if e.memtableManager.GetActiveCount() > 0 {
		// 切换并 Flush
		e.switchMemTable()
	}

	// 等待所有 Immutable Flush 完成
	// TODO: 添加更优雅的等待机制
	for e.memtableManager.GetImmutableCount() > 0 {
		time.Sleep(100 * time.Millisecond)
	}

	// 3. 保存所有索引
	if e.indexManager != nil {
		e.indexManager.BuildAll()
		e.indexManager.Close()
	}

	// 4. 关闭 VersionSet
	if e.versionSet != nil {
		e.versionSet.Close()
	}

	// 5. 关闭 WAL Manager
	if e.walManager != nil {
		e.walManager.Close()
	}

	// 6. 关闭 SST Manager
	if e.sstManager != nil {
		e.sstManager.Close()
	}

	return nil
}

// TableStats 统计信息
type TableStats struct {
	MemTableSize  int64
	MemTableCount int
	SSTCount      int
	TotalRows     int64
}

// GetVersionSet 获取 VersionSet（用于高级操作）
func (e *Engine) GetVersionSet() *VersionSet {
	return e.versionSet
}

// GetCompactionManager 获取 Compaction Manager（用于高级操作）
func (e *Engine) GetCompactionManager() *CompactionManager {
	return e.compactionManager
}

// GetMemtableManager 获取 Memtable Manager
func (e *Engine) GetMemtableManager() *MemTableManager {
	return e.memtableManager
}

// GetSSTManager 获取 SST Manager
func (e *Engine) GetSSTManager() *SSTableManager {
	return e.sstManager
}

// GetMaxSeq 获取当前最大的 seq 号
func (e *Engine) GetMaxSeq() int64 {
	return e.seq.Load() - 1 // seq 是下一个要分配的，所以最大的是 seq - 1
}

// GetSchema 获取 Schema
func (e *Engine) GetSchema() *Schema {
	return e.schema
}

// Stats 获取统计信息
func (e *Engine) Stats() *TableStats {
	memStats := e.memtableManager.GetStats()
	sstStats := e.sstManager.GetStats()

	stats := &TableStats{
		MemTableSize:  memStats.TotalSize,
		MemTableCount: memStats.TotalCount,
		SSTCount:      sstStats.FileCount,
	}

	// 计算总行数
	stats.TotalRows = int64(memStats.TotalCount)
	readers := e.sstManager.GetReaders()
	for _, reader := range readers {
		header := reader.GetHeader()
		stats.TotalRows += header.RowCount
	}

	return stats
}

// CreateIndex 创建索引
func (e *Engine) CreateIndex(field string) error {
	if e.indexManager == nil {
		return fmt.Errorf("no schema defined, cannot create index")
	}

	return e.indexManager.CreateIndex(field)
}

// DropIndex 删除索引
func (e *Engine) DropIndex(field string) error {
	if e.indexManager == nil {
		return fmt.Errorf("no schema defined, cannot drop index")
	}

	return e.indexManager.DropIndex(field)
}

// ListIndexes 列出所有索引
func (e *Engine) ListIndexes() []string {
	if e.indexManager == nil {
		return nil
	}

	return e.indexManager.ListIndexes()
}

// GetIndexMetadata 获取索引元数据
func (e *Engine) GetIndexMetadata() map[string]IndexMetadata {
	if e.indexManager == nil {
		return nil
	}

	return e.indexManager.GetIndexMetadata()
}

// RepairIndexes 手动修复索引
func (e *Engine) RepairIndexes() error {
	return e.verifyAndRepairIndexes()
}

// Query 创建查询构建器
func (e *Engine) Query() *QueryBuilder {
	return newQueryBuilder(e)
}

// scanAllWithBuilder 使用 QueryBuilder 全表扫描
func (e *Engine) scanAllWithBuilder(qb *QueryBuilder) ([]*SSTableRow, error) {
	// 使用 map 去重（同一个 seq 只保留一次）
	rowMap := make(map[int64]*SSTableRow)

	// 扫描 Active MemTable
	iter := e.memtableManager.NewIterator()
	for iter.Next() {
		seq := iter.Key()
		row, err := e.Get(seq)
		if err == nil && qb.Match(row.Data) {
			rowMap[seq] = row
		}
	}

	// 扫描 Immutable MemTables
	immutables := e.memtableManager.GetImmutables()
	for _, imm := range immutables {
		iter := imm.NewIterator()
		for iter.Next() {
			seq := iter.Key()
			if _, exists := rowMap[seq]; !exists {
				row, err := e.Get(seq)
				if err == nil && qb.Match(row.Data) {
					rowMap[seq] = row
				}
			}
		}
	}

	// 扫描 SST 文件
	readers := e.sstManager.GetReaders()
	for _, reader := range readers {
		header := reader.GetHeader()
		for seq := header.MinKey; seq <= header.MaxKey; seq++ {
			if _, exists := rowMap[seq]; !exists {
				row, err := reader.Get(seq)
				if err == nil && qb.Match(row.Data) {
					rowMap[seq] = row
				}
			}
		}
	}

	// 转换为数组并按 Seq 排序
	results := make([]*SSTableRow, 0, len(rowMap))
	for _, row := range rowMap {
		results = append(results, row)
	}

	// 按 Seq 排序（保证查询结果有序）
	sort.Slice(results, func(i, j int) bool {
		return results[i].Seq < results[j].Seq
	})

	return results, nil
}

// verifyAndRepairIndexes 验证并修复索引
func (e *Engine) verifyAndRepairIndexes() error {
	if e.indexManager == nil {
		return nil
	}

	// 获取当前最大 seq
	currentMaxSeq := e.seq.Load()

	// 创建 getData 函数
	getData := func(seq int64) (map[string]any, error) {
		row, err := e.Get(seq)
		if err != nil {
			return nil, err
		}
		return row.Data, nil
	}

	// 验证并修复
	return e.indexManager.VerifyAndRepair(currentMaxSeq, getData)
}
