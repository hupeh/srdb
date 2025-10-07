package sst

import (
	"encoding/json"
	"os"

	"code.tczkiot.com/srdb/btree"
	"github.com/golang/snappy"
)

// Writer SST 文件写入器
type Writer struct {
	file        *os.File
	builder     *btree.Builder
	dataOffset  int64
	dataStart   int64 // 数据起始位置
	rowCount    int64
	minKey      int64
	maxKey      int64
	minTime     int64
	maxTime     int64
	compression uint8
}

// NewWriter 创建 SST 写入器
func NewWriter(file *os.File) *Writer {
	return &Writer{
		file:        file,
		builder:     btree.NewBuilder(file, HeaderSize),
		dataOffset:  0, // 先写数据，后面会更新
		compression: CompressionSnappy,
		minKey:      -1,
		maxKey:      -1,
		minTime:     -1,
		maxTime:     -1,
	}
}

// Row 表示一行数据
type Row struct {
	Seq  int64          // _seq
	Time int64          // _time
	Data map[string]any // 用户数据
}

// Add 添加一行数据
func (w *Writer) Add(row *Row) error {
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

	// 序列化数据 (简单的 JSON 序列化，后续可以优化)
	data := encodeRow(row)

	// 压缩数据
	var compressed []byte
	if w.compression == CompressionSnappy {
		compressed = snappy.Encode(nil, data)
	} else {
		compressed = data
	}

	// 写入数据块
	// 第一次写入时，确定数据起始位置
	if w.dataStart == 0 {
		// 预留足够空间给 B+Tree 索引
		// 假设索引最多占用 10% 的空间，最少 1 MB
		estimatedIndexSize := int64(10 * 1024 * 1024) // 10 MB
		w.dataStart = HeaderSize + estimatedIndexSize
		w.dataOffset = w.dataStart
	}

	offset := w.dataOffset
	_, err := w.file.WriteAt(compressed, offset)
	if err != nil {
		return err
	}

	// 添加到 B+Tree
	err = w.builder.Add(row.Seq, offset, int32(len(compressed)))
	if err != nil {
		return err
	}

	// 更新数据偏移
	w.dataOffset += int64(len(compressed))

	return nil
}

// Finish 完成写入
func (w *Writer) Finish() error {
	// 1. 构建 B+Tree 索引
	rootOffset, err := w.builder.Build()
	if err != nil {
		return err
	}

	// 2. 计算索引大小
	indexSize := w.dataStart - HeaderSize

	// 3. 创建 Header
	header := &Header{
		Magic:       MagicNumber,
		Version:     Version,
		Compression: w.compression,
		IndexOffset: HeaderSize,
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

// encodeRow 编码行数据 (使用二进制格式)
func encodeRow(row *Row) []byte {
	// 使用二进制格式编码
	encoded, err := encodeRowBinary(row)
	if err != nil {
		// 降级到 JSON (不应该发生)
		data := map[string]interface{}{
			"_seq":  row.Seq,
			"_time": row.Time,
			"data":  row.Data,
		}
		encoded, _ = json.Marshal(data)
	}
	return encoded
}
