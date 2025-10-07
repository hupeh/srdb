package memtable

import (
	"sort"
	"sync"
)

// MemTable 内存表
type MemTable struct {
	data map[int64][]byte // key -> value
	keys []int64          // 排序的 keys
	size int64            // 数据大小
	mu   sync.RWMutex
}

// New 创建 MemTable
func New() *MemTable {
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
		sort.Slice(m.keys, func(i, j int) bool {
			return m.keys[i] < m.keys[j]
		})
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

// Iterator 迭代器
type Iterator struct {
	mt    *MemTable
	index int
}

// NewIterator 创建迭代器
func (m *MemTable) NewIterator() *Iterator {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &Iterator{
		mt:    m,
		index: -1,
	}
}

// Next 移动到下一个
func (it *Iterator) Next() bool {
	it.mt.mu.RLock()
	defer it.mt.mu.RUnlock()

	it.index++
	return it.index < len(it.mt.keys)
}

// Key 当前 key
func (it *Iterator) Key() int64 {
	it.mt.mu.RLock()
	defer it.mt.mu.RUnlock()

	if it.index < 0 || it.index >= len(it.mt.keys) {
		return 0
	}
	return it.mt.keys[it.index]
}

// Value 当前 value
func (it *Iterator) Value() []byte {
	it.mt.mu.RLock()
	defer it.mt.mu.RUnlock()

	if it.index < 0 || it.index >= len(it.mt.keys) {
		return nil
	}
	key := it.mt.keys[it.index]
	return it.mt.data[key]
}

// Reset 重置迭代器
func (it *Iterator) Reset() {
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
