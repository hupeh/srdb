package srdb

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	// Entry 类型
	WALEntryTypePut    = 1
	WALEntryTypeDelete = 2 // 预留，暂不支持

	// Entry Header 大小
	WALEntryHeaderSize = 17 // CRC32(4) + Length(4) + Type(1) + Seq(8)
)

// WALEntry WAL 条目
type WALEntry struct {
	Type  byte   // 操作类型
	Seq   int64  // _seq
	Data  []byte // 数据
	CRC32 uint32 // 校验和
}

// WAL Write-Ahead Log
type WAL struct {
	file   *os.File
	offset int64
	mu     sync.Mutex
}

// OpenWAL 打开 WAL 文件
func OpenWAL(path string) (*WAL, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	// 获取当前文件大小
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	return &WAL{
		file:   file,
		offset: stat.Size(),
	}, nil
}

// Append 追加一条记录
func (w *WAL) Append(entry *WALEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 序列化 Entry
	data := w.marshalEntry(entry)

	// 写入文件
	_, err := w.file.Write(data)
	if err != nil {
		return err
	}

	w.offset += int64(len(data))

	return nil
}

// Sync 同步到磁盘
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.file.Sync()
}

// Close 关闭 WAL
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.file.Close()
}

// Truncate 清空 WAL
func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 在 Windows 上，带 O_APPEND 标志的文件不能直接 truncate
	// 需要先关闭，重新打开（不带 O_APPEND），truncate，再重新打开
	path := w.file.Name()

	// 关闭当前文件
	w.file.Close()

	// 以读写模式打开（不带 O_APPEND）
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return err
	}

	// Truncate
	err = file.Truncate(0)
	if err != nil {
		file.Close()
		return err
	}

	file.Close()

	// 重新以 APPEND 模式打开
	file, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	w.file = file
	w.offset = 0
	return nil
}

// marshalEntry 序列化 Entry
func (w *WAL) marshalEntry(entry *WALEntry) []byte {
	dataLen := len(entry.Data)
	totalLen := WALEntryHeaderSize + dataLen

	buf := make([]byte, totalLen)

	// 计算 CRC32 (不包括 CRC32 字段本身)
	crcData := buf[4:totalLen]
	binary.LittleEndian.PutUint32(crcData[0:4], uint32(dataLen))
	crcData[4] = entry.Type
	binary.LittleEndian.PutUint64(crcData[5:13], uint64(entry.Seq))
	copy(crcData[13:], entry.Data)

	crc := crc32.ChecksumIEEE(crcData)

	// 写入 CRC32
	binary.LittleEndian.PutUint32(buf[0:4], crc)

	return buf
}

// WALReader WAL 读取器
type WALReader struct {
	file *os.File
}

// NewWALReader 创建 WAL 读取器
func NewWALReader(path string) (*WALReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &WALReader{
		file: file,
	}, nil
}

// Read 读取所有 Entry
func (r *WALReader) Read() ([]*WALEntry, error) {
	var entries []*WALEntry

	for {
		entry, err := r.readEntry()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// Close 关闭读取器
func (r *WALReader) Close() error {
	return r.file.Close()
}

// readEntry 读取一条 Entry
func (r *WALReader) readEntry() (*WALEntry, error) {
	// 读取 Header
	header := make([]byte, WALEntryHeaderSize)
	_, err := io.ReadFull(r.file, header)
	if err != nil {
		return nil, err
	}

	// 解析 Header
	crc := binary.LittleEndian.Uint32(header[0:4])
	dataLen := binary.LittleEndian.Uint32(header[4:8])
	entryType := header[8]
	seq := int64(binary.LittleEndian.Uint64(header[9:17]))

	// 读取 Data
	data := make([]byte, dataLen)
	_, err = io.ReadFull(r.file, data)
	if err != nil {
		return nil, err
	}

	// 验证 CRC32
	crcData := make([]byte, WALEntryHeaderSize-4+int(dataLen))
	copy(crcData[0:WALEntryHeaderSize-4], header[4:])
	copy(crcData[WALEntryHeaderSize-4:], data)

	if crc32.ChecksumIEEE(crcData) != crc {
		return nil, io.ErrUnexpectedEOF // CRC 校验失败
	}

	return &WALEntry{
		Type:  entryType,
		Seq:   seq,
		Data:  data,
		CRC32: crc,
	}, nil
}

// WALManager WAL 管理器，管理多个 WAL 文件
type WALManager struct {
	dir           string
	currentWAL    *WAL
	currentNumber int64
	mu            sync.Mutex
}

// NewWALManager 创建 WAL 管理器
func NewWALManager(dir string) (*WALManager, error) {
	// 确保目录存在
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	// 读取当前 WAL 编号
	number, err := readWALCurrentNumber(dir)
	if err != nil {
		// 如果读取失败，从 1 开始
		number = 1
	}

	// 打开当前 WAL
	walPath := filepath.Join(dir, fmt.Sprintf("%06d.wal", number))
	wal, err := OpenWAL(walPath)
	if err != nil {
		return nil, err
	}

	// 保存当前编号
	err = saveWALCurrentNumber(dir, number)
	if err != nil {
		wal.Close()
		return nil, err
	}

	return &WALManager{
		dir:           dir,
		currentWAL:    wal,
		currentNumber: number,
	}, nil
}

// Append 追加记录到当前 WAL
func (m *WALManager) Append(entry *WALEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.currentWAL.Append(entry)
}

// Sync 同步当前 WAL 到磁盘
func (m *WALManager) Sync() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.currentWAL.Sync()
}

// Rotate 切换到新的 WAL 文件
func (m *WALManager) Rotate() (int64, error) {
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
	wal, err := OpenWAL(walPath)
	if err != nil {
		return 0, err
	}

	m.currentWAL = wal

	// 更新 CURRENT 文件
	err = saveWALCurrentNumber(m.dir, m.currentNumber)
	if err != nil {
		return 0, err
	}

	return oldNumber, nil
}

// Delete 删除指定的 WAL 文件
func (m *WALManager) Delete(number int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	walPath := filepath.Join(m.dir, fmt.Sprintf("%06d.wal", number))
	return os.Remove(walPath)
}

// GetCurrentNumber 获取当前 WAL 编号
func (m *WALManager) GetCurrentNumber() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.currentNumber
}

// RecoverAll 恢复所有 WAL 文件
func (m *WALManager) RecoverAll() ([]*WALEntry, error) {
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

	var allEntries []*WALEntry

	// 依次读取每个 WAL
	for _, file := range files {
		reader, err := NewWALReader(file)
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
func (m *WALManager) ListWALFiles() ([]string, error) {
	pattern := filepath.Join(m.dir, "*.wal")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

// Close 关闭 WAL 管理器
func (m *WALManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentWAL != nil {
		return m.currentWAL.Close()
	}

	return nil
}

// readWALCurrentNumber 读取当前 WAL 编号
func readWALCurrentNumber(dir string) (int64, error) {
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

// saveWALCurrentNumber 保存当前 WAL 编号
func saveWALCurrentNumber(dir string, number int64) error {
	currentPath := filepath.Join(dir, "CURRENT")
	data := fmt.Appendf(nil, "%d\n", number)
	return os.WriteFile(currentPath, data, 0644)
}
