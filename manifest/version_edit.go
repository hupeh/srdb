package manifest

import (
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"io"
)

// EditType 变更类型
type EditType byte

const (
	EditTypeAddFile     EditType = 1 // 添加文件
	EditTypeDeleteFile  EditType = 2 // 删除文件
	EditTypeSetNextFile EditType = 3 // 设置下一个文件编号
	EditTypeSetLastSeq  EditType = 4 // 设置最后序列号
)

// VersionEdit 版本变更记录
type VersionEdit struct {
	// 添加的文件
	AddedFiles []*FileMetadata

	// 删除的文件（文件编号列表）
	DeletedFiles []int64

	// 下一个文件编号
	NextFileNumber *int64

	// 最后序列号
	LastSequence *int64
}

// NewVersionEdit 创建版本变更
func NewVersionEdit() *VersionEdit {
	return &VersionEdit{
		AddedFiles:   make([]*FileMetadata, 0),
		DeletedFiles: make([]int64, 0),
	}
}

// AddFile 添加文件
func (e *VersionEdit) AddFile(file *FileMetadata) {
	e.AddedFiles = append(e.AddedFiles, file)
}

// DeleteFile 删除文件
func (e *VersionEdit) DeleteFile(fileNumber int64) {
	e.DeletedFiles = append(e.DeletedFiles, fileNumber)
}

// SetNextFileNumber 设置下一个文件编号
func (e *VersionEdit) SetNextFileNumber(num int64) {
	e.NextFileNumber = &num
}

// SetLastSequence 设置最后序列号
func (e *VersionEdit) SetLastSequence(seq int64) {
	e.LastSequence = &seq
}

// Encode 编码为字节
func (e *VersionEdit) Encode() ([]byte, error) {
	// 使用 JSON 编码（简单实现）
	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}

	// 格式: CRC32(4) + Length(4) + Data
	totalLen := 8 + len(data)
	buf := make([]byte, totalLen)

	// 计算 CRC32
	crc := crc32.ChecksumIEEE(data)
	binary.LittleEndian.PutUint32(buf[0:4], crc)

	// 写入长度
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(data)))

	// 写入数据
	copy(buf[8:], data)

	return buf, nil
}

// Decode 从字节解码
func (e *VersionEdit) Decode(data []byte) error {
	if len(data) < 8 {
		return io.ErrUnexpectedEOF
	}

	// 读取 CRC32
	crc := binary.LittleEndian.Uint32(data[0:4])

	// 读取长度
	length := binary.LittleEndian.Uint32(data[4:8])

	if len(data) < int(8+length) {
		return io.ErrUnexpectedEOF
	}

	// 读取数据
	editData := data[8 : 8+length]

	// 验证 CRC32
	if crc32.ChecksumIEEE(editData) != crc {
		return io.ErrUnexpectedEOF
	}

	// JSON 解码
	return json.Unmarshal(editData, e)
}
