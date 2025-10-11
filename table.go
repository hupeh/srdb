package srdb

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultMemTableSize     = 64 * 1024 * 1024 // 64 MB
	DefaultAutoFlushTimeout = 30 * time.Second // 30 秒无写入自动 flush
)

// Table 表
type Table struct {
	dir               string
	schema            *Schema
	indexManager      *IndexManager
	walManager        *WALManager        // WAL 管理器
	sstManager        *SSTableManager    // SST 管理器
	memtableManager   *MemTableManager   // MemTable 管理器
	versionSet        *VersionSet        // MANIFEST 管理器
	compactionManager *CompactionManager // Compaction 管理器
	logger            *slog.Logger       // 日志器
	seq               atomic.Int64
	flushMu           sync.Mutex

	// 自动 flush 相关
	autoFlushTimeout time.Duration
	lastWriteTime    atomic.Int64 // 最后写入时间（UnixNano）
	stopAutoFlush    chan struct{}
}

// TableOptions 配置选项
type TableOptions struct {
	Dir              string
	MemTableSize     int64
	Name             string        // 表名
	Fields           []Field       // 字段列表（可选）
	AutoFlushTimeout time.Duration // 自动 flush 超时时间，0 表示禁用
}

// OpenTable 打开数据库
func OpenTable(opts *TableOptions) (*Table, error) {
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
	idxDir := filepath.Join(opts.Dir, "idx")

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

	// 处理 Schema
	var sch *Schema
	if opts.Name != "" && len(opts.Fields) > 0 {
		// 从 Name 和 Fields 创建 Schema
		sch, err = NewSchema(opts.Name, opts.Fields)
		if err != nil {
			return nil, fmt.Errorf("create schema: %w", err)
		}
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
		} else {
			// Schema 文件不存在
			return nil, fmt.Errorf("schema is required but schema.json not found in %s", opts.Dir)
		}
	}

	// 强制要求 Schema
	if sch == nil {
		return nil, fmt.Errorf("schema is required to open table")
	}

	// 创建索引管理器
	indexMgr := NewIndexManager(idxDir, sch)

	// 自动为 Schema 中标记 Indexed 的字段创建索引
	for _, field := range sch.Fields {
		if field.Indexed {
			// 检查索引是否已存在（避免重复创建）
			if _, exists := indexMgr.GetIndex(field.Name); !exists {
				err := indexMgr.CreateIndex(field.Name)
				if err != nil {
					// 索引创建失败，记录警告但不阻塞表创建
					// 此时使用临时 logger（Table 还未完全创建）
					tmpLogger := slog.New(slog.NewTextHandler(os.Stderr, nil))
					tmpLogger.Warn("[Table] Failed to create index for field",
						"field", field.Name,
						"error", err)
				}
			}
		}
	}

	// 创建 SST Manager
	sstMgr, err := NewSSTableManager(sstDir)
	if err != nil {
		return nil, err
	}

	// 设置 Schema（用于优化编解码）
	sstMgr.SetSchema(sch)

	// 创建 MemTable Manager
	memMgr := NewMemTableManager(opts.MemTableSize)

	// 创建/恢复 MANIFEST
	manifestDir := opts.Dir
	versionSet, err := NewVersionSet(manifestDir)
	if err != nil {
		return nil, fmt.Errorf("create version set: %w", err)
	}

	// 创建 Table（暂时不设置 WAL Manager）
	table := &Table{
		dir:             opts.Dir,
		schema:          sch,
		indexManager:    indexMgr,
		walManager:      nil, // 先不设置，恢复后再创建
		sstManager:      sstMgr,
		memtableManager: memMgr,
		versionSet:      versionSet,
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)), // 默认丢弃日志
	}

	// 先恢复数据（包括从 WAL 恢复）
	err = table.recover()
	if err != nil {
		return nil, err
	}

	// 恢复完成后，创建 WAL Manager 用于后续写入
	walMgr, err := NewWALManager(walDir)
	if err != nil {
		return nil, err
	}
	table.walManager = walMgr
	table.memtableManager.SetActiveWAL(walMgr.GetCurrentNumber())

	// 创建 Compaction Manager
	table.compactionManager = NewCompactionManager(sstDir, versionSet, sstMgr)

	// 设置 Schema
	table.compactionManager.SetSchema(sch)

	// 启动时清理孤儿文件（崩溃恢复后的清理）
	table.compactionManager.CleanupOrphanFiles()

	// 启动后台 Compaction 和垃圾回收
	table.compactionManager.Start()

	// 验证并修复索引
	table.verifyAndRepairIndexes()

	// 设置自动 flush 超时时间
	if opts.AutoFlushTimeout > 0 {
		table.autoFlushTimeout = opts.AutoFlushTimeout
	} else {
		table.autoFlushTimeout = DefaultAutoFlushTimeout
	}
	table.stopAutoFlush = make(chan struct{})
	table.lastWriteTime.Store(time.Now().UnixNano())

	// 启动自动 flush 监控
	go table.autoFlushMonitor()

	return table, nil
}

// Insert 插入数据（支持单条或批量）
// 支持的类型：
//   - map[string]any: 单条数据
//   - []map[string]any: 批量数据
//   - *struct{}: 单个结构体指针
//   - []struct{}: 结构体切片
//   - []*struct{}: 结构体指针切片
func (t *Table) Insert(data any) error {
	// 1. 将输入转换为 []map[string]any
	rows, err := t.normalizeInsertData(data)
	if err != nil {
		return err
	}

	// 2. 批量插入
	return t.insertBatch(rows)
}

// normalizeInsertData 将各种输入格式转换为 []map[string]any
func (t *Table) normalizeInsertData(data any) ([]map[string]any, error) {
	// 处理 nil
	if data == nil {
		return nil, fmt.Errorf("data cannot be nil")
	}

	// 获取反射值
	val := reflect.ValueOf(data)
	typ := reflect.TypeOf(data)

	// 如果是指针，解引用
	if typ.Kind() == reflect.Pointer {
		if val.IsNil() {
			return nil, fmt.Errorf("data pointer cannot be nil")
		}
		val = val.Elem()
		typ = val.Type()
	}

	// 获取解引用后的实际值
	actualData := val.Interface()

	switch typ.Kind() {
	case reflect.Map:
		// map[string]any - 单条
		m, ok := actualData.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected map[string]any, got %T", actualData)
		}
		return []map[string]any{m}, nil

	case reflect.Slice:
		// 检查切片元素类型
		elemType := typ.Elem()

		// []map[string]any
		if elemType.Kind() == reflect.Map {
			maps, ok := actualData.([]map[string]any)
			if !ok {
				return nil, fmt.Errorf("expected []map[string]any, got %T", actualData)
			}
			return maps, nil
		}

		// []*struct{} 或 []struct{}
		if elemType.Kind() == reflect.Pointer {
			elemType = elemType.Elem()
		}

		if elemType.Kind() == reflect.Struct {
			// 将每个结构体转换为 map
			var rows []map[string]any
			for i := 0; i < val.Len(); i++ {
				elem := val.Index(i)
				// 如果是指针，解引用
				if elem.Kind() == reflect.Pointer {
					if elem.IsNil() {
						continue // 跳过 nil 指针
					}
					elem = elem.Elem()
				}

				m, err := t.structToMap(elem.Interface())
				if err != nil {
					return nil, fmt.Errorf("convert struct at index %d: %w", i, err)
				}
				rows = append(rows, m)
			}
			return rows, nil
		}

		return nil, fmt.Errorf("unsupported slice element type: %s", elemType.Kind())

	case reflect.Struct:
		// struct{} - 单个结构体
		m, err := t.structToMap(actualData)
		if err != nil {
			return nil, err
		}
		return []map[string]any{m}, nil

	default:
		return nil, fmt.Errorf("unsupported data type: %T (kind: %s)", actualData, typ.Kind())
	}
}

// structToMap 将结构体转换为 map[string]any
func (t *Table) structToMap(v any) (map[string]any, error) {
	val := reflect.ValueOf(v)
	typ := reflect.TypeOf(v)

	if typ.Kind() == reflect.Pointer {
		val = val.Elem()
		typ = val.Type()
	}

	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", typ.Kind())
	}

	result := make(map[string]any)

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// 跳过未导出的字段
		if !field.IsExported() {
			continue
		}

		// 解析 srdb tag
		tag := field.Tag.Get("srdb")
		if tag == "-" {
			// 忽略该字段
			continue
		}

		// 默认使用 snake_case 转换字段名
		fieldName := camelToSnake(field.Name)

		// 解析 tag（与 StructToFields 保持一致）
		if tag != "" {
			parts := strings.Split(tag, ";")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}

				// 检查是否为 field:xxx 格式
				if strings.HasPrefix(part, "field:") {
					fieldName = strings.TrimPrefix(part, "field:")
					break // 找到字段名，停止解析
				}
				// 忽略其他标记（indexed, nullable, comment:xxx）
			}
		}

		// 获取字段值
		fieldVal := val.Field(i)

		// 处理指针类型：如果是指针，解引用（nil 保持为 nil）
		if fieldVal.Kind() == reflect.Pointer {
			if fieldVal.IsNil() {
				result[fieldName] = nil
			} else {
				result[fieldName] = fieldVal.Elem().Interface()
			}
		} else {
			result[fieldName] = fieldVal.Interface()
		}
	}

	return result, nil
}

// insertBatch 批量插入数据
func (t *Table) insertBatch(rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}

	// 逐条插入
	for _, data := range rows {
		if err := t.insertSingle(data); err != nil {
			return err
		}
	}

	return nil
}

// insertSingle 插入单条数据
func (t *Table) insertSingle(data map[string]any) error {
	// 1. 验证 Schema
	if err := t.schema.Validate(data); err != nil {
		return NewError(ErrCodeSchemaValidationFailed, err)
	}

	// 2. 类型转换：将数据转换为 Schema 定义的类型
	// 这样可以确保写入时的类型与 Schema 一致（例如将 int64 转换为 time.Time）
	convertedData := make(map[string]any, len(data))
	for key, value := range data {
		// 跳过 nil 值
		if value == nil {
			convertedData[key] = nil
			continue
		}

		// 获取字段定义
		field, err := t.schema.GetField(key)
		if err != nil {
			// 字段不在 Schema 中，保持原值
			convertedData[key] = value
			continue
		}

		// 使用 Schema 的类型转换
		converted, err := convertValue(value, field.Type)
		if err != nil {
			return NewErrorf(ErrCodeSchemaValidationFailed, "convert field %s: %v", key, err)
		}
		convertedData[key] = converted
	}

	// 3. 生成 _seq
	seq := t.seq.Add(1)

	// 4. 添加系统字段
	row := &SSTableRow{
		Seq:  seq,
		Time: time.Now().UnixNano(),
		Data: convertedData,
	}

	// 3. 序列化（使用二进制格式，保留类型信息）
	rowData, err := encodeSSTableRowBinary(row, t.schema)
	if err != nil {
		return err
	}

	// 4. 写入 WAL
	entry := &WALEntry{
		Type: WALEntryTypePut,
		Seq:  seq,
		Data: rowData,
	}
	err = t.walManager.Append(entry)
	if err != nil {
		return err
	}

	// 5. 写入 MemTable Manager
	t.memtableManager.Put(seq, rowData)

	// 6. 添加到索引
	t.indexManager.AddToIndexes(data, seq)

	// 7. 更新最后写入时间
	t.lastWriteTime.Store(time.Now().UnixNano())

	// 8. 检查是否需要切换 MemTable
	if t.memtableManager.ShouldSwitch() {
		go t.switchMemTable()
	}

	return nil
}

// SetLogger 设置 logger（由 Database 调用）
func (t *Table) SetLogger(logger *slog.Logger) {
	t.logger = logger
}

// Get 查询数据
func (t *Table) Get(seq int64) (*SSTableRow, error) {
	// 1. 先查 MemTable Manager (Active + Immutables)
	data, found := t.memtableManager.Get(seq)
	if found {
		// 使用二进制解码
		row, err := decodeSSTableRowBinary(data, t.schema)
		if err != nil {
			return nil, err
		}
		return row, nil
	}

	// 2. 查询 SST 文件
	return t.sstManager.Get(seq)
}

// GetPartial 按需查询数据（只读取指定字段）
func (t *Table) GetPartial(seq int64, fields []string) (*SSTableRow, error) {
	// 1. 先查 MemTable Manager (Active + Immutables)
	data, found := t.memtableManager.Get(seq)
	if found {
		// 使用二进制解码（支持部分解码）
		row, err := decodeSSTableRowBinaryPartial(data, t.schema, fields)
		if err != nil {
			return nil, err
		}
		return row, nil
	}

	// 2. 查询 SST 文件（按需解码）
	return t.sstManager.GetPartial(seq, fields)
}

// switchMemTable 切换 MemTable
func (t *Table) switchMemTable() error {
	t.flushMu.Lock()
	defer t.flushMu.Unlock()

	// 1. 切换到新的 WAL
	oldWALNumber, err := t.walManager.Rotate()
	if err != nil {
		return err
	}
	newWALNumber := t.walManager.GetCurrentNumber()

	// 2. 切换 MemTable (Active → Immutable)
	_, immutable := t.memtableManager.Switch(newWALNumber)

	// 3. 异步 Flush Immutable
	go t.flushImmutable(immutable, oldWALNumber)

	return nil
}

// flushImmutable 将 Immutable MemTable 刷新到 SST
func (t *Table) flushImmutable(imm *ImmutableMemTable, walNumber int64) error {
	// 1. 收集所有行
	var rows []*SSTableRow
	iter := imm.NewIterator()
	for iter.Next() {
		// 使用二进制解码
		row, err := decodeSSTableRowBinary(iter.Value(), t.schema)
		if err == nil {
			rows = append(rows, row)
		}
	}

	if len(rows) == 0 {
		// 没有数据，直接清理
		t.walManager.Delete(walNumber)
		t.memtableManager.RemoveImmutable(imm)
		return nil
	}

	// 2. 从 VersionSet 分配文件编号
	fileNumber := t.versionSet.AllocateFileNumber()

	// 3. 创建 SST 文件到 L0
	reader, err := t.sstManager.CreateSST(fileNumber, rows)
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

	err = t.versionSet.LogAndApply(edit)
	if err != nil {
		return fmt.Errorf("log and apply version edit: %w", err)
	}

	// 6. 删除对应的 WAL
	t.walManager.Delete(walNumber)

	// 7. 从 Immutable 列表中移除
	t.memtableManager.RemoveImmutable(imm)

	// 8. 持久化索引（防止崩溃丢失索引数据）
	t.indexManager.BuildAll()

	// 9. Compaction 由后台线程负责，不在 flush 路径中触发
	// 避免同步 compaction 导致刚创建的文件立即被删除
	// t.compactionManager.MaybeCompact()

	return nil
}

// recover 恢复数据
func (t *Table) recover() error {
	// 1. 恢复 SST 文件（SST Manager 已经在 NewManager 中恢复了）
	// 只需要获取最大 seq
	maxSeq := t.sstManager.GetMaxSeq()
	if maxSeq > t.seq.Load() {
		t.seq.Store(maxSeq)
	}

	// 2. 恢复所有 WAL 文件到 MemTable Manager
	walDir := filepath.Join(t.dir, "wal")
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
				// 使用二进制解码验证 Schema
				row, err := decodeSSTableRowBinary(entry.Data, t.schema)
				if err != nil {
					return fmt.Errorf("failed to decode row during recovery (seq=%d): %w", entry.Seq, err)
				}

				// 验证 Schema
				if err := t.schema.Validate(row.Data); err != nil {
					return NewErrorf(ErrCodeSchemaValidationFailed, "schema validation failed during recovery (seq=%d)", entry.Seq, err)
				}

				t.memtableManager.Put(entry.Seq, entry.Data)
				if entry.Seq > t.seq.Load() {
					t.seq.Store(entry.Seq)
				}
			}
		}
	}

	return nil
}

// autoFlushMonitor 自动 flush 监控
func (t *Table) autoFlushMonitor() {
	ticker := time.NewTicker(t.autoFlushTimeout / 2) // 每半个超时时间检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 检查是否超时
			lastWrite := time.Unix(0, t.lastWriteTime.Load())
			if time.Since(lastWrite) >= t.autoFlushTimeout {
				// 检查 MemTable 是否有数据
				active := t.memtableManager.GetActive()
				if active != nil && active.Size() > 0 {
					// 触发 flush
					t.Flush()
				}
			}
		case <-t.stopAutoFlush:
			return
		}
	}
}

// Flush 手动刷新 Active MemTable 到磁盘
func (t *Table) Flush() error {
	// 检查 Active MemTable 是否有数据
	active := t.memtableManager.GetActive()
	if active == nil || active.Size() == 0 {
		return nil // 没有数据，无需 flush
	}

	// 强制切换 MemTable（switchMemTable 内部有锁）
	return t.switchMemTable()
}

// Close 关闭引擎
func (t *Table) Close() error {
	// 1. 停止自动 flush 监控（如果还在运行）
	if t.stopAutoFlush != nil {
		select {
		case <-t.stopAutoFlush:
			// 已经关闭，跳过
		default:
			close(t.stopAutoFlush)
		}
	}

	// 2. 停止 Compaction Manager
	if t.compactionManager != nil {
		t.compactionManager.Stop()
	}

	// 3. 刷新 Active MemTable（确保所有数据都写入磁盘）
	// 检查 memtableManager 是否存在（可能已被 Destroy）
	if t.memtableManager != nil {
		t.Flush()
	}

	// 3. 关闭 WAL Manager
	if t.walManager != nil {
		t.walManager.Close()
	}

	// 4. 等待所有 Immutable Flush 完成
	// TODO: 添加更优雅的等待机制
	if t.memtableManager != nil {
		for t.memtableManager.GetImmutableCount() > 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// 5. 保存所有索引
	if t.indexManager != nil {
		t.indexManager.BuildAll()
		t.indexManager.Close()
	}

	// 6. 关闭 VersionSet
	if t.versionSet != nil {
		t.versionSet.Close()
	}

	// 7. 关闭 WAL Manager
	if t.walManager != nil {
		t.walManager.Close()
	}

	// 6. 关闭 SST Manager
	if t.sstManager != nil {
		t.sstManager.Close()
	}

	return nil
}

// Clean 清除所有数据（保留 Table 可用）
func (t *Table) Clean() error {
	t.flushMu.Lock()
	defer t.flushMu.Unlock()

	// 0. 停止自动 flush 监控（临时）
	if t.stopAutoFlush != nil {
		close(t.stopAutoFlush)
	}

	// 1. 停止 Compaction Manager
	if t.compactionManager != nil {
		t.compactionManager.Stop()
	}

	// 2. 等待所有 Immutable Flush 完成
	for t.memtableManager.GetImmutableCount() > 0 {
		time.Sleep(100 * time.Millisecond)
	}

	// 3. 清空 MemTable
	t.memtableManager = NewMemTableManager(DefaultMemTableSize)

	// 2. 删除所有 WAL 文件
	if t.walManager != nil {
		t.walManager.Close()
		walDir := filepath.Join(t.dir, "wal")
		os.RemoveAll(walDir)
		os.MkdirAll(walDir, 0755)

		// 重新创建 WAL Manager
		walMgr, err := NewWALManager(walDir)
		if err != nil {
			return fmt.Errorf("recreate wal manager: %w", err)
		}
		t.walManager = walMgr
		t.memtableManager.SetActiveWAL(walMgr.GetCurrentNumber())
	}

	// 3. 删除所有 SST 文件
	if t.sstManager != nil {
		t.sstManager.Close()
		sstDir := filepath.Join(t.dir, "sst")
		os.RemoveAll(sstDir)
		os.MkdirAll(sstDir, 0755)

		// 重新创建 SST Manager
		sstMgr, err := NewSSTableManager(sstDir)
		if err != nil {
			return fmt.Errorf("recreate sst manager: %w", err)
		}
		t.sstManager = sstMgr
		// 设置 Schema
		t.sstManager.SetSchema(t.schema)
	}

	// 4. 删除所有索引文件
	if t.indexManager != nil {
		t.indexManager.Close()
		indexFiles, _ := filepath.Glob(filepath.Join(t.dir, "idx_*.sst"))
		for _, f := range indexFiles {
			os.Remove(f)
		}

		// 重新创建 Index Manager
		t.indexManager = NewIndexManager(t.dir, t.schema)
	}

	// 5. 重置 MANIFEST
	if t.versionSet != nil {
		t.versionSet.Close()
		manifestDir := t.dir
		os.Remove(filepath.Join(manifestDir, "MANIFEST"))
		os.Remove(filepath.Join(manifestDir, "CURRENT"))

		// 重新创建 VersionSet
		versionSet, err := NewVersionSet(manifestDir)
		if err != nil {
			return fmt.Errorf("recreate version set: %w", err)
		}
		t.versionSet = versionSet
	}

	// 6. 重新创建 Compaction Manager
	sstDir := filepath.Join(t.dir, "sst")
	t.compactionManager = NewCompactionManager(sstDir, t.versionSet, t.sstManager)
	t.compactionManager.SetSchema(t.schema)
	t.compactionManager.Start()

	// 7. 重置序列号
	t.seq.Store(0)

	// 8. 更新最后写入时间
	t.lastWriteTime.Store(time.Now().UnixNano())

	// 9. 重启自动 flush 监控
	t.stopAutoFlush = make(chan struct{})
	go t.autoFlushMonitor()

	return nil
}

// Destroy 销毁 Table 并删除所有数据文件
func (t *Table) Destroy() error {
	// 1. 先关闭 Table
	if err := t.Close(); err != nil {
		return fmt.Errorf("close table: %w", err)
	}

	// 2. 删除整个数据目录
	if err := os.RemoveAll(t.dir); err != nil {
		return fmt.Errorf("remove data directory: %w", err)
	}

	// 3. 标记 Table 为不可用（将所有管理器设为 nil）
	t.walManager = nil
	t.sstManager = nil
	t.memtableManager = nil
	t.versionSet = nil
	t.compactionManager = nil
	t.indexManager = nil

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
func (t *Table) GetVersionSet() *VersionSet {
	return t.versionSet
}

// GetCompactionManager 获取 Compaction Manager（用于高级操作）
func (t *Table) GetCompactionManager() *CompactionManager {
	return t.compactionManager
}

// GetMemtableManager 获取 Memtable Manager
func (t *Table) GetMemtableManager() *MemTableManager {
	return t.memtableManager
}

// GetSSTManager 获取 SST Manager
func (t *Table) GetSSTManager() *SSTableManager {
	return t.sstManager
}

// GetMaxSeq 获取当前最大的 seq 号
func (t *Table) GetMaxSeq() int64 {
	return t.seq.Load() - 1 // seq 是下一个要分配的，所以最大的是 seq - 1
}

// GetName 获取表名
func (t *Table) GetName() string {
	return t.schema.Name
}

// GetDir 获取表目录
func (t *Table) GetDir() string {
	return t.dir
}

// GetSchema 获取 Schema
func (t *Table) GetSchema() *Schema {
	return t.schema
}

// Stats 获取统计信息
func (t *Table) Stats() *TableStats {
	memStats := t.memtableManager.GetStats()
	sstStats := t.sstManager.GetStats()

	stats := &TableStats{
		MemTableSize:  memStats.TotalSize,
		MemTableCount: memStats.TotalCount,
		SSTCount:      sstStats.FileCount,
	}

	// 计算总行数
	stats.TotalRows = int64(memStats.TotalCount)
	readers := t.sstManager.GetReaders()
	for _, reader := range readers {
		header := reader.GetHeader()
		stats.TotalRows += header.RowCount
	}

	return stats
}

// CreateIndex 创建索引
func (t *Table) CreateIndex(field string) error {
	return t.indexManager.CreateIndex(field)
}

// DropIndex 删除索引
func (t *Table) DropIndex(field string) error {
	return t.indexManager.DropIndex(field)
}

// ListIndexes 列出所有索引
func (t *Table) ListIndexes() []string {
	return t.indexManager.ListIndexes()
}

// GetIndex 获取指定字段的索引
func (t *Table) GetIndex(field string) (*SecondaryIndex, bool) {
	return t.indexManager.GetIndex(field)
}

// BuildIndexes 构建所有索引
func (t *Table) BuildIndexes() error {
	return t.indexManager.BuildAll()
}

// GetIndexMetadata 获取索引元数据
func (t *Table) GetIndexMetadata() map[string]IndexMetadata {
	return t.indexManager.GetIndexMetadata()
}

// RepairIndexes 手动修复索引
func (t *Table) RepairIndexes() error {
	return t.verifyAndRepairIndexes()
}

// Query 创建查询构建器
func (t *Table) Query() *QueryBuilder {
	return newQueryBuilder(t)
}

// verifyAndRepairIndexes 验证并修复索引
func (t *Table) verifyAndRepairIndexes() error {
	// 获取当前最大 seq
	currentMaxSeq := t.seq.Load()

	// 创建 getData 函数
	getData := func(seq int64) (map[string]any, error) {
		row, err := t.Get(seq)
		if err != nil {
			return nil, err
		}
		return row.Data, nil
	}

	// 验证并修复
	return t.indexManager.VerifyAndRepair(currentMaxSeq, getData)
}
