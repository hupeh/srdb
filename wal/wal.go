package wal

import (
	"encoding/binary"
	"hash/crc32"
	"io"
	"os"
	"sync"
)

const (
	// Entry 类型
	EntryTypePut    = 1
	EntryTypeDelete = 2 // 预留，暂不支持

	// Entry Header 大小
	EntryHeaderSize = 17 // CRC32(4) + Length(4) + Type(1) + Seq(8)
)

// Entry WAL 条目
type Entry struct {
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

// Open 打开 WAL 文件
func Open(path string) (*WAL, error) {
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
func (w *WAL) Append(entry *Entry) error {
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

	err := w.file.Truncate(0)
	if err != nil {
		return err
	}

	_, err = w.file.Seek(0, 0)
	if err != nil {
		return err
	}

	w.offset = 0
	return nil
}

// marshalEntry 序列化 Entry
func (w *WAL) marshalEntry(entry *Entry) []byte {
	dataLen := len(entry.Data)
	totalLen := EntryHeaderSize + dataLen

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

// Reader WAL 读取器
type Reader struct {
	file *os.File
}

// NewReader 创建 WAL 读取器
func NewReader(path string) (*Reader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &Reader{
		file: file,
	}, nil
}

// Read 读取所有 Entry
func (r *Reader) Read() ([]*Entry, error) {
	var entries []*Entry

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
func (r *Reader) Close() error {
	return r.file.Close()
}

// readEntry 读取一条 Entry
func (r *Reader) readEntry() (*Entry, error) {
	// 读取 Header
	header := make([]byte, EntryHeaderSize)
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
	crcData := make([]byte, EntryHeaderSize-4+int(dataLen))
	copy(crcData[0:EntryHeaderSize-4], header[4:])
	copy(crcData[EntryHeaderSize-4:], data)

	if crc32.ChecksumIEEE(crcData) != crc {
		return nil, io.ErrUnexpectedEOF // CRC 校验失败
	}

	return &Entry{
		Type:  entryType,
		Seq:   seq,
		Data:  data,
		CRC32: crc,
	}, nil
}
