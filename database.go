package srdb

import (
	"encoding/json"
	"fmt"
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

// Open 打开数据库
func Open(dir string) (*Database, error) {
	// 创建目录
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	db := &Database{
		dir:    dir,
		tables: make(map[string]*Table),
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
			Dir:          tableDir,
			MemTableSize: DefaultMemTableSize,
		})
		if err != nil {
			// 记录失败的表，但继续恢复其他表
			failedTables = append(failedTables, tableInfo.Name)
			fmt.Printf("[WARNING] Failed to open table %s: %v\n", tableInfo.Name, err)
			fmt.Printf("[WARNING] Table %s will be skipped. You may need to drop and recreate it.\n", tableInfo.Name)
			continue
		}
		db.tables[tableInfo.Name] = table
	}

	// 如果有失败的表，输出汇总信息
	if len(failedTables) > 0 {
		fmt.Printf("[WARNING] %d table(s) failed to recover: %v\n", len(failedTables), failedTables)
		fmt.Printf("[WARNING] To fix: Delete the corrupted table directory and restart.\n")
		fmt.Printf("[WARNING] Example: rm -rf %s/<table_name>\n", db.dir)
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

	// 创建表
	table, err := OpenTable(&TableOptions{
		Dir:          tableDir,
		MemTableSize: DefaultMemTableSize,
		Name:         schema.Name,
		Fields:       schema.Fields,
	})
	if err != nil {
		os.RemoveAll(tableDir)
		return nil, err
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
