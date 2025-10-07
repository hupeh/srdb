package sst

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// 二进制编码格式:
// [Magic: 4 bytes][Seq: 8 bytes][Time: 8 bytes][DataLen: 4 bytes][Data: variable]

const (
	RowMagic = 0x524F5733 // "ROW3"
)

// encodeRowBinary 使用二进制格式编码行数据
func encodeRowBinary(row *Row) ([]byte, error) {
	buf := new(bytes.Buffer)

	// 写入 Magic Number (用于验证)
	if err := binary.Write(buf, binary.LittleEndian, uint32(RowMagic)); err != nil {
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

	// 序列化用户数据 (仍使用 JSON，但只序列化用户数据部分)
	dataBytes, err := json.Marshal(row.Data)
	if err != nil {
		return nil, err
	}

	// 写入数据长度
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(dataBytes))); err != nil {
		return nil, err
	}

	// 写入数据
	if _, err := buf.Write(dataBytes); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// decodeRowBinary 解码二进制格式的行数据
func decodeRowBinary(data []byte) (*Row, error) {
	buf := bytes.NewReader(data)

	// 读取并验证 Magic Number
	var magic uint32
	if err := binary.Read(buf, binary.LittleEndian, &magic); err != nil {
		return nil, err
	}
	if magic != RowMagic {
		return nil, fmt.Errorf("invalid row magic: %x", magic)
	}

	row := &Row{}

	// 读取 Seq
	if err := binary.Read(buf, binary.LittleEndian, &row.Seq); err != nil {
		return nil, err
	}

	// 读取 Time
	if err := binary.Read(buf, binary.LittleEndian, &row.Time); err != nil {
		return nil, err
	}

	// 读取数据长度
	var dataLen uint32
	if err := binary.Read(buf, binary.LittleEndian, &dataLen); err != nil {
		return nil, err
	}

	// 读取数据
	dataBytes := make([]byte, dataLen)
	if _, err := buf.Read(dataBytes); err != nil {
		return nil, err
	}

	// 反序列化用户数据
	if err := json.Unmarshal(dataBytes, &row.Data); err != nil {
		return nil, err
	}

	return row, nil
}
