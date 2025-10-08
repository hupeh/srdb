package srdb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/edsrzf/mmap-go"
)

const (
	// 文件格式
	SSTableMagicNumber = 0x53535433 // "SST3"
	SSTableVersion     = 1
	SSTableHeaderSize  = 256       // 文件头大小
	SSTableBlockSize   = 64 * 1024 // 数据块大小 (64 KB)

	// 二进制编码格式:
	// [Magic: 4 bytes][Seq: 8 bytes][Time: 8 bytes][DataLen: 4 bytes][Data: variable]
	SSTableRowMagic = 0x524F5731 // "ROW1"
)

// SSTableHeader SST 文件头 (256 bytes)
type SSTableHeader struct {
	// 基础信息 (32 bytes)
	Magic       uint32 // Magic Number: 0x53535433
	Version     uint32 // 版本号
	Compression uint8  // 压缩类型（保留字段用于向后兼容）
	Reserved1   [3]byte
	Flags       uint32 // 标志位
	Reserved2   [16]byte

	// 索引信息 (32 bytes)
	IndexOffset int64 // B+Tree 索引起始位置
	IndexSize   int64 // B+Tree 索引大小
	RootOffset  int64 // B+Tree 根节点位置
	Reserved3   [8]byte

	// 数据信息 (32 bytes)
	DataOffset int64 // 数据块起始位置
	DataSize   int64 // 数据块总大小
	RowCount   int64 // 行数
	Reserved4  [8]byte

	// 统计信息 (32 bytes)
	MinKey  int64 // 最小 key (_seq)
	MaxKey  int64 // 最大 key (_seq)
	MinTime int64 // 最小时间戳
	MaxTime int64 // 最大时间戳

	// CRC 校验 (8 bytes)
	CRC32     uint32 // Header CRC32
	Reserved5 [4]byte

	// 预留空间 (120 bytes)
	Reserved6 [120]byte
}

// Marshal 序列化 Header
func (h *SSTableHeader) Marshal() []byte {
	buf := make([]byte, SSTableHeaderSize)

	// 基础信息
	binary.LittleEndian.PutUint32(buf[0:4], h.Magic)
	binary.LittleEndian.PutUint32(buf[4:8], h.Version)
	buf[8] = h.Compression
	copy(buf[9:12], h.Reserved1[:])
	binary.LittleEndian.PutUint32(buf[12:16], h.Flags)
	copy(buf[16:32], h.Reserved2[:])

	// 索引信息
	binary.LittleEndian.PutUint64(buf[32:40], uint64(h.IndexOffset))
	binary.LittleEndian.PutUint64(buf[40:48], uint64(h.IndexSize))
	binary.LittleEndian.PutUint64(buf[48:56], uint64(h.RootOffset))
	copy(buf[56:64], h.Reserved3[:])

	// 数据信息
	binary.LittleEndian.PutUint64(buf[64:72], uint64(h.DataOffset))
	binary.LittleEndian.PutUint64(buf[72:80], uint64(h.DataSize))
	binary.LittleEndian.PutUint64(buf[80:88], uint64(h.RowCount))
	copy(buf[88:96], h.Reserved4[:])

	// 统计信息
	binary.LittleEndian.PutUint64(buf[96:104], uint64(h.MinKey))
	binary.LittleEndian.PutUint64(buf[104:112], uint64(h.MaxKey))
	binary.LittleEndian.PutUint64(buf[112:120], uint64(h.MinTime))
	binary.LittleEndian.PutUint64(buf[120:128], uint64(h.MaxTime))

	// CRC 校验
	binary.LittleEndian.PutUint32(buf[128:132], h.CRC32)
	copy(buf[132:136], h.Reserved5[:])

	// 预留空间
	copy(buf[136:256], h.Reserved6[:])

	return buf
}

// Unmarshal 反序列化 Header
func UnmarshalSSTableHeader(data []byte) *SSTableHeader {
	if len(data) < SSTableHeaderSize {
		return nil
	}

	h := &SSTableHeader{}

	// 基础信息
	h.Magic = binary.LittleEndian.Uint32(data[0:4])
	h.Version = binary.LittleEndian.Uint32(data[4:8])
	h.Compression = data[8]
	copy(h.Reserved1[:], data[9:12])
	h.Flags = binary.LittleEndian.Uint32(data[12:16])
	copy(h.Reserved2[:], data[16:32])

	// 索引信息
	h.IndexOffset = int64(binary.LittleEndian.Uint64(data[32:40]))
	h.IndexSize = int64(binary.LittleEndian.Uint64(data[40:48]))
	h.RootOffset = int64(binary.LittleEndian.Uint64(data[48:56]))
	copy(h.Reserved3[:], data[56:64])

	// 数据信息
	h.DataOffset = int64(binary.LittleEndian.Uint64(data[64:72]))
	h.DataSize = int64(binary.LittleEndian.Uint64(data[72:80]))
	h.RowCount = int64(binary.LittleEndian.Uint64(data[80:88]))
	copy(h.Reserved4[:], data[88:96])

	// 统计信息
	h.MinKey = int64(binary.LittleEndian.Uint64(data[96:104]))
	h.MaxKey = int64(binary.LittleEndian.Uint64(data[104:112]))
	h.MinTime = int64(binary.LittleEndian.Uint64(data[112:120]))
	h.MaxTime = int64(binary.LittleEndian.Uint64(data[120:128]))

	// CRC 校验
	h.CRC32 = binary.LittleEndian.Uint32(data[128:132])
	copy(h.Reserved5[:], data[132:136])

	// 预留空间
	copy(h.Reserved6[:], data[136:256])

	return h
}

// Validate 验证 Header
func (h *SSTableHeader) Validate() bool {
	return h.Magic == SSTableMagicNumber && h.Version == SSTableVersion
}

// encodeSSTableRowBinary 使用二进制格式编码行数据（按字段压缩）
func encodeSSTableRowBinary(row *SSTableRow, schema *Schema) ([]byte, error) {
	buf := new(bytes.Buffer)

	// 写入 Magic Number (用于验证)
	if err := binary.Write(buf, binary.LittleEndian, uint32(SSTableRowMagic)); err != nil {
		return nil, err
	}

	// 写入 Seq
	if err := binary.Write(buf, binary.LittleEndian, row.Seq); err != nil {
		return nil, err
	}

	// 写入 Time
	if err := binary.Write(buf, binary.LittleEndian, row.Time); err != nil {
		return nil, err
	}

	// 强制要求 Schema
	if schema == nil {
		return nil, fmt.Errorf("schema is required for encoding SSTable rows")
	}

	// 按字段分别编码和压缩
	fieldCount := uint16(len(schema.Fields))
	if err := binary.Write(buf, binary.LittleEndian, fieldCount); err != nil {
		return nil, err
	}

	// 1. 先编码所有字段到各自的 buffer（无压缩）
	fieldData := make([][]byte, len(schema.Fields))

	for i, field := range schema.Fields {
		fieldBuf := new(bytes.Buffer)
		value, exists := row.Data[field.Name]

		if !exists {
			// 字段不存在，写入零值
			if err := writeFieldZeroValue(fieldBuf, field.Type); err != nil {
				return nil, fmt.Errorf("write zero value for field %s: %w", field.Name, err)
			}
		} else {
			if err := writeFieldBinaryValue(fieldBuf, field.Type, value); err != nil {
				return nil, fmt.Errorf("write field %s: %w", field.Name, err)
			}
		}

		// 直接使用二进制数据（无压缩）
		fieldData[i] = fieldBuf.Bytes()
	}

	// 2. 写入字段偏移表（相对于数据区起始位置）
	currentOffset := 0

	for _, data := range fieldData {
		// 写入字段偏移（相对于数据区）
		if err := binary.Write(buf, binary.LittleEndian, uint32(currentOffset)); err != nil {
			return nil, err
		}
		// 写入数据大小
		if err := binary.Write(buf, binary.LittleEndian, uint32(len(data))); err != nil {
			return nil, err
		}
		currentOffset += len(data)
	}

	// 3. 写入字段数据
	for _, data := range fieldData {
		if _, err := buf.Write(data); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// writeFieldBinaryValue 写入字段值（二进制格式）
func writeFieldBinaryValue(buf *bytes.Buffer, typ FieldType, value any) error {
	switch typ {
	case FieldTypeInt64:
		var v int64
		switch val := value.(type) {
		case int:
			v = int64(val)
		case int64:
			v = val
		case int32:
			v = int64(val)
		case int16:
			v = int64(val)
		case int8:
			v = int64(val)
		case float64:
			v = int64(val)
		default:
			return fmt.Errorf("cannot convert %T to int64", value)
		}
		return binary.Write(buf, binary.LittleEndian, v)

	case FieldTypeFloat:
		var v float64
		switch val := value.(type) {
		case float64:
			v = val
		case float32:
			v = float64(val)
		default:
			return fmt.Errorf("cannot convert %T to float64", value)
		}
		return binary.Write(buf, binary.LittleEndian, v)

	case FieldTypeBool:
		var b byte
		if value.(bool) {
			b = 1
		}
		return buf.WriteByte(b)

	case FieldTypeString:
		s := value.(string)
		// 写入长度
		if err := binary.Write(buf, binary.LittleEndian, uint32(len(s))); err != nil {
			return err
		}
		// 写入内容
		_, err := buf.WriteString(s)
		return err

	default:
		return fmt.Errorf("unsupported field type: %d", typ)
	}
}

// writeFieldZeroValue 写入字段零值
func writeFieldZeroValue(buf *bytes.Buffer, typ FieldType) error {
	switch typ {
	case FieldTypeInt64:
		return binary.Write(buf, binary.LittleEndian, int64(0))
	case FieldTypeFloat:
		return binary.Write(buf, binary.LittleEndian, float64(0))
	case FieldTypeBool:
		return buf.WriteByte(0)
	case FieldTypeString:
		return binary.Write(buf, binary.LittleEndian, uint32(0))
	default:
		return fmt.Errorf("unsupported field type: %d", typ)
	}
}

// decodeSSTableRowBinary 解码二进制格式的行数据（完整解码）
func decodeSSTableRowBinary(data []byte, schema *Schema) (*SSTableRow, error) {
	return decodeSSTableRowBinaryPartial(data, schema, nil)
}

// decodeSSTableRowBinaryPartial 按需解码（只读取和解压指定字段）
func decodeSSTableRowBinaryPartial(data []byte, schema *Schema, fields []string) (*SSTableRow, error) {
	buf := bytes.NewReader(data)

	// 读取并验证 Magic Number
	var magic uint32
	if err := binary.Read(buf, binary.LittleEndian, &magic); err != nil {
		return nil, err
	}
	if magic != SSTableRowMagic {
		return nil, fmt.Errorf("invalid row magic: %x", magic)
	}

	row := &SSTableRow{Data: make(map[string]any)}

	// 读取 Seq
	if err := binary.Read(buf, binary.LittleEndian, &row.Seq); err != nil {
		return nil, err
	}

	// 读取 Time
	if err := binary.Read(buf, binary.LittleEndian, &row.Time); err != nil {
		return nil, err
	}

	// 强制要求 Schema
	if schema == nil {
		return nil, fmt.Errorf("schema is required for decoding SSTable rows")
	}

	// 读取字段数量
	var fieldCount uint16
	if err := binary.Read(buf, binary.LittleEndian, &fieldCount); err != nil {
		return nil, err
	}

	// 读取字段偏移表
	type fieldInfo struct {
		offset uint32
		size   uint32
	}
	fieldInfos := make([]fieldInfo, fieldCount)
	for i := range fieldInfos {
		if err := binary.Read(buf, binary.LittleEndian, &fieldInfos[i].offset); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.LittleEndian, &fieldInfos[i].size); err != nil {
			return nil, err
		}
	}

	// 构建需要读取的字段集合
	needFields := make(map[string]bool)
	if fields == nil {
		// nil 表示读取所有字段
		for _, f := range schema.Fields {
			needFields[f.Name] = true
		}
	} else {
		for _, f := range fields {
			needFields[f] = true
		}
	}

	// 数据区起始位置（当前 buf 位置）
	dataStart := buf.Size() - int64(buf.Len())

	// 按需读取和解压字段
	for i, field := range schema.Fields {
		if i >= int(fieldCount) {
			break
		}

		need := needFields[field.Name]
		if !need {
			// 跳过不需要的字段（不读取，不解压）
			continue
		}

		// 读取字段数据（无压缩）
		info := fieldInfos[i]
		fieldPos := dataStart + int64(info.offset)

		// Seek 到字段位置
		if _, err := buf.Seek(fieldPos, 0); err != nil {
			return nil, fmt.Errorf("seek to field %s: %w", field.Name, err)
		}

		fieldData := make([]byte, info.size)
		if _, err := buf.Read(fieldData); err != nil {
			return nil, fmt.Errorf("read field %s: %w", field.Name, err)
		}

		// 解析字段值（直接从二进制数据）
		fieldBuf := bytes.NewReader(fieldData)
		value, err := readFieldBinaryValue(fieldBuf, field.Type, true)
		if err != nil {
			return nil, fmt.Errorf("parse field %s: %w", field.Name, err)
		}

		if value != nil {
			row.Data[field.Name] = value
		}
	}

	return row, nil
}

// readFieldBinaryValue 读取字段值（二进制格式）
func readFieldBinaryValue(buf *bytes.Reader, typ FieldType, keep bool) (any, error) {
	switch typ {
	case FieldTypeInt64:
		var v int64
		if err := binary.Read(buf, binary.LittleEndian, &v); err != nil {
			return nil, err
		}
		if keep {
			return v, nil
		}
		return nil, nil

	case FieldTypeFloat:
		var v float64
		if err := binary.Read(buf, binary.LittleEndian, &v); err != nil {
			return nil, err
		}
		if keep {
			return v, nil
		}
		return nil, nil

	case FieldTypeBool:
		b, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		if keep {
			return b == 1, nil
		}
		return nil, nil

	case FieldTypeString:
		var length uint32
		if err := binary.Read(buf, binary.LittleEndian, &length); err != nil {
			return nil, err
		}
		str := make([]byte, length)
		if _, err := buf.Read(str); err != nil {
			return nil, err
		}
		if keep {
			return string(str), nil
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("unsupported field type: %d", typ)
	}
}

// SSTableWriter SST 文件写入器
type SSTableWriter struct {
	file       *os.File
	builder    *BTreeBuilder
	dataOffset int64
	dataStart  int64 // 数据起始位置
	rowCount   int64
	minKey     int64
	maxKey     int64
	minTime    int64
	maxTime    int64
	schema     *Schema // Schema 用于优化编码
}

// NewSSTableWriter 创建 SST 写入器
func NewSSTableWriter(file *os.File, schema *Schema) *SSTableWriter {
	return &SSTableWriter{
		file:       file,
		builder:    NewBTreeBuilder(file, SSTableHeaderSize),
		dataOffset: 0, // 先写数据，后面会更新
		minKey:     -1,
		maxKey:     -1,
		minTime:    -1,
		maxTime:    -1,
		schema:     schema,
	}
}

// SSTableRow 表示一行数据
type SSTableRow struct {
	Seq  int64          // _seq
	Time int64          // _time
	Data map[string]any // 用户数据
}

// Add 添加一行数据
func (w *SSTableWriter) Add(row *SSTableRow) error {
	// 更新统计信息
	if w.minKey == -1 || row.Seq < w.minKey {
		w.minKey = row.Seq
	}
	if w.maxKey == -1 || row.Seq > w.maxKey {
		w.maxKey = row.Seq
	}
	if w.minTime == -1 || row.Time < w.minTime {
		w.minTime = row.Time
	}
	if w.maxTime == -1 || row.Time > w.maxTime {
		w.maxTime = row.Time
	}
	w.rowCount++

	// 序列化数据（使用 Schema 优化的二进制格式，无压缩）
	data, err := encodeSSTableRow(row, w.schema)
	if err != nil {
		return fmt.Errorf("encode row: %w", err)
	}

	// 写入数据块（不压缩）
	// 第一次写入时，确定数据起始位置
	if w.dataStart == 0 {
		// 预留足够空间给 B+Tree 索引
		// 假设索引最多占用 10% 的空间，最少 1 MB
		estimatedIndexSize := int64(10 * 1024 * 1024) // 10 MB
		w.dataStart = SSTableHeaderSize + estimatedIndexSize
		w.dataOffset = w.dataStart
	}

	offset := w.dataOffset
	_, err = w.file.WriteAt(data, offset)
	if err != nil {
		return err
	}

	// 添加到 B+Tree
	err = w.builder.Add(row.Seq, offset, int32(len(data)))
	if err != nil {
		return err
	}

	// 更新数据偏移
	w.dataOffset += int64(len(data))

	return nil
}

// Finish 完成写入
func (w *SSTableWriter) Finish() error {
	// 1. 构建 B+Tree 索引
	rootOffset, err := w.builder.Build()
	if err != nil {
		return err
	}

	// 2. 计算索引大小
	indexSize := w.dataStart - SSTableHeaderSize

	// 3. 创建 Header
	header := &SSTableHeader{
		Magic:       SSTableMagicNumber,
		Version:     SSTableVersion,
		Compression: 0, // 不使用压缩（保留字段用于向后兼容）
		IndexOffset: SSTableHeaderSize,
		IndexSize:   indexSize,
		RootOffset:  rootOffset,
		DataOffset:  w.dataStart,
		DataSize:    w.dataOffset - w.dataStart,
		RowCount:    w.rowCount,
		MinKey:      w.minKey,
		MaxKey:      w.maxKey,
		MinTime:     w.minTime,
		MaxTime:     w.maxTime,
	}

	// 4. 写入 Header
	headerData := header.Marshal()
	_, err = w.file.WriteAt(headerData, 0)
	if err != nil {
		return err
	}

	// 5. Sync 到磁盘
	return w.file.Sync()
}

// encodeSSTableRow 编码行数据 (使用二进制格式)
func encodeSSTableRow(row *SSTableRow, schema *Schema) ([]byte, error) {
	// 使用二进制格式编码
	encoded, err := encodeSSTableRowBinary(row, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to encode row: %w", err)
	}
	return encoded, nil
}

// SSTableReader SST 文件读取器
type SSTableReader struct {
	path     string
	file     *os.File
	mmap     mmap.MMap
	header   *SSTableHeader
	btReader *BTreeReader
	schema   *Schema // Schema 用于优化解码
}

// NewSSTableReader 创建 SST 读取器
func NewSSTableReader(path string) (*SSTableReader, error) {
	// 1. 打开文件
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// 2. mmap 映射
	mmapData, err := mmap.Map(file, mmap.RDONLY, 0)
	if err != nil {
		file.Close()
		return nil, err
	}

	// 3. 读取 Header
	if len(mmapData) < SSTableHeaderSize {
		mmapData.Unmap()
		file.Close()
		return nil, fmt.Errorf("file too small")
	}

	header := UnmarshalSSTableHeader(mmapData[:SSTableHeaderSize])
	if header == nil || !header.Validate() {
		mmapData.Unmap()
		file.Close()
		return nil, fmt.Errorf("invalid header")
	}

	// 4. 创建 B+Tree Reader
	btReader := NewBTreeReader(mmapData, header.RootOffset)

	return &SSTableReader{
		path:     path,
		file:     file,
		mmap:     mmapData,
		header:   header,
		btReader: btReader,
	}, nil
}

// Get 查询一行数据
func (r *SSTableReader) Get(key int64) (*SSTableRow, error) {
	// 1. 检查范围
	if key < r.header.MinKey || key > r.header.MaxKey {
		return nil, fmt.Errorf("key out of range")
	}

	// 2. 在 B+Tree 中查找
	dataOffset, dataSize, found := r.btReader.Get(key)
	if !found {
		return nil, fmt.Errorf("key not found")
	}

	// 3. 读取数据
	if dataOffset+int64(dataSize) > int64(len(r.mmap)) {
		return nil, fmt.Errorf("invalid data offset")
	}

	data := r.mmap[dataOffset : dataOffset+int64(dataSize)]

	// 4. 反序列化（无压缩）
	row, err := decodeSSTableRow(data, r.schema)
	if err != nil {
		return nil, err
	}

	return row, nil
}

// GetPartial 按需查询一行数据（只读取指定字段）
func (r *SSTableReader) GetPartial(key int64, fields []string) (*SSTableRow, error) {
	// 1. 检查范围
	if key < r.header.MinKey || key > r.header.MaxKey {
		return nil, fmt.Errorf("key out of range")
	}

	// 2. 在 B+Tree 中查找
	dataOffset, dataSize, found := r.btReader.Get(key)
	if !found {
		return nil, fmt.Errorf("key not found")
	}

	// 3. 读取数据
	if dataOffset+int64(dataSize) > int64(len(r.mmap)) {
		return nil, fmt.Errorf("invalid data offset")
	}

	data := r.mmap[dataOffset : dataOffset+int64(dataSize)]

	// 4. 按需反序列化（只解析需要的字段，无压缩）
	row, err := decodeSSTableRowBinaryPartial(data, r.schema, fields)
	if err != nil {
		return nil, err
	}

	return row, nil
}

// SetSchema 设置 Schema（用于优化编解码）
func (r *SSTableReader) SetSchema(schema *Schema) {
	r.schema = schema
}

// GetHeader 获取文件头信息
func (r *SSTableReader) GetHeader() *SSTableHeader {
	return r.header
}

// GetPath 获取文件路径
func (r *SSTableReader) GetPath() string {
	return r.path
}

// GetAllKeys 获取文件中所有的 key（按顺序）
func (r *SSTableReader) GetAllKeys() []int64 {
	return r.btReader.GetAllKeys()
}

// Close 关闭读取器
func (r *SSTableReader) Close() error {
	if r.mmap != nil {
		r.mmap.Unmap()
	}
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// decodeSSTableRow 解码行数据（只支持二进制格式）
func decodeSSTableRow(data []byte, schema *Schema) (*SSTableRow, error) {
	// 使用二进制格式解码
	row, err := decodeSSTableRowBinary(data, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to decode row: %w", err)
	}
	return row, nil
}

// SSTableManager SST 文件管理器
type SSTableManager struct {
	dir     string
	readers []*SSTableReader
	mu      sync.RWMutex
	schema  *Schema // Schema 用于优化编解码
}

// NewSSTableManager 创建 SST 管理器
func NewSSTableManager(dir string) (*SSTableManager, error) {
	// 确保目录存在
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	mgr := &SSTableManager{
		dir:     dir,
		readers: make([]*SSTableReader, 0),
	}

	// 恢复现有的 SST 文件
	err = mgr.recover()
	if err != nil {
		return nil, err
	}

	return mgr, nil
}

// recover 恢复现有的 SST 文件
func (m *SSTableManager) recover() error {
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
		reader, err := NewSSTableReader(file)
		if err != nil {
			return err
		}

		// 设置 Schema
		if m.schema != nil {
			reader.SetSchema(m.schema)
		}

		m.readers = append(m.readers, reader)
	}

	return nil
}

// CreateSST 创建新的 SST 文件
// fileNumber: 文件编号（由 VersionSet 分配）
func (m *SSTableManager) CreateSST(fileNumber int64, rows []*SSTableRow) (*SSTableReader, error) {
	return m.CreateSSTWithLevel(fileNumber, rows, 0) // 默认创建到 L0
}

// CreateSSTWithLevel 创建新的 SST 文件到指定层级
// fileNumber: 文件编号（由 VersionSet 分配）
func (m *SSTableManager) CreateSSTWithLevel(fileNumber int64, rows []*SSTableRow, level int) (*SSTableReader, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sstPath := filepath.Join(m.dir, fmt.Sprintf("%06d.sst", fileNumber))

	// 创建文件
	file, err := os.Create(sstPath)
	if err != nil {
		return nil, err
	}

	writer := NewSSTableWriter(file, m.schema)

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
	reader, err := NewSSTableReader(sstPath)
	if err != nil {
		return nil, err
	}

	// 设置 Schema
	if m.schema != nil {
		reader.SetSchema(m.schema)
	}

	// 添加到 readers 列表
	m.readers = append(m.readers, reader)

	return reader, nil
}

// SetSchema 设置 Schema（用于优化编解码）
func (m *SSTableManager) SetSchema(schema *Schema) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.schema = schema

	// 为所有已存在的 readers 设置 schema
	for _, reader := range m.readers {
		reader.SetSchema(schema)
	}
}

// Get 从所有 SST 文件中查找数据
func (m *SSTableManager) Get(seq int64) (*SSTableRow, error) {
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

// GetPartial 从所有 SST 文件中按需查找数据（只读取指定字段）
func (m *SSTableManager) GetPartial(seq int64, fields []string) (*SSTableRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 从后往前查找（新的文件优先）
	for i := len(m.readers) - 1; i >= 0; i-- {
		reader := m.readers[i]
		row, err := reader.GetPartial(seq, fields)
		if err == nil {
			return row, nil
		}
	}

	return nil, fmt.Errorf("key not found: %d", seq)
}

// RemoveReader 移除指定文件编号的 reader（用于 compaction）
func (m *SSTableManager) RemoveReader(fileNumber int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找并移除对应的 reader
	for i, reader := range m.readers {
		// 从文件名中提取文件编号
		filename := filepath.Base(reader.path)
		var readerFileNum int64
		if _, err := fmt.Sscanf(filename, "%d.sst", &readerFileNum); err == nil {
			if readerFileNum == fileNumber {
				// 关闭 reader
				reader.Close()

				// 从列表中移除
				m.readers = append(m.readers[:i], m.readers[i+1:]...)
				return nil
			}
		}
	}

	return fmt.Errorf("reader for file %d not found", fileNumber)
}

// AddReader 添加 reader 到管理器（用于 compaction 创建的新文件）
func (m *SSTableManager) AddReader(reader *SSTableReader) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.readers = append(m.readers, reader)
}

// GetReaders 获取所有 Readers（用于扫描）
func (m *SSTableManager) GetReaders() []*SSTableReader {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	readers := make([]*SSTableReader, len(m.readers))
	copy(readers, m.readers)

	// 按 MinKey 排序，确保查询时按 seq 顺序遍历
	// 这对于 compaction 后的文件顺序至关重要：
	// compaction 生成的新文件包含旧 seq，但被添加到 readers 末尾
	// 排序后保证查询结果有序
	sort.Slice(readers, func(i, j int) bool {
		return readers[i].header.MinKey < readers[j].header.MinKey
	})

	return readers
}

// GetMaxSeq 获取所有 SST 中的最大 seq
func (m *SSTableManager) GetMaxSeq() int64 {
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
func (m *SSTableManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.readers)
}

// ListFiles 列出所有 SST 文件
func (m *SSTableManager) ListFiles() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := make([]string, 0, len(m.readers))
	for _, reader := range m.readers {
		files = append(files, reader.path)
	}

	return files
}

// Close 关闭所有 SST Readers
func (m *SSTableManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, reader := range m.readers {
		reader.Close()
	}

	m.readers = nil
	return nil
}

// SSTableStats 统计信息
type SSTableStats struct {
	FileCount int
	TotalSize int64
	MinSeq    int64
	MaxSeq    int64
}

// GetStats 获取统计信息
func (m *SSTableManager) GetStats() *SSTableStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SSTableStats{
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
