package wal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Manager WAL 管理器，管理多个 WAL 文件
type Manager struct {
	dir           string
	currentWAL    *WAL
	currentNumber int64
	mu            sync.Mutex
}

// NewManager 创建 WAL 管理器
func NewManager(dir string) (*Manager, error) {
	// 确保目录存在
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	// 读取当前 WAL 编号
	number, err := readCurrentNumber(dir)
	if err != nil {
		// 如果读取失败，从 1 开始
		number = 1
	}

	// 打开当前 WAL
	walPath := filepath.Join(dir, fmt.Sprintf("%06d.wal", number))
	wal, err := Open(walPath)
	if err != nil {
		return nil, err
	}

	// 保存当前编号
	err = saveCurrentNumber(dir, number)
	if err != nil {
		wal.Close()
		return nil, err
	}

	return &Manager{
		dir:           dir,
		currentWAL:    wal,
		currentNumber: number,
	}, nil
}

// Append 追加记录到当前 WAL
func (m *Manager) Append(entry *Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.currentWAL.Append(entry)
}

// Sync 同步当前 WAL 到磁盘
func (m *Manager) Sync() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.currentWAL.Sync()
}

// Rotate 切换到新的 WAL 文件
func (m *Manager) Rotate() (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 记录旧的 WAL 编号
	oldNumber := m.currentNumber

	// 关闭当前 WAL
	err := m.currentWAL.Close()
	if err != nil {
		return 0, err
	}

	// 创建新 WAL
	m.currentNumber++
	walPath := filepath.Join(m.dir, fmt.Sprintf("%06d.wal", m.currentNumber))
	wal, err := Open(walPath)
	if err != nil {
		return 0, err
	}

	m.currentWAL = wal

	// 更新 CURRENT 文件
	err = saveCurrentNumber(m.dir, m.currentNumber)
	if err != nil {
		return 0, err
	}

	return oldNumber, nil
}

// Delete 删除指定的 WAL 文件
func (m *Manager) Delete(number int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	walPath := filepath.Join(m.dir, fmt.Sprintf("%06d.wal", number))
	return os.Remove(walPath)
}

// GetCurrentNumber 获取当前 WAL 编号
func (m *Manager) GetCurrentNumber() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.currentNumber
}

// RecoverAll 恢复所有 WAL 文件
func (m *Manager) RecoverAll() ([]*Entry, error) {
	// 查找所有 WAL 文件
	pattern := filepath.Join(m.dir, "*.wal")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, nil
	}

	// 按文件名排序（确保按时间顺序）
	sort.Strings(files)

	var allEntries []*Entry

	// 依次读取每个 WAL
	for _, file := range files {
		reader, err := NewReader(file)
		if err != nil {
			continue
		}

		entries, err := reader.Read()
		reader.Close()

		if err != nil {
			continue
		}

		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}

// ListWALFiles 列出所有 WAL 文件
func (m *Manager) ListWALFiles() ([]string, error) {
	pattern := filepath.Join(m.dir, "*.wal")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

// Close 关闭 WAL 管理器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentWAL != nil {
		return m.currentWAL.Close()
	}

	return nil
}

// readCurrentNumber 读取当前 WAL 编号
func readCurrentNumber(dir string) (int64, error) {
	currentPath := filepath.Join(dir, "CURRENT")
	data, err := os.ReadFile(currentPath)
	if err != nil {
		return 0, err
	}

	number, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, err
	}

	return number, nil
}

// saveCurrentNumber 保存当前 WAL 编号
func saveCurrentNumber(dir string, number int64) error {
	currentPath := filepath.Join(dir, "CURRENT")
	data := []byte(fmt.Sprintf("%d\n", number))
	return os.WriteFile(currentPath, data, 0644)
}
