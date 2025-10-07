package sst

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Manager SST 文件管理器
type Manager struct {
	dir     string
	readers []*Reader
	mu      sync.RWMutex
}

// NewManager 创建 SST 管理器
func NewManager(dir string) (*Manager, error) {
	// 确保目录存在
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	mgr := &Manager{
		dir:     dir,
		readers: make([]*Reader, 0),
	}

	// 恢复现有的 SST 文件
	err = mgr.recover()
	if err != nil {
		return nil, err
	}

	return mgr, nil
}

// recover 恢复现有的 SST 文件
func (m *Manager) recover() error {
	// 查找所有 SST 文件
	files, err := filepath.Glob(filepath.Join(m.dir, "*.sst"))
	if err != nil {
		return err
	}

	for _, file := range files {
		// 跳过索引文件
		filename := filepath.Base(file)
		if strings.HasPrefix(filename, "idx_") {
			continue
		}

		// 打开 SST Reader
		reader, err := NewReader(file)
		if err != nil {
			return err
		}

		m.readers = append(m.readers, reader)
	}

	return nil
}

// CreateSST 创建新的 SST 文件
// fileNumber: 文件编号（由 VersionSet 分配）
func (m *Manager) CreateSST(fileNumber int64, rows []*Row) (*Reader, error) {
	return m.CreateSSTWithLevel(fileNumber, rows, 0) // 默认创建到 L0
}

// CreateSSTWithLevel 创建新的 SST 文件到指定层级
// fileNumber: 文件编号（由 VersionSet 分配）
func (m *Manager) CreateSSTWithLevel(fileNumber int64, rows []*Row, level int) (*Reader, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sstPath := filepath.Join(m.dir, fmt.Sprintf("%06d.sst", fileNumber))

	// 创建文件
	file, err := os.Create(sstPath)
	if err != nil {
		return nil, err
	}

	writer := NewWriter(file)

	// 写入所有行
	for _, row := range rows {
		err = writer.Add(row)
		if err != nil {
			file.Close()
			os.Remove(sstPath)
			return nil, err
		}
	}

	// 完成写入
	err = writer.Finish()
	if err != nil {
		file.Close()
		os.Remove(sstPath)
		return nil, err
	}

	file.Close()

	// 打开 SST Reader
	reader, err := NewReader(sstPath)
	if err != nil {
		return nil, err
	}

	// 添加到 readers 列表
	m.readers = append(m.readers, reader)

	return reader, nil
}

// Get 从所有 SST 文件中查找数据
func (m *Manager) Get(seq int64) (*Row, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 从后往前查找（新的文件优先）
	for i := len(m.readers) - 1; i >= 0; i-- {
		reader := m.readers[i]
		row, err := reader.Get(seq)
		if err == nil {
			return row, nil
		}
	}

	return nil, fmt.Errorf("key not found: %d", seq)
}

// GetReaders 获取所有 Readers（用于扫描）
func (m *Manager) GetReaders() []*Reader {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	readers := make([]*Reader, len(m.readers))
	copy(readers, m.readers)
	return readers
}

// GetMaxSeq 获取所有 SST 中的最大 seq
func (m *Manager) GetMaxSeq() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	maxSeq := int64(0)
	for _, reader := range m.readers {
		header := reader.GetHeader()
		if header.MaxKey > maxSeq {
			maxSeq = header.MaxKey
		}
	}

	return maxSeq
}

// Count 获取 SST 文件数量
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.readers)
}

// ListFiles 列出所有 SST 文件
func (m *Manager) ListFiles() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := make([]string, 0, len(m.readers))
	for _, reader := range m.readers {
		files = append(files, reader.path)
	}

	return files
}

// CompactionConfig Compaction 配置
// 已废弃：请使用 compaction 包中的 Manager
type CompactionConfig struct {
	Threshold int // 触发阈值（SST 文件数量）
	BatchSize int // 每次合并的文件数量
}

// DefaultCompactionConfig 默认配置
// 已废弃：请使用 compaction 包中的 Manager
var DefaultCompactionConfig = CompactionConfig{
	Threshold: 10,
	BatchSize: 10,
}

// ShouldCompact 检查是否需要 Compaction
// 已废弃：请使用 compaction 包中的 Manager
func (m *Manager) ShouldCompact(config CompactionConfig) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.readers) > config.Threshold
}

// Compact 执行 Compaction
// 已废弃：请使用 compaction 包中的 Manager
// 注意：此方法已不再维护，不应在新代码中使用
func (m *Manager) Compact(config CompactionConfig) error {
	// 此方法已废弃，不再实现
	return fmt.Errorf("Compact is deprecated, please use compaction.Manager")
}

// sortRows 按 seq 排序
func sortRows(rows []*Row) {
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Seq < rows[j].Seq
	})
}

// Delete 删除指定的 SST 文件（预留接口）
func (m *Manager) Delete(fileNumber int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sstPath := filepath.Join(m.dir, fmt.Sprintf("%06d.sst", fileNumber))
	return os.Remove(sstPath)
}

// Close 关闭所有 SST Readers
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, reader := range m.readers {
		reader.Close()
	}

	m.readers = nil
	return nil
}

// Stats 统计信息
type Stats struct {
	FileCount int
	TotalSize int64
	MinSeq    int64
	MaxSeq    int64
}

// GetStats 获取统计信息
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		FileCount: len(m.readers),
		MinSeq:    -1,
		MaxSeq:    -1,
	}

	for _, reader := range m.readers {
		header := reader.GetHeader()

		if stats.MinSeq == -1 || header.MinKey < stats.MinSeq {
			stats.MinSeq = header.MinKey
		}

		if stats.MaxSeq == -1 || header.MaxKey > stats.MaxSeq {
			stats.MaxSeq = header.MaxKey
		}

		// 获取文件大小
		if stat, err := os.Stat(reader.path); err == nil {
			stats.TotalSize += stat.Size()
		}
	}

	return stats
}
