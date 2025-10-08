package srdb

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"os"
	"sort"

	"github.com/edsrzf/mmap-go"
)

/*
索引文件存储格式 (B+Tree)

文件结构：
┌─────────────────────────────────────────────────────────────┐
│ Header (256 bytes)                                          │
├─────────────────────────────────────────────────────────────┤
│ B+Tree 索引区                                                │
│   ├─ 内部节点 (BTreeNodeSize = 4096 bytes)                  │
│   │   ├─ NodeType (1 byte): 0=Leaf, 1=Internal             │
│   │   ├─ KeyCount (2 bytes): 节点中的 key 数量              │
│   │   ├─ Keys[]: int64 数组 (8 bytes each)                 │
│   │   └─ Children[]: int64 偏移量数组 (8 bytes each)        │
│   │                                                          │
│   └─ 叶子节点 (BTreeNodeSize = 4096 bytes)                  │
│       ├─ NodeType (1 byte): 0                               │
│       ├─ KeyCount (2 bytes): 节点中的 key 数量              │
│       ├─ Keys[]: int64 数组 (8 bytes each)                 │
│       ├─ Offsets[]: int64 数据偏移量 (8 bytes each)         │
│       └─ Sizes[]: int32 数据大小 (4 bytes each)             │
├─────────────────────────────────────────────────────────────┤
│ 数据块区                                                     │
│   ├─ Entry 1:                                               │
│   │   ├─ ValueLen (4 bytes): 字段值长度                     │
│   │   ├─ Value (N bytes): 字段值 (原始字符串)               │
│   │   ├─ SeqCount (4 bytes): seq 数量                       │
│   │   └─ Seqs (8 bytes each): seq 列表                     │
│   │                                                          │
│   ├─ Entry 2: ...                                           │
│   └─ Entry N: ...                                           │
└─────────────────────────────────────────────────────────────┘

Header 格式 (256 bytes):
  Offset | Size | Field          | Description
  -------|------|----------------|----------------------------------
  0      | 4    | Magic          | 0x49445842 ("IDXB")
  4      | 4    | FormatVersion  | 文件格式版本 (1)
  8      | 8    | IndexVersion   | 索引版本号 (对应 Metadata.Version)
  16     | 8    | RootOffset     | B+Tree 根节点偏移
  24     | 8    | DataStart      | 数据块起始位置
  32     | 8    | MinSeq         | 最小 seq
  40     | 8    | MaxSeq         | 最大 seq
  48     | 8    | RowCount       | 总行数
  56     | 8    | CreatedAt      | 创建时间 (UnixNano)
  64     | 8    | UpdatedAt      | 更新时间 (UnixNano)
  72     | 184  | Reserved       | 预留空间

索引条目格式 (变长):
  Offset | Size        | Field      | Description
  -------|-------------|------------|----------------------------------
  0      | 4           | ValueLen   | 字段值长度 (N)
  4      | N           | Value      | 字段值 (原始字符串，用于验证哈希冲突)
  4+N    | 4           | SeqCount   | seq 数量 (M)
  8+N    | M * 8       | Seqs       | seq 列表 (int64 数组)

Key 生成规则:
  - 使用 MD5 哈希将字符串值转为 int64
  - key = MD5(value)[0:8] 的 LittleEndian uint64
  - 存储原始 value 用于验证哈希冲突

查询流程:
  1. value → key (MD5 哈希)
  2. B+Tree.Get(key) → (offset, size)
  3. 读取数据块 mmap[offset:offset+size]
  4. 解码并验证原始 value (处理哈希冲突)
  5. 返回 seqs 列表

性能特点:
  - 加载: O(1) - 只需 mmap 映射
  - 查询: O(log n) - B+Tree 查找
  - 内存: 几乎为 0 - 零拷贝 mmap
  - 支持范围查询 (未来可扩展)
*/

const (
	IndexHeaderSize = 256          // 索引文件头大小
	IndexMagic      = 0x49445842   // "IDXB" - Index B-Tree
	IndexVersion    = 1            // 文件格式版本
)

// IndexHeader 索引文件头
type IndexHeader struct {
	Magic          uint32 // 魔数 "IDXB"
	FormatVersion  uint32 // 文件格式版本号
	IndexVersion   int64  // 索引版本号（对应 IndexMetadata.Version）
	RootOffset     int64  // B+Tree 根节点偏移
	DataStart      int64  // 数据块起始位置
	MinSeq         int64  // 最小 seq
	MaxSeq         int64  // 最大 seq
	RowCount       int64  // 总行数
	CreatedAt      int64  // 创建时间
	UpdatedAt      int64  // 更新时间
	Reserved       [184]byte // 预留空间（减少 8 字节给 IndexVersion，减少 8 字节调整对齐）
}

// Marshal 序列化 Header
func (h *IndexHeader) Marshal() []byte {
	buf := make([]byte, IndexHeaderSize)
	binary.LittleEndian.PutUint32(buf[0:4], h.Magic)
	binary.LittleEndian.PutUint32(buf[4:8], h.FormatVersion)
	binary.LittleEndian.PutUint64(buf[8:16], uint64(h.IndexVersion))
	binary.LittleEndian.PutUint64(buf[16:24], uint64(h.RootOffset))
	binary.LittleEndian.PutUint64(buf[24:32], uint64(h.DataStart))
	binary.LittleEndian.PutUint64(buf[32:40], uint64(h.MinSeq))
	binary.LittleEndian.PutUint64(buf[40:48], uint64(h.MaxSeq))
	binary.LittleEndian.PutUint64(buf[48:56], uint64(h.RowCount))
	binary.LittleEndian.PutUint64(buf[56:64], uint64(h.CreatedAt))
	binary.LittleEndian.PutUint64(buf[64:72], uint64(h.UpdatedAt))
	copy(buf[72:], h.Reserved[:])
	return buf
}

// UnmarshalIndexHeader 反序列化 Header
func UnmarshalIndexHeader(data []byte) *IndexHeader {
	if len(data) < IndexHeaderSize {
		return nil
	}
	h := &IndexHeader{}
	h.Magic = binary.LittleEndian.Uint32(data[0:4])
	h.FormatVersion = binary.LittleEndian.Uint32(data[4:8])
	h.IndexVersion = int64(binary.LittleEndian.Uint64(data[8:16]))
	h.RootOffset = int64(binary.LittleEndian.Uint64(data[16:24]))
	h.DataStart = int64(binary.LittleEndian.Uint64(data[24:32]))
	h.MinSeq = int64(binary.LittleEndian.Uint64(data[32:40]))
	h.MaxSeq = int64(binary.LittleEndian.Uint64(data[40:48]))
	h.RowCount = int64(binary.LittleEndian.Uint64(data[48:56]))
	h.CreatedAt = int64(binary.LittleEndian.Uint64(data[56:64]))
	h.UpdatedAt = int64(binary.LittleEndian.Uint64(data[64:72]))
	copy(h.Reserved[:], data[72:IndexHeaderSize])
	return h
}

// valueToKey 将字段值转换为 B+Tree key（使用哈希）
//
// 原理：
//   - 字符串无法直接用作 B+Tree key (需要 int64)
//   - 使用 MD5 哈希将任意字符串映射为 int64
//   - 取 MD5 的前 8 字节作为 key
//
// 哈希冲突处理：
//   - 存储原始 value 在数据块中
//   - 查询时验证原始 value 是否匹配
//   - 冲突时返回 nil（极低概率）
//
// 示例：
//   "Alice" → MD5 → 0x3bc15c8aae3e4124... → key = 0x3bc15c8aae3e4124
func valueToKey(value string) int64 {
	// 使用 MD5 的前 8 字节作为 int64 key
	hash := md5.Sum([]byte(value))
	return int64(binary.LittleEndian.Uint64(hash[:8]))
}

// encodeIndexEntry 将索引条目编码为二进制格式（零拷贝友好）
//
// 格式：[ValueLen(4B)][Value(N bytes)][SeqCount(4B)][Seq1(8B)][Seq2(8B)]...
//
// 示例：
//   value = "Alice", seqs = [1, 5, 10]
//   编码结果：
//     [0x05, 0x00, 0x00, 0x00]           // ValueLen = 5
//     [0x41, 0x6c, 0x69, 0x63, 0x65]     // "Alice"
//     [0x03, 0x00, 0x00, 0x00]           // SeqCount = 3
//     [0x01, 0x00, 0x00, 0x00, ...]      // Seq1 = 1
//     [0x05, 0x00, 0x00, 0x00, ...]      // Seq2 = 5
//     [0x0a, 0x00, 0x00, 0x00, ...]      // Seq3 = 10
//
// 总大小：4 + 5 + 4 + 3*8 = 37 bytes
func encodeIndexEntry(value string, seqs []int64) []byte {
	valueBytes := []byte(value)
	size := 4 + len(valueBytes) + 4 + len(seqs)*8
	buf := make([]byte, size)

	// 写入 ValueLen
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(valueBytes)))

	// 写入 Value
	copy(buf[4:], valueBytes)

	// 写入 SeqCount
	offset := 4 + len(valueBytes)
	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(seqs)))

	// 写入 Seqs
	offset += 4
	for i, seq := range seqs {
		binary.LittleEndian.PutUint64(buf[offset+i*8:offset+(i+1)*8], uint64(seq))
	}

	return buf
}

// decodeIndexEntry 从二进制格式解码索引条目（零拷贝）
//
// 参数：
//   data: 编码后的二进制数据（来自 mmap，零拷贝）
//
// 返回：
//   value: 原始字段值（用于验证哈希冲突）
//   seqs: seq 列表
//   err: 解码错误
//
// 零拷贝优化：
//   - 直接从 mmap 数据中读取，不复制
//   - string(data[4:4+valueLen]) 会复制，但无法避免
//   - seqs 数组需要分配，但只复制指针大小的数据
func decodeIndexEntry(data []byte) (value string, seqs []int64, err error) {
	if len(data) < 8 {
		return "", nil, fmt.Errorf("data too short: %d bytes", len(data))
	}

	// 读取 ValueLen
	valueLen := binary.LittleEndian.Uint32(data[0:4])
	if len(data) < int(4+valueLen+4) {
		return "", nil, fmt.Errorf("data too short for value: expected %d, got %d", 4+valueLen+4, len(data))
	}

	// 读取 Value
	value = string(data[4 : 4+valueLen])

	// 读取 SeqCount
	offset := 4 + int(valueLen)
	seqCount := binary.LittleEndian.Uint32(data[offset : offset+4])

	// 验证数据长度
	expectedSize := offset + 4 + int(seqCount)*8
	if len(data) < expectedSize {
		return "", nil, fmt.Errorf("data too short for seqs: expected %d, got %d", expectedSize, len(data))
	}

	// 读取 Seqs
	seqs = make([]int64, seqCount)
	offset += 4
	for i := 0; i < int(seqCount); i++ {
		seqs[i] = int64(binary.LittleEndian.Uint64(data[offset+i*8 : offset+(i+1)*8]))
	}

	return value, seqs, nil
}

// IndexBTreeWriter 使用 B+Tree 写入索引
//
// 写入流程：
//   1. Add(): 收集所有 (value, seqs) 到内存
//   2. Build(): 
//      a. 计算所有 value 的 key (MD5 哈希)
//      b. 按 key 排序（B+Tree 要求有序）
//      c. 编码所有条目为二进制格式
//      d. 构建 B+Tree 索引
//      e. 写入 Header + B+Tree + 数据块
//
// 文件布局：
//   [Header] → [B+Tree] → [Data Blocks]
type IndexBTreeWriter struct {
	file       *os.File
	header     IndexHeader
	entries    map[string][]int64 // value -> seqs
	dataOffset int64
}

// NewIndexBTreeWriter 创建索引写入器
func NewIndexBTreeWriter(file *os.File, metadata IndexMetadata) *IndexBTreeWriter {
	return &IndexBTreeWriter{
		file: file,
		header: IndexHeader{
			Magic:         IndexMagic,
			FormatVersion: IndexVersion,
			IndexVersion:  metadata.Version,
			MinSeq:        metadata.MinSeq,
			MaxSeq:        metadata.MaxSeq,
			RowCount:      metadata.RowCount,
			CreatedAt:     metadata.CreatedAt,
			UpdatedAt:     metadata.UpdatedAt,
		},
		entries:    make(map[string][]int64),
		dataOffset: IndexHeaderSize,
	}
}

// Add 添加索引条目
func (w *IndexBTreeWriter) Add(value string, seqs []int64) {
	w.entries[value] = seqs
}

// Build 构建并写入索引文件
func (w *IndexBTreeWriter) Build() error {
	// 1. 计算所有 key 并按 key 排序（确保 B+Tree 构建有序）
	type valueKey struct {
		value string
		key   int64
	}
	var valueKeys []valueKey
	for value := range w.entries {
		valueKeys = append(valueKeys, valueKey{
			value: value,
			key:   valueToKey(value),
		})
	}

	// 按 key 排序（而不是按字符串）
	sort.Slice(valueKeys, func(i, j int) bool {
		return valueKeys[i].key < valueKeys[j].key
	})

	// 2. 先写入数据块并记录位置
	type keyOffset struct {
		key    int64
		offset int64
		size   int32
	}
	var keyOffsets []keyOffset

	// 预留 Header 空间
	currentOffset := int64(IndexHeaderSize)

	// 构建数据块（使用二进制格式，无压缩）
	var dataBlocks [][]byte
	for _, vk := range valueKeys {
		value := vk.value
		seqs := w.entries[value]

		// 编码为二进制格式
		binaryData := encodeIndexEntry(value, seqs)

		// 记录 key 和数据位置（key 已经在 vk 中）
		key := vk.key
		dataBlocks = append(dataBlocks, binaryData)

		// 暂时不知道确切的 offset，先占位
		keyOffsets = append(keyOffsets, keyOffset{
			key:    key,
			offset: 0, // 稍后填充
			size:   int32(len(binaryData)),
		})

		currentOffset += int64(len(binaryData))
	}

	// 3. 计算 B+Tree 起始位置（紧接 Header）
	btreeStart := int64(IndexHeaderSize)

	// 估算 B+Tree 大小（每个叶子节点最多 BTreeOrder 个条目）
	numEntries := len(keyOffsets)
	numLeafNodes := (numEntries + BTreeOrder - 1) / BTreeOrder

	// 计算所有层级的节点总数
	totalNodes := numLeafNodes
	nodesAtCurrentLevel := numLeafNodes
	for nodesAtCurrentLevel > 1 {
		nodesAtCurrentLevel = (nodesAtCurrentLevel + BTreeOrder - 1) / BTreeOrder
		totalNodes += nodesAtCurrentLevel
	}

	btreeSize := int64(totalNodes * BTreeNodeSize)
	dataStart := btreeStart + btreeSize

	// 4. 更新数据块的实际偏移量
	currentDataOffset := dataStart
	for i := range keyOffsets {
		keyOffsets[i].offset = currentDataOffset
		currentDataOffset += int64(keyOffsets[i].size)
	}

	// 5. 写入 Header（预留位置）
	w.header.DataStart = dataStart
	w.file.WriteAt(w.header.Marshal(), 0)

	// 6. 构建 B+Tree
	builder := NewBTreeBuilder(w.file, btreeStart)
	for _, ko := range keyOffsets {
		err := builder.Add(ko.key, ko.offset, ko.size)
		if err != nil {
			return fmt.Errorf("failed to add to btree: %w", err)
		}
	}

	rootOffset, err := builder.Build()
	if err != nil {
		return fmt.Errorf("failed to build btree: %w", err)
	}

	// 7. 写入数据块
	currentDataOffset = dataStart
	for _, data := range dataBlocks {
		_, err := w.file.WriteAt(data, currentDataOffset)
		if err != nil {
			return fmt.Errorf("failed to write data block: %w", err)
		}
		currentDataOffset += int64(len(data))
	}

	// 8. 更新 Header（写入正确的 RootOffset）
	w.header.RootOffset = rootOffset
	_, err = w.file.WriteAt(w.header.Marshal(), 0)
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// 9. Sync 到磁盘
	return w.file.Sync()
}

// IndexBTreeReader 使用 B+Tree 读取索引
//
// 读取流程：
//   1. mmap 映射整个文件（零拷贝）
//   2. 读取 Header
//   3. 创建 BTreeReader (指向 RootOffset)
//   4. Get(value):
//      a. value → key (MD5 哈希)
//      b. BTree.Get(key) → (offset, size)
//      c. 读取 mmap[offset:offset+size]（零拷贝）
//      d. 解码并验证原始 value
//      e. 返回 seqs
//
// 性能优化：
//   - mmap 零拷贝：不需要加载整个文件到内存
//   - B+Tree 索引：O(log n) 查询
//   - 按需读取：只读取需要的数据块
type IndexBTreeReader struct {
	file   *os.File
	mmap   mmap.MMap
	header IndexHeader
	btree  *BTreeReader
}

// NewIndexBTreeReader 创建索引读取器
func NewIndexBTreeReader(file *os.File) (*IndexBTreeReader, error) {
	// 读取 Header
	headerData := make([]byte, IndexHeaderSize)
	_, err := file.ReadAt(headerData, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	header := UnmarshalIndexHeader(headerData)
	if header == nil || header.Magic != IndexMagic {
		return nil, fmt.Errorf("invalid index file: bad magic")
	}

	// mmap 整个文件
	mmapData, err := mmap.Map(file, mmap.RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to mmap index file: %w", err)
	}

	// 创建 B+Tree Reader
	btree := NewBTreeReader(mmapData, header.RootOffset)

	return &IndexBTreeReader{
		file:   file,
		mmap:   mmapData,
		header: *header,
		btree:  btree,
	}, nil
}

// Get 查询字段值对应的 seq 列表（零拷贝）
//
// 参数：
//   value: 字段值（例如 "Alice"）
//
// 返回：
//   seqs: seq 列表（例如 [1, 5, 10]）
//   err: 查询错误
//
// 查询流程：
//   1. value → key (MD5 哈希)
//   2. B+Tree.Get(key) → (offset, size)
//   3. 读取 mmap[offset:offset+size]（零拷贝）
//   4. 解码并验证原始 value（处理哈希冲突）
//   5. 返回 seqs
//
// 哈希冲突处理：
//   - 如果 storedValue != value，说明发生哈希冲突
//   - 返回 nil（表示未找到）
//   - 冲突概率极低（MD5 64位空间）
func (r *IndexBTreeReader) Get(value string) ([]int64, error) {
	// 计算 key
	key := valueToKey(value)

	// 在 B+Tree 中查找
	dataOffset, dataSize, found := r.btree.Get(key)
	if !found {
		return nil, nil
	}

	// 读取数据块（零拷贝）
	if dataOffset+int64(dataSize) > int64(len(r.mmap)) {
		return nil, fmt.Errorf("data offset out of range: offset=%d, size=%d, mmap_len=%d", dataOffset, dataSize, len(r.mmap))
	}

	binaryData := r.mmap[dataOffset : dataOffset+int64(dataSize)]

	// 解码二进制数据
	storedValue, seqs, err := decodeIndexEntry(binaryData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode entry: %w", err)
	}

	// 验证原始值（处理哈希冲突）
	if storedValue != value {
		// 哈希冲突，返回空
		return nil, nil
	}

	return seqs, nil
}

// GetMetadata 获取元数据
func (r *IndexBTreeReader) GetMetadata() IndexMetadata {
	return IndexMetadata{
		Version:   r.header.IndexVersion,
		MinSeq:    r.header.MinSeq,
		MaxSeq:    r.header.MaxSeq,
		RowCount:  r.header.RowCount,
		CreatedAt: r.header.CreatedAt,
		UpdatedAt: r.header.UpdatedAt,
	}
}

// Close 关闭读取器
func (r *IndexBTreeReader) Close() error {
	if r.mmap != nil {
		r.mmap.Unmap()
	}
	return nil
}
