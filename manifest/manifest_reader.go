package manifest

import (
	"encoding/binary"
	"io"
)

// Reader MANIFEST 读取器
type Reader struct {
	file io.Reader
}

// NewReader 创建 MANIFEST 读取器
func NewReader(file io.Reader) *Reader {
	return &Reader{
		file: file,
	}
}

// ReadEdit 读取版本变更
func (r *Reader) ReadEdit() (*VersionEdit, error) {
	// 读取 CRC32 和 Length
	header := make([]byte, 8)
	_, err := io.ReadFull(r.file, header)
	if err != nil {
		return nil, err
	}

	// 读取长度
	length := binary.LittleEndian.Uint32(header[4:8])

	// 读取数据
	data := make([]byte, 8+length)
	copy(data[0:8], header)
	_, err = io.ReadFull(r.file, data[8:])
	if err != nil {
		return nil, err
	}

	// 解码
	edit := NewVersionEdit()
	err = edit.Decode(data)
	if err != nil {
		return nil, err
	}

	return edit, nil
}
