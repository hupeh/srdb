package srdb

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Database 数据库，管理多个表
type Database struct {
	// 数据库目录
	dir string

	// 所有表
	tables map[string]*Table

	// 元数据
	metadata *Metadata

	// 配置选项
	options *Options

	// 锁
	mu sync.RWMutex
}

// Metadata 数据库元数据
type Metadata struct {
	Version int         `json:"version"`
	Tables  []TableInfo `json:"tables"`
}

// TableInfo 表信息
type TableInfo struct {
	Name      string `json:"name"`
	Dir       string `json:"dir"`
	CreatedAt int64  `json:"created_at"`
}

// Options 数据库配置选项
type Options struct {
	// ========== 基础配置 ==========
	Dir    string       // 数据库目录（必需）
	Logger *slog.Logger // 日志器（可选，nil 表示不输出日志）

	// ========== MemTable 配置 ==========
	MemTableSize     int64         // MemTable 大小限制（字节），默认 64MB
	AutoFlushTimeout time.Duration // 自动 flush 超时时间，默认 30s，0 表示禁用

	// ========== Compaction 配置 ==========
	// 层级大小限制
	Level0SizeLimit int64 // L0 层大小限制，默认 64MB
	Level1SizeLimit int64 // L1 层大小限制，默认 256MB
	Level2SizeLimit int64 // L2 层大小限制，默认 512MB
	Level3SizeLimit int64 // L3 层大小限制，默认 1GB

	// 后台任务间隔
	CompactionInterval time.Duration // Compaction 检查间隔，默认 10s
	GCInterval         time.Duration // 垃圾回收检查间隔，默认 5min

	// ========== 高级配置（可选）==========
	DisableAutoCompaction bool          // 禁用自动 Compaction，默认 false
	DisableGC             bool          // 禁用垃圾回收，默认 false
	GCFileMinAge          time.Duration // GC 文件最小年龄，默认 1min
}

// DefaultOptions 返回默认配置
func DefaultOptions(dir string) *Options {
	return &Options{
		Dir:                   dir,
		Logger:                nil,                // 默认不输出日志
		MemTableSize:          64 * 1024 * 1024,   // 64MB
		AutoFlushTimeout:      30 * time.Second,   // 30s
		Level0SizeLimit:       64 * 1024 * 1024,   // 64MB
		Level1SizeLimit:       256 * 1024 * 1024,  // 256MB
		Level2SizeLimit:       512 * 1024 * 1024,  // 512MB
		Level3SizeLimit:       1024 * 1024 * 1024, // 1GB
		CompactionInterval:    10 * time.Second,   // 10s
		GCInterval:            5 * time.Minute,    // 5min
		DisableAutoCompaction: false,
		DisableGC:             false,
		GCFileMinAge:          1 * time.Minute, // 1min
	}
}

// fillDefaults 填充未设置的默认值（修改传入的 opts）
func (opts *Options) fillDefaults() {
	// Logger：如果为 nil，创建一个丢弃所有日志的 logger
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opts.MemTableSize == 0 {
		opts.MemTableSize = 64 * 1024 * 1024 // 64MB
	}
	if opts.AutoFlushTimeout == 0 {
		opts.AutoFlushTimeout = 30 * time.Second // 30s
	}
	if opts.Level0SizeLimit == 0 {
		opts.Level0SizeLimit = 64 * 1024 * 1024 // 64MB
	}
	if opts.Level1SizeLimit == 0 {
		opts.Level1SizeLimit = 256 * 1024 * 1024 // 256MB
	}
	if opts.Level2SizeLimit == 0 {
		opts.Level2SizeLimit = 512 * 1024 * 1024 // 512MB
	}
	if opts.Level3SizeLimit == 0 {
		opts.Level3SizeLimit = 1024 * 1024 * 1024 // 1GB
	}
	if opts.CompactionInterval == 0 {
		opts.CompactionInterval = 10 * time.Second // 10s
	}
	if opts.GCInterval == 0 {
		opts.GCInterval = 5 * time.Minute // 5min
	}
	if opts.GCFileMinAge == 0 {
		opts.GCFileMinAge = 1 * time.Minute // 1min
	}
}

// Validate 验证配置的有效性
func (opts *Options) Validate() error {
	if opts.Dir == "" {
		return NewErrorf(ErrCodeInvalidParam, "database directory cannot be empty")
	}
	if opts.MemTableSize < 1*1024*1024 {
		return NewErrorf(ErrCodeInvalidParam, "MemTableSize must be at least 1MB, got %d", opts.MemTableSize)
	}
	if opts.Level0SizeLimit < 1*1024*1024 {
		return NewErrorf(ErrCodeInvalidParam, "Level0SizeLimit must be at least 1MB, got %d", opts.Level0SizeLimit)
	}
	if opts.Level1SizeLimit < opts.Level0SizeLimit {
		return NewErrorf(ErrCodeInvalidParam, "Level1SizeLimit (%d) must be >= Level0SizeLimit (%d)", opts.Level1SizeLimit, opts.Level0SizeLimit)
	}
	if opts.Level2SizeLimit < opts.Level1SizeLimit {
		return NewErrorf(ErrCodeInvalidParam, "Level2SizeLimit (%d) must be >= Level1SizeLimit (%d)", opts.Level2SizeLimit, opts.Level1SizeLimit)
	}
	if opts.Level3SizeLimit < opts.Level2SizeLimit {
		return NewErrorf(ErrCodeInvalidParam, "Level3SizeLimit (%d) must be >= Level2SizeLimit (%d)", opts.Level3SizeLimit, opts.Level2SizeLimit)
	}
	if opts.CompactionInterval < 1*time.Second {
		return NewErrorf(ErrCodeInvalidParam, "CompactionInterval must be at least 1s, got %v", opts.CompactionInterval)
	}
	if opts.GCInterval < 1*time.Minute {
		return NewErrorf(ErrCodeInvalidParam, "GCInterval must be at least 1min, got %v", opts.GCInterval)
	}
	if opts.GCFileMinAge < 0 {
		return NewErrorf(ErrCodeInvalidParam, "GCFileMinAge cannot be negative, got %v", opts.GCFileMinAge)
	}
	return nil
}

// Open 打开数据库（向后兼容，使用默认配置）
func Open(dir string) (*Database, error) {
	return OpenWithOptions(DefaultOptions(dir))
}

// OpenWithOptions 使用指定配置打开数据库
func OpenWithOptions(opts *Options) (*Database, error) {
	// 填充默认值
	opts.fillDefaults()

	// 验证配置
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// 创建目录
	err := os.MkdirAll(opts.Dir, 0755)
	if err != nil {
		return nil, err
	}

	db := &Database{
		dir:     opts.Dir,
		tables:  make(map[string]*Table),
		options: opts,
	}

	// 加载元数据
	err = db.loadMetadata()
	if err != nil {
		// 如果元数据不存在，创建新的
		db.metadata = &Metadata{
			Version: 1,
			Tables:  make([]TableInfo, 0),
		}
		err = db.saveMetadata()
		if err != nil {
			return nil, err
		}
	}

	// 恢复所有表
	err = db.recoverTables()
	if err != nil {
		return nil, err
	}

	return db, nil
}

// loadMetadata 加载元数据
func (db *Database) loadMetadata() error {
	metaPath := filepath.Join(db.dir, "database.meta")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return err
	}

	db.metadata = &Metadata{}
	return json.Unmarshal(data, db.metadata)
}

// saveMetadata 保存元数据
func (db *Database) saveMetadata() error {
	metaPath := filepath.Join(db.dir, "database.meta")
	data, err := json.MarshalIndent(db.metadata, "", "  ")
	if err != nil {
		return err
	}

	// 原子性写入
	tmpPath := metaPath + ".tmp"
	err = os.WriteFile(tmpPath, data, 0644)
	if err != nil {
		return err
	}

	return os.Rename(tmpPath, metaPath)
}

// recoverTables 恢复所有表
func (db *Database) recoverTables() error {
	var failedTables []string

	for _, tableInfo := range db.metadata.Tables {
		tableDir := filepath.Join(db.dir, tableInfo.Name)
		table, err := OpenTable(&TableOptions{
			Dir:              tableDir,
			MemTableSize:     db.options.MemTableSize,
			AutoFlushTimeout: db.options.AutoFlushTimeout,
		})
		if err != nil {
			// 记录失败的表，但继续恢复其他表
			failedTables = append(failedTables, tableInfo.Name)
			db.options.Logger.Warn("[Database] Failed to open table",
				"table", tableInfo.Name,
				"error", err)
			db.options.Logger.Warn("[Database] Table will be skipped. You may need to drop and recreate it.",
				"table", tableInfo.Name)
			continue
		}

		// 设置 Logger
		table.SetLogger(db.options.Logger)

		// 将数据库级 Compaction 配置应用到表的 CompactionManager
		if table.compactionManager != nil {
			table.compactionManager.ApplyConfig(db.options)
		}

		db.tables[tableInfo.Name] = table
	}

	// 如果有失败的表，输出汇总信息
	if len(failedTables) > 0 {
		db.options.Logger.Warn("[Database] Failed to recover tables",
			"failed_count", len(failedTables),
			"failed_tables", failedTables)
		db.options.Logger.Warn("[Database] To fix: Delete the corrupted table directory and restart",
			"example", fmt.Sprintf("rm -rf %s/<table_name>", db.dir))
	}

	return nil
}

// CreateTable 创建表
func (db *Database) CreateTable(name string, schema *Schema) (*Table, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	// 检查表是否已存在
	if _, exists := db.tables[name]; exists {
		return nil, NewErrorf(ErrCodeTableExists, "table %s already exists", name)
	}

	// 创建表目录
	tableDir := filepath.Join(db.dir, name)
	err := os.MkdirAll(tableDir, 0755)
	if err != nil {
		return nil, err
	}

	// 创建表（传递数据库级配置）
	table, err := OpenTable(&TableOptions{
		Dir:              tableDir,
		MemTableSize:     db.options.MemTableSize,
		AutoFlushTimeout: db.options.AutoFlushTimeout,
		Name:             schema.Name,
		Fields:           schema.Fields,
	})
	if err != nil {
		os.RemoveAll(tableDir)
		return nil, err
	}

	// 设置 Logger
	table.SetLogger(db.options.Logger)

	// 将数据库级 Compaction 配置应用到表的 CompactionManager
	if table.compactionManager != nil {
		table.compactionManager.ApplyConfig(db.options)
	}

	// 添加到 tables map
	db.tables[name] = table

	// 更新元数据
	db.metadata.Tables = append(db.metadata.Tables, TableInfo{
		Name:      name,
		Dir:       name,
		CreatedAt: time.Now().Unix(),
	})

	err = db.saveMetadata()
	if err != nil {
		return nil, err
	}

	return table, nil
}

// GetTable 获取表
func (db *Database) GetTable(name string) (*Table, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	table, exists := db.tables[name]
	if !exists {
		return nil, NewErrorf(ErrCodeTableNotFound, "table %s not found", name)
	}

	return table, nil
}

// DropTable 删除表
func (db *Database) DropTable(name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// 检查表是否存在
	table, exists := db.tables[name]
	if !exists {
		return NewErrorf(ErrCodeTableNotFound, "table %s not found", name)
	}

	// 关闭表
	err := table.Close()
	if err != nil {
		return err
	}

	// 从 map 中移除
	delete(db.tables, name)

	// 删除表目录
	tableDir := filepath.Join(db.dir, name)
	err = os.RemoveAll(tableDir)
	if err != nil {
		return err
	}

	// 更新元数据
	newTables := make([]TableInfo, 0)
	for _, info := range db.metadata.Tables {
		if info.Name != name {
			newTables = append(newTables, info)
		}
	}
	db.metadata.Tables = newTables

	return db.saveMetadata()
}

// ListTables 列出所有表
func (db *Database) ListTables() []string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	tables := make([]string, 0, len(db.tables))
	for name := range db.tables {
		tables = append(tables, name)
	}
	return tables
}

// Close 关闭数据库
func (db *Database) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// 关闭所有表
	for _, table := range db.tables {
		err := table.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// GetAllTablesInfo 获取所有表的信息（用于 WebUI）
func (db *Database) GetAllTablesInfo() map[string]*Table {
	db.mu.RLock()
	defer db.mu.RUnlock()

	// 返回副本以避免并发问题
	result := make(map[string]*Table, len(db.tables))
	maps.Copy(result, db.tables)
	return result
}

// CleanTable 清除指定表的数据（保留表结构）
func (db *Database) CleanTable(name string) error {
	db.mu.RLock()
	table, exists := db.tables[name]
	db.mu.RUnlock()

	if !exists {
		return fmt.Errorf("table %s does not exist", name)
	}

	return table.Clean()
}

// DestroyTable 销毁指定表并从 Database 中删除
func (db *Database) DestroyTable(name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	table, exists := db.tables[name]
	if !exists {
		return fmt.Errorf("table %s does not exist", name)
	}

	// 1. 销毁表（删除文件）
	if err := table.Destroy(); err != nil {
		return fmt.Errorf("destroy table: %w", err)
	}

	// 2. 从内存中删除
	delete(db.tables, name)

	// 3. 从元数据中删除
	newTables := make([]TableInfo, 0, len(db.metadata.Tables)-1)
	for _, info := range db.metadata.Tables {
		if info.Name != name {
			newTables = append(newTables, info)
		}
	}
	db.metadata.Tables = newTables

	// 4. 保存元数据
	return db.saveMetadata()
}

// Clean 清除所有表的数据（保留表结构和 Database 可用）
func (db *Database) Clean() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// 清除所有表的数据
	for name, table := range db.tables {
		if err := table.Clean(); err != nil {
			return fmt.Errorf("clean table %s: %w", name, err)
		}
	}

	return nil
}

// Destroy 销毁整个数据库并删除所有数据文件
func (db *Database) Destroy() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// 1. 关闭所有表
	for _, table := range db.tables {
		if err := table.Close(); err != nil {
			return fmt.Errorf("close table: %w", err)
		}
	}

	// 2. 删除整个数据库目录
	if err := os.RemoveAll(db.dir); err != nil {
		return fmt.Errorf("remove database directory: %w", err)
	}

	// 3. 清空内存中的表
	db.tables = make(map[string]*Table)
	db.metadata.Tables = nil

	return nil
}
