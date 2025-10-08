package srdb

import (
	"slices"
	"sync"
)

// MemTable 内存表
type MemTable struct {
	data map[int64][]byte // key -> value
	keys []int64          // 排序的 keys
	size int64            // 数据大小
	mu   sync.RWMutex
}

// NewMemTable 创建 MemTable
func NewMemTable() *MemTable {
	return &MemTable{
		data: make(map[int64][]byte),
		keys: make([]int64, 0),
		size: 0,
	}
}

// Put 插入数据
func (m *MemTable) Put(key int64, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	if _, exists := m.data[key]; !exists {
		m.keys = append(m.keys, key)
		// 保持 keys 有序
		slices.Sort(m.keys)
	}

	m.data[key] = value
	m.size += int64(len(value))
}

// Get 查询数据
func (m *MemTable) Get(key int64) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, exists := m.data[key]
	return value, exists
}

// Size 获取大小
func (m *MemTable) Size() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.size
}

// Count 获取条目数量
func (m *MemTable) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.data)
}

// Keys 获取所有 keys 的副本（已排序）
func (m *MemTable) Keys() []int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本以避免并发问题
	keysCopy := make([]int64, len(m.keys))
	copy(keysCopy, m.keys)
	return keysCopy
}

// MemTableIterator 迭代器
type MemTableIterator struct {
	mt    *MemTable
	index int
}

// NewIterator 创建迭代器
func (m *MemTable) NewIterator() *MemTableIterator {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &MemTableIterator{
		mt:    m,
		index: -1,
	}
}

// Next 移动到下一个
func (it *MemTableIterator) Next() bool {
	it.mt.mu.RLock()
	defer it.mt.mu.RUnlock()

	it.index++
	return it.index < len(it.mt.keys)
}

// Key 当前 key
func (it *MemTableIterator) Key() int64 {
	it.mt.mu.RLock()
	defer it.mt.mu.RUnlock()

	if it.index < 0 || it.index >= len(it.mt.keys) {
		return 0
	}
	return it.mt.keys[it.index]
}

// Value 当前 value
func (it *MemTableIterator) Value() []byte {
	it.mt.mu.RLock()
	defer it.mt.mu.RUnlock()

	if it.index < 0 || it.index >= len(it.mt.keys) {
		return nil
	}
	key := it.mt.keys[it.index]
	return it.mt.data[key]
}

// Reset 重置迭代器
func (it *MemTableIterator) Reset() {
	it.index = -1
}

// Clear 清空 MemTable
func (m *MemTable) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[int64][]byte)
	m.keys = make([]int64, 0)
	m.size = 0
}

// ImmutableMemTable 不可变的 MemTable
type ImmutableMemTable struct {
	*MemTable
	WALNumber int64 // 对应的 WAL 编号
}

// MemTableManager MemTable 管理器
type MemTableManager struct {
	active     *MemTable            // Active MemTable (可写)
	immutables []*ImmutableMemTable // Immutable MemTables (只读)
	activeWAL  int64                // Active MemTable 对应的 WAL 编号
	maxSize    int64                // MemTable 最大大小
	mu         sync.RWMutex         // 读写锁
}

// NewMemTableManager 创建 MemTable 管理器
func NewMemTableManager(maxSize int64) *MemTableManager {
	return &MemTableManager{
		active:     NewMemTable(),
		immutables: make([]*ImmutableMemTable, 0),
		maxSize:    maxSize,
	}
}

// SetActiveWAL 设置 Active MemTable 对应的 WAL 编号
func (m *MemTableManager) SetActiveWAL(walNumber int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeWAL = walNumber
}

// Put 写入数据到 Active MemTable
func (m *MemTableManager) Put(key int64, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active.Put(key, value)
}

// Get 查询数据（先查 Active，再查 Immutables）
func (m *MemTableManager) Get(key int64) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. 先查 Active MemTable
	if value, found := m.active.Get(key); found {
		return value, true
	}

	// 2. 查 Immutable MemTables（从新到旧）
	for i := len(m.immutables) - 1; i >= 0; i-- {
		if value, found := m.immutables[i].MemTable.Get(key); found {
			return value, true
		}
	}

	return nil, false
}

// GetActiveSize 获取 Active MemTable 大小
func (m *MemTableManager) GetActiveSize() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active.Size()
}

// GetActiveCount 获取 Active MemTable 条目数
func (m *MemTableManager) GetActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active.Count()
}

// ShouldSwitch 检查是否需要切换 MemTable
func (m *MemTableManager) ShouldSwitch() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active.Size() >= m.maxSize
}

// Switch 切换 MemTable（Active → Immutable，创建新 Active）
// 返回：旧的 WAL 编号，新的 Active MemTable
func (m *MemTableManager) Switch(newWALNumber int64) (oldWALNumber int64, immutable *ImmutableMemTable) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 将 Active 变为 Immutable
	immutable = &ImmutableMemTable{
		MemTable:  m.active,
		WALNumber: m.activeWAL,
	}
	m.immutables = append(m.immutables, immutable)

	// 2. 创建新的 Active MemTable
	m.active = NewMemTable()
	oldWALNumber = m.activeWAL
	m.activeWAL = newWALNumber

	return oldWALNumber, immutable
}

// RemoveImmutable 移除指定的 Immutable MemTable
func (m *MemTableManager) RemoveImmutable(target *ImmutableMemTable) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找并移除
	for i, imm := range m.immutables {
		if imm == target {
			m.immutables = append(m.immutables[:i], m.immutables[i+1:]...)
			break
		}
	}
}

// GetImmutableCount 获取 Immutable MemTable 数量
func (m *MemTableManager) GetImmutableCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.immutables)
}

// GetImmutables 获取所有 Immutable MemTables（副本）
func (m *MemTableManager) GetImmutables() []*ImmutableMemTable {
	m.mu.RLock()
	defer m.mu.RUnlock()

	immutables := make([]*ImmutableMemTable, len(m.immutables))
	copy(immutables, m.immutables)
	return immutables
}

// GetActive 获取 Active MemTable（用于 Flush 时读取）
func (m *MemTableManager) GetActive() *MemTable {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// TotalCount 获取总条目数（Active + Immutables）
func (m *MemTableManager) TotalCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.active.Count()
	for _, imm := range m.immutables {
		total += imm.MemTable.Count()
	}
	return total
}

// TotalSize 获取总大小（Active + Immutables）
func (m *MemTableManager) TotalSize() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.active.Size()
	for _, imm := range m.immutables {
		total += imm.MemTable.Size()
	}
	return total
}

// NewIterator 创建 Active MemTable 的迭代器
func (m *MemTableManager) NewIterator() *MemTableIterator {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active.NewIterator()
}

// MemTableStats 统计信息
type MemTableStats struct {
	ActiveSize      int64
	ActiveCount     int
	ImmutableCount  int
	ImmutablesSize  int64
	ImmutablesTotal int
	TotalSize       int64
	TotalCount      int
}

// GetStats 获取统计信息
func (m *MemTableManager) GetStats() *MemTableStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &MemTableStats{
		ActiveSize:     m.active.Size(),
		ActiveCount:    m.active.Count(),
		ImmutableCount: len(m.immutables),
	}

	for _, imm := range m.immutables {
		stats.ImmutablesSize += imm.MemTable.Size()
		stats.ImmutablesTotal += imm.MemTable.Count()
	}

	stats.TotalSize = stats.ActiveSize + stats.ImmutablesSize
	stats.TotalCount = stats.ActiveCount + stats.ImmutablesTotal

	return stats
}

// Clear 清空所有 MemTables（用于测试）
func (m *MemTableManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.active = NewMemTable()
	m.immutables = make([]*ImmutableMemTable, 0)
}
