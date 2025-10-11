package srdb

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// IndexMetadata 索引元数据
type IndexMetadata struct {
	Version   int64 // 索引版本号
	MaxSeq    int64 // 索引包含的最大 seq
	MinSeq    int64 // 索引包含的最小 seq
	RowCount  int64 // 索引包含的行数
	CreatedAt int64 // 创建时间
	UpdatedAt int64 // 更新时间
}

// SecondaryIndex 二级索引
type SecondaryIndex struct {
	name         string             // 索引名称
	field        string             // 字段名
	fieldType    FieldType          // 字段类型
	file         *os.File           // 索引文件
	btreeReader  *IndexBTreeReader  // B+Tree 读取器
	valueToSeq   map[string][]int64 // 值 → seq 列表 (构建时使用)
	metadata     IndexMetadata      // 元数据
	mu           sync.RWMutex
	ready        bool // 索引是否就绪
	useBTree     bool // 是否使用 B+Tree 存储（新格式）
}

// NewSecondaryIndex 创建二级索引
func NewSecondaryIndex(dir, field string, fieldType FieldType) (*SecondaryIndex, error) {
	indexPath := filepath.Join(dir, fmt.Sprintf("idx_%s.sst", field))
	file, err := os.OpenFile(indexPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	return &SecondaryIndex{
		name:       field,
		field:      field,
		fieldType:  fieldType,
		file:       file,
		valueToSeq: make(map[string][]int64),
		ready:      false,
	}, nil
}

// Add 添加索引条目（增量更新元数据）
func (idx *SecondaryIndex) Add(value any, seq int64) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 将值转换为字符串作为 key
	key := fmt.Sprintf("%v", value)
	idx.valueToSeq[key] = append(idx.valueToSeq[key], seq)

	// 增量更新元数据 O(1)
	if idx.metadata.MinSeq == 0 || seq < idx.metadata.MinSeq {
		idx.metadata.MinSeq = seq
	}
	if seq > idx.metadata.MaxSeq {
		idx.metadata.MaxSeq = seq
	}
	idx.metadata.RowCount++
	idx.metadata.UpdatedAt = time.Now().UnixNano()

	// 首次添加时设置 CreatedAt
	if idx.metadata.CreatedAt == 0 {
		idx.metadata.CreatedAt = time.Now().UnixNano()
	}

	return nil
}

// Build 构建索引并持久化（B+Tree 格式）
func (idx *SecondaryIndex) Build() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 元数据已在 Add 时增量更新，这里只更新版本号
	idx.metadata.Version++
	idx.metadata.UpdatedAt = time.Now().UnixNano()

	// Truncate 文件
	err := idx.file.Truncate(0)
	if err != nil {
		return err
	}

	// 使用 B+Tree 写入器
	writer := NewIndexBTreeWriter(idx.file, idx.metadata)

	// 写入内存中的所有条目
	// 注意：这假设 valueToSeq 包含所有数据（包括从磁盘加载的）
	// 对于增量更新场景，Get() 会合并内存和磁盘的结果
	for value, seqs := range idx.valueToSeq {
		writer.Add(value, seqs)
	}

	// 构建并写入
	err = writer.Build()
	if err != nil {
		return fmt.Errorf("failed to build btree index: %w", err)
	}

	// 关闭旧的 btreeReader
	if idx.btreeReader != nil {
		idx.btreeReader.Close()
	}

	// 重新加载 btreeReader（读取刚写入的数据）
	reader, err := NewIndexBTreeReader(idx.file)
	if err != nil {
		return fmt.Errorf("failed to reload btree reader: %w", err)
	}

	idx.btreeReader = reader
	idx.useBTree = true
	idx.ready = true

	// 不清空 valueToSeq，保留所有数据在内存中
	// 这样下次 Build() 时可以写入完整数据
	// Get() 方法会合并内存和磁盘的结果（去重）

	return nil
}

// load 从磁盘加载索引（支持 B+Tree 和 JSON 格式）
func (idx *SecondaryIndex) load() error {
	// 获取文件大小
	stat, err := idx.file.Stat()
	if err != nil {
		return err
	}

	if stat.Size() == 0 {
		// 空文件，索引不存在
		return nil
	}

	// 读取文件头，判断格式
	headerData := make([]byte, min(int(stat.Size()), IndexHeaderSize))
	_, err = idx.file.ReadAt(headerData, 0)
	if err != nil {
		return err
	}

	// 检查是否为 B+Tree 格式
	if len(headerData) >= 4 {
		magic := binary.LittleEndian.Uint32(headerData[0:4])
		if magic == IndexMagic {
			// B+Tree 格式
			return idx.loadBTree()
		}
	}

	// 回退到 JSON 格式（向后兼容）
	return idx.loadJSON()
}

// loadBTree 加载 B+Tree 格式的索引
func (idx *SecondaryIndex) loadBTree() error {
	reader, err := NewIndexBTreeReader(idx.file)
	if err != nil {
		return fmt.Errorf("failed to create btree reader: %w", err)
	}

	idx.btreeReader = reader
	idx.metadata = reader.GetMetadata()
	idx.useBTree = true
	idx.ready = true
	return nil
}

// loadJSON 加载 JSON 格式的索引（向后兼容）
func (idx *SecondaryIndex) loadJSON() error {
	stat, err := idx.file.Stat()
	if err != nil {
		return err
	}

	// 读取文件内容
	data := make([]byte, stat.Size())
	_, err = idx.file.ReadAt(data, 0)
	if err != nil {
		return err
	}

	// 加载 JSON 格式
	var indexData struct {
		Metadata   IndexMetadata      `json:"metadata"`
		ValueToSeq map[string][]int64 `json:"data"`
	}

	err = json.Unmarshal(data, &indexData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal index data: %w", err)
	}

	if indexData.ValueToSeq == nil {
		return fmt.Errorf("invalid index data: missing data field")
	}

	idx.metadata = indexData.Metadata
	idx.valueToSeq = indexData.ValueToSeq
	idx.useBTree = false
	idx.ready = true
	return nil
}

// Get 查询索引（优先查内存，然后查磁盘，合并结果）
func (idx *SecondaryIndex) Get(value any) ([]int64, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if !idx.ready {
		return nil, fmt.Errorf("index not ready")
	}

	key := fmt.Sprintf("%v", value)

	// 收集所有匹配的 seqs（需要去重）
	seqMap := make(map[int64]bool)

	// 1. 先从内存 map 读取（包含最新的未持久化数据）
	if memSeqs, exists := idx.valueToSeq[key]; exists {
		for _, seq := range memSeqs {
			seqMap[seq] = true
		}
	}

	// 2. 如果使用 B+Tree，从 B+Tree 读取（持久化的数据）
	if idx.useBTree && idx.btreeReader != nil {
		diskSeqs, err := idx.btreeReader.Get(key)
		if err == nil && diskSeqs != nil {
			for _, seq := range diskSeqs {
				seqMap[seq] = true
			}
		}
	}

	// 3. 合并结果
	if len(seqMap) == 0 {
		return nil, nil
	}

	result := make([]int64, 0, len(seqMap))
	for seq := range seqMap {
		result = append(result, seq)
	}

	return result, nil
}

// IsReady 索引是否就绪
func (idx *SecondaryIndex) IsReady() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.ready
}

// GetMetadata 获取元数据
func (idx *SecondaryIndex) GetMetadata() IndexMetadata {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.metadata
}

// ForEach 升序迭代所有索引条目
// callback 返回 false 时停止迭代，支持提前终止
// 注意：只能迭代已持久化的数据（B+Tree），不包括内存中未持久化的数据
func (idx *SecondaryIndex) ForEach(callback IndexEntryCallback) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if !idx.ready {
		return fmt.Errorf("index not ready")
	}

	// 只支持 B+Tree 格式的索引
	if !idx.useBTree || idx.btreeReader == nil {
		return fmt.Errorf("ForEach only supports B+Tree format indexes")
	}

	idx.btreeReader.ForEach(callback)
	return nil
}

// ForEachDesc 降序迭代所有索引条目
// callback 返回 false 时停止迭代，支持提前终止
// 注意：只能迭代已持久化的数据（B+Tree），不包括内存中未持久化的数据
func (idx *SecondaryIndex) ForEachDesc(callback IndexEntryCallback) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if !idx.ready {
		return fmt.Errorf("index not ready")
	}

	// 只支持 B+Tree 格式的索引
	if !idx.useBTree || idx.btreeReader == nil {
		return fmt.Errorf("ForEachDesc only supports B+Tree format indexes")
	}

	idx.btreeReader.ForEachDesc(callback)
	return nil
}

// NeedsUpdate 检查是否需要更新
func (idx *SecondaryIndex) NeedsUpdate(currentMaxSeq int64) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.metadata.MaxSeq < currentMaxSeq
}

// IncrementalUpdate 增量更新索引
func (idx *SecondaryIndex) IncrementalUpdate(getData func(int64) (map[string]any, error), fromSeq, toSeq int64) error {
	idx.mu.Lock()

	addedCount := int64(0)
	// 遍历缺失的 seq 范围
	for seq := fromSeq; seq <= toSeq; seq++ {
		// 获取数据
		data, err := getData(seq)
		if err != nil {
			continue // 跳过错误的数据
		}

		// 提取字段值
		value, exists := data[idx.field]
		if !exists {
			continue
		}

		// 添加到索引
		key := fmt.Sprintf("%v", value)
		idx.valueToSeq[key] = append(idx.valueToSeq[key], seq)

		// 更新元数据
		if idx.metadata.MinSeq == 0 || seq < idx.metadata.MinSeq {
			idx.metadata.MinSeq = seq
		}
		if seq > idx.metadata.MaxSeq {
			idx.metadata.MaxSeq = seq
		}
		addedCount++
	}

	idx.metadata.RowCount += addedCount
	idx.metadata.UpdatedAt = time.Now().UnixNano()

	// 释放锁，然后调用 Build（Build 会重新获取锁）
	idx.mu.Unlock()

	// 保存更新后的索引
	return idx.Build()
}

// Close 关闭索引
func (idx *SecondaryIndex) Close() error {
	// 关闭 B+Tree reader
	if idx.btreeReader != nil {
		idx.btreeReader.Close()
	}
	// 关闭文件
	if idx.file != nil {
		return idx.file.Close()
	}
	return nil
}

// IndexManager 索引管理器
type IndexManager struct {
	dir     string
	schema  *Schema
	indexes map[string]*SecondaryIndex // field → index
	mu      sync.RWMutex
}

// NewIndexManager 创建索引管理器
func NewIndexManager(dir string, schema *Schema) *IndexManager {
	mgr := &IndexManager{
		dir:     dir,
		schema:  schema,
		indexes: make(map[string]*SecondaryIndex),
	}

	// 自动加载已存在的索引
	mgr.loadExistingIndexes()

	return mgr
}

// loadExistingIndexes 加载已存在的索引文件
func (m *IndexManager) loadExistingIndexes() error {
	// 确保目录存在
	if _, err := os.Stat(m.dir); os.IsNotExist(err) {
		return nil // 目录不存在，跳过
	}

	// 查找所有索引文件
	pattern := filepath.Join(m.dir, "idx_*.sst")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil // 忽略错误，继续
	}

	for _, filePath := range files {
		// 从文件名提取字段名
		// idx_name.sst -> name
		filename := filepath.Base(filePath)
		if len(filename) < 8 { // "idx_" (4) + ".sst" (4)
			continue
		}
		field := filename[4 : len(filename)-4] // 去掉 "idx_" 和 ".sst"

		// 检查字段是否在 Schema 中
		fieldDef, err := m.schema.GetField(field)
		if err != nil {
			continue // 跳过不在 Schema 中的索引
		}

		// 打开索引文件
		file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
		if err != nil {
			continue
		}

		// 创建索引对象
		idx := &SecondaryIndex{
			name:       field,
			field:      field,
			fieldType:  fieldDef.Type,
			file:       file,
			valueToSeq: make(map[string][]int64),
			ready:      false,
		}

		// 加载索引数据
		err = idx.load()
		if err != nil {
			file.Close()
			continue
		}

		m.indexes[field] = idx
	}

	return nil
}

// CreateIndex 创建索引
func (m *IndexManager) CreateIndex(field string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查字段是否存在
	fieldDef, err := m.schema.GetField(field)
	if err != nil {
		return err
	}

	// 检查是否已存在
	if _, exists := m.indexes[field]; exists {
		return fmt.Errorf("index on field %s already exists", field)
	}

	// 创建索引
	idx, err := NewSecondaryIndex(m.dir, field, fieldDef.Type)
	if err != nil {
		return err
	}

	m.indexes[field] = idx
	return nil
}

// DropIndex 删除索引
func (m *IndexManager) DropIndex(field string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx, exists := m.indexes[field]
	if !exists {
		return fmt.Errorf("index on field %s does not exist", field)
	}

	// 获取文件路径
	indexPath := filepath.Join(m.dir, fmt.Sprintf("idx_%s.sst", field))

	// 关闭索引
	idx.Close()

	// 删除索引文件
	os.Remove(indexPath)

	// 从内存中删除
	delete(m.indexes, field)

	return nil
}

// GetIndex 获取索引
func (m *IndexManager) GetIndex(field string) (*SecondaryIndex, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idx, exists := m.indexes[field]
	return idx, exists
}

// AddToIndexes 添加到所有索引
func (m *IndexManager) AddToIndexes(data map[string]any, seq int64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for field, idx := range m.indexes {
		if value, exists := data[field]; exists {
			err := idx.Add(value, seq)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// BuildAll 构建所有索引
func (m *IndexManager) BuildAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, idx := range m.indexes {
		err := idx.Build()
		if err != nil {
			return err
		}
	}

	return nil
}

// ListIndexes 列出所有索引
func (m *IndexManager) ListIndexes() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fields := make([]string, 0, len(m.indexes))
	for field := range m.indexes {
		fields = append(fields, field)
	}
	return fields
}

// VerifyAndRepair 验证并修复所有索引
func (m *IndexManager) VerifyAndRepair(currentMaxSeq int64, getData func(int64) (map[string]any, error)) error {
	m.mu.RLock()
	indexes := make(map[string]*SecondaryIndex)
	maps.Copy(indexes, m.indexes)
	m.mu.RUnlock()

	for field, idx := range indexes {
		// 检查是否需要更新
		if idx.NeedsUpdate(currentMaxSeq) {
			metadata := idx.GetMetadata()
			fromSeq := metadata.MaxSeq + 1
			toSeq := currentMaxSeq

			// 增量更新
			err := idx.IncrementalUpdate(getData, fromSeq, toSeq)
			if err != nil {
				return fmt.Errorf("failed to update index %s: %v", field, err)
			}
		}
	}

	return nil
}

// GetIndexMetadata 获取所有索引的元数据
func (m *IndexManager) GetIndexMetadata() map[string]IndexMetadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metadata := make(map[string]IndexMetadata)
	for field, idx := range m.indexes {
		metadata[field] = idx.GetMetadata()
	}
	return metadata
}

// Close 关闭所有索引
func (m *IndexManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, idx := range m.indexes {
		idx.Close()
	}

	return nil
}
