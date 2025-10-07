package memtable

import (
	"sync"
)

// ImmutableMemTable 不可变的 MemTable
type ImmutableMemTable struct {
	MemTable  *MemTable
	WALNumber int64 // 对应的 WAL 编号
}

// Manager MemTable 管理器
type Manager struct {
	active     *MemTable            // Active MemTable (可写)
	immutables []*ImmutableMemTable // Immutable MemTables (只读)
	activeWAL  int64                // Active MemTable 对应的 WAL 编号
	maxSize    int64                // MemTable 最大大小
	mu         sync.RWMutex         // 读写锁
}

// NewManager 创建 MemTable 管理器
func NewManager(maxSize int64) *Manager {
	return &Manager{
		active:     New(),
		immutables: make([]*ImmutableMemTable, 0),
		maxSize:    maxSize,
	}
}

// SetActiveWAL 设置 Active MemTable 对应的 WAL 编号
func (m *Manager) SetActiveWAL(walNumber int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeWAL = walNumber
}

// Put 写入数据到 Active MemTable
func (m *Manager) Put(key int64, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active.Put(key, value)
}

// Get 查询数据（先查 Active，再查 Immutables）
func (m *Manager) Get(key int64) ([]byte, bool) {
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
func (m *Manager) GetActiveSize() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active.Size()
}

// GetActiveCount 获取 Active MemTable 条目数
func (m *Manager) GetActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active.Count()
}

// ShouldSwitch 检查是否需要切换 MemTable
func (m *Manager) ShouldSwitch() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active.Size() >= m.maxSize
}

// Switch 切换 MemTable（Active → Immutable，创建新 Active）
// 返回：旧的 WAL 编号，新的 Active MemTable
func (m *Manager) Switch(newWALNumber int64) (oldWALNumber int64, immutable *ImmutableMemTable) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 将 Active 变为 Immutable
	immutable = &ImmutableMemTable{
		MemTable:  m.active,
		WALNumber: m.activeWAL,
	}
	m.immutables = append(m.immutables, immutable)

	// 2. 创建新的 Active MemTable
	m.active = New()
	oldWALNumber = m.activeWAL
	m.activeWAL = newWALNumber

	return oldWALNumber, immutable
}

// RemoveImmutable 移除指定的 Immutable MemTable
func (m *Manager) RemoveImmutable(target *ImmutableMemTable) {
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
func (m *Manager) GetImmutableCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.immutables)
}

// GetImmutables 获取所有 Immutable MemTables（副本）
func (m *Manager) GetImmutables() []*ImmutableMemTable {
	m.mu.RLock()
	defer m.mu.RUnlock()

	immutables := make([]*ImmutableMemTable, len(m.immutables))
	copy(immutables, m.immutables)
	return immutables
}

// GetActive 获取 Active MemTable（用于 Flush 时读取）
func (m *Manager) GetActive() *MemTable {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// TotalCount 获取总条目数（Active + Immutables）
func (m *Manager) TotalCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.active.Count()
	for _, imm := range m.immutables {
		total += imm.MemTable.Count()
	}
	return total
}

// TotalSize 获取总大小（Active + Immutables）
func (m *Manager) TotalSize() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.active.Size()
	for _, imm := range m.immutables {
		total += imm.MemTable.Size()
	}
	return total
}

// NewIterator 创建 Active MemTable 的迭代器
func (m *Manager) NewIterator() *Iterator {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active.NewIterator()
}

// Stats 统计信息
type Stats struct {
	ActiveSize      int64
	ActiveCount     int
	ImmutableCount  int
	ImmutablesSize  int64
	ImmutablesTotal int
	TotalSize       int64
	TotalCount      int
}

// GetStats 获取统计信息
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
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
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.active = New()
	m.immutables = make([]*ImmutableMemTable, 0)
}
