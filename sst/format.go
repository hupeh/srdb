package sst

import (
	"encoding/binary"
)

const (
	// 文件格式
	MagicNumber = 0x53535433 // "SST3"
	Version     = 1
	HeaderSize  = 256       // 文件头大小
	BlockSize   = 64 * 1024 // 数据块大小 (64 KB)

	// 压缩类型
	CompressionNone   = 0
	CompressionSnappy = 1
)

// Header SST 文件头 (256 bytes)
type Header struct {
	// 基础信息 (32 bytes)
	Magic       uint32 // Magic Number: 0x53535433
	Version     uint32 // 版本号
	Compression uint8  // 压缩类型
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
func (h *Header) Marshal() []byte {
	buf := make([]byte, HeaderSize)

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
func UnmarshalHeader(data []byte) *Header {
	if len(data) < HeaderSize {
		return nil
	}

	h := &Header{}

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
func (h *Header) Validate() bool {
	return h.Magic == MagicNumber && h.Version == Version
}
