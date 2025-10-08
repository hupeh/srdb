package srdb

import (
	"os"
	"path/filepath"
	"time"
)

// Table 表
type Table struct {
	name      string    // 表名
	dir       string    // 表目录
	schema    *Schema   // Schema
	engine    *Engine   // Engine 实例
	database  *Database // 所属数据库
	createdAt int64     // 创建时间
}

// createTable 创建新表
func createTable(name string, schema *Schema, db *Database) (*Table, error) {
	// 创建表目录
	tableDir := filepath.Join(db.dir, name)
	err := os.MkdirAll(tableDir, 0755)
	if err != nil {
		os.RemoveAll(tableDir)
		return nil, err
	}

	// 创建 Engine（Engine 会自动保存 Schema 到文件）
	engine, err := OpenEngine(&EngineOptions{
		Dir:          tableDir,
		MemTableSize: DefaultMemTableSize,
		Schema:       schema,
	})
	if err != nil {
		os.RemoveAll(tableDir)
		return nil, err
	}

	table := &Table{
		name:      name,
		dir:       tableDir,
		schema:    schema,
		engine:    engine,
		database:  db,
		createdAt: time.Now().Unix(),
	}

	return table, nil
}

// openTable 打开已存在的表
func openTable(name string, db *Database) (*Table, error) {
	tableDir := filepath.Join(db.dir, name)

	// 打开 Engine（Engine 会自动从 schema.json 恢复 Schema）
	eng, err := OpenEngine(&EngineOptions{
		Dir:          tableDir,
		MemTableSize: DefaultMemTableSize,
		// Schema 不设置，让 Engine 自动从磁盘恢复
	})
	if err != nil {
		return nil, err
	}

	// 从 Engine 获取 Schema
	sch := eng.GetSchema()

	table := &Table{
		name:     name,
		dir:      tableDir,
		schema:   sch,
		engine:   eng,
		database: db,
	}

	return table, nil
}

// GetName 获取表名
func (t *Table) GetName() string {
	return t.name
}

// GetSchema 获取 Schema
func (t *Table) GetSchema() *Schema {
	return t.schema
}

// Insert 插入数据
func (t *Table) Insert(data map[string]any) error {
	return t.engine.Insert(data)
}

// Get 查询数据
func (t *Table) Get(seq int64) (*SSTableRow, error) {
	return t.engine.Get(seq)
}

// Query 创建查询构建器
func (t *Table) Query() *QueryBuilder {
	return t.engine.Query()
}

// CreateIndex 创建索引
func (t *Table) CreateIndex(field string) error {
	return t.engine.CreateIndex(field)
}

// DropIndex 删除索引
func (t *Table) DropIndex(field string) error {
	return t.engine.DropIndex(field)
}

// ListIndexes 列出所有索引
func (t *Table) ListIndexes() []string {
	return t.engine.ListIndexes()
}

// Stats 获取统计信息
func (t *Table) Stats() *TableStats {
	return t.engine.Stats()
}

// GetEngine 获取底层 Engine
func (t *Table) GetEngine() *Engine {
	return t.engine
}

// Close 关闭表
func (t *Table) Close() error {
	if t.engine != nil {
		return t.engine.Close()
	}
	return nil
}

// GetCreatedAt 获取表创建时间
func (t *Table) GetCreatedAt() int64 {
	return t.createdAt
}
