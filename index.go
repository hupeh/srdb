package srdb

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
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
	name       string             // 索引名称
	field      string             // 字段名
	fieldType  FieldType          // 字段类型
	file       *os.File           // 索引文件
	builder    *BTreeBuilder      // B+Tree 构建器
	reader     *BTreeReader       // B+Tree 读取器
	valueToSeq map[string][]int64 // 值 → seq 列表 (构建时使用)
	metadata   IndexMetadata      // 元数据
	mu         sync.RWMutex
	ready      bool // 索引是否就绪
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

// Add 添加索引条目
func (idx *SecondaryIndex) Add(value any, seq int64) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 将值转换为字符串作为 key
	key := fmt.Sprintf("%v", value)
	idx.valueToSeq[key] = append(idx.valueToSeq[key], seq)

	return nil
}

// Build 构建索引并持久化
func (idx *SecondaryIndex) Build() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// 持久化索引数据到 JSON 文件
	return idx.save()
}

// save 保存索引到磁盘
func (idx *SecondaryIndex) save() error {
	// 更新元数据
	idx.updateMetadata()

	// 创建包含元数据的数据结构
	indexData := struct {
		Metadata   IndexMetadata      `json:"metadata"`
		ValueToSeq map[string][]int64 `json:"data"`
	}{
		Metadata:   idx.metadata,
		ValueToSeq: idx.valueToSeq,
	}

	// 序列化索引数据
	data, err := json.Marshal(indexData)
	if err != nil {
		return err
	}

	// Truncate 文件
	err = idx.file.Truncate(0)
	if err != nil {
		return err
	}

	// 写入文件
	_, err = idx.file.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = idx.file.Write(data)
	if err != nil {
		return err
	}

	// Sync 到磁盘
	err = idx.file.Sync()
	if err != nil {
		return err
	}

	idx.ready = true
	return nil
}

// updateMetadata 更新元数据
func (idx *SecondaryIndex) updateMetadata() {
	now := time.Now().UnixNano()

	if idx.metadata.CreatedAt == 0 {
		idx.metadata.CreatedAt = now
	}
	idx.metadata.UpdatedAt = now
	idx.metadata.Version++

	// 计算 MinSeq, MaxSeq, RowCount
	var minSeq, maxSeq int64 = -1, -1
	rowCount := int64(0)

	for _, seqs := range idx.valueToSeq {
		for _, seq := range seqs {
			if minSeq == -1 || seq < minSeq {
				minSeq = seq
			}
			if maxSeq == -1 || seq > maxSeq {
				maxSeq = seq
			}
			rowCount++
		}
	}

	idx.metadata.MinSeq = minSeq
	idx.metadata.MaxSeq = maxSeq
	idx.metadata.RowCount = rowCount
}

// load 从磁盘加载索引
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

	// 读取文件内容
	data := make([]byte, stat.Size())
	_, err = idx.file.ReadAt(data, 0)
	if err != nil {
		return err
	}

	// 尝试加载新格式（带元数据）
	var indexData struct {
		Metadata   IndexMetadata      `json:"metadata"`
		ValueToSeq map[string][]int64 `json:"data"`
	}

	err = json.Unmarshal(data, &indexData)
	if err == nil && indexData.ValueToSeq != nil {
		// 新格式
		idx.metadata = indexData.Metadata
		idx.valueToSeq = indexData.ValueToSeq
	} else {
		// 旧格式（兼容性）
		err = json.Unmarshal(data, &idx.valueToSeq)
		if err != nil {
			return err
		}
		// 初始化元数据
		idx.updateMetadata()
	}

	idx.ready = true
	return nil
}

// Get 查询索引
func (idx *SecondaryIndex) Get(value any) ([]int64, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if !idx.ready {
		return nil, fmt.Errorf("index not ready")
	}

	key := fmt.Sprintf("%v", value)
	seqs, exists := idx.valueToSeq[key]
	if !exists {
		return nil, nil
	}

	return seqs, nil
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

// NeedsUpdate 检查是否需要更新
func (idx *SecondaryIndex) NeedsUpdate(currentMaxSeq int64) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.metadata.MaxSeq < currentMaxSeq
}

// IncrementalUpdate 增量更新索引
func (idx *SecondaryIndex) IncrementalUpdate(getData func(int64) (map[string]any, error), fromSeq, toSeq int64) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

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
	}

	// 保存更新后的索引
	return idx.save()
}

// Close 关闭索引
func (idx *SecondaryIndex) Close() error {
	if idx.file != nil {
		return idx.file.Close()
	}
	return nil
}

// encodeSeqList 编码 seq 列表
func encodeSeqList(seqs []int64) []byte {
	buf := make([]byte, 8*len(seqs))
	for i, seq := range seqs {
		binary.LittleEndian.PutUint64(buf[i*8:], uint64(seq))
	}
	return buf
}

// decodeSeqList 解码 seq 列表
func decodeSeqList(data []byte) []int64 {
	count := len(data) / 8
	seqs := make([]int64, count)
	for i := range count {
		seqs[i] = int64(binary.LittleEndian.Uint64(data[i*8:]))
	}
	return seqs
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
	for k, v := range m.indexes {
		indexes[k] = v
	}
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
