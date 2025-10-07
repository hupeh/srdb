package sst

import (
	"encoding/json"
	"fmt"
	"os"

	"code.tczkiot.com/srdb/btree"
	"github.com/edsrzf/mmap-go"
	"github.com/golang/snappy"
)

// Reader SST 文件读取器
type Reader struct {
	path     string
	file     *os.File
	mmap     mmap.MMap
	header   *Header
	btReader *btree.Reader
}

// NewReader 创建 SST 读取器
func NewReader(path string) (*Reader, error) {
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
	if len(mmapData) < HeaderSize {
		mmapData.Unmap()
		file.Close()
		return nil, fmt.Errorf("file too small")
	}

	header := UnmarshalHeader(mmapData[:HeaderSize])
	if header == nil || !header.Validate() {
		mmapData.Unmap()
		file.Close()
		return nil, fmt.Errorf("invalid header")
	}

	// 4. 创建 B+Tree Reader
	btReader := btree.NewReader(mmapData, header.RootOffset)

	return &Reader{
		path:     path,
		file:     file,
		mmap:     mmapData,
		header:   header,
		btReader: btReader,
	}, nil
}

// Get 查询一行数据
func (r *Reader) Get(key int64) (*Row, error) {
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

	compressed := r.mmap[dataOffset : dataOffset+int64(dataSize)]

	// 4. 解压缩
	var data []byte
	var err error
	if r.header.Compression == CompressionSnappy {
		data, err = snappy.Decode(nil, compressed)
		if err != nil {
			return nil, err
		}
	} else {
		data = compressed
	}

	// 5. 反序列化
	row, err := decodeRow(data)
	if err != nil {
		return nil, err
	}

	return row, nil
}

// GetHeader 获取文件头信息
func (r *Reader) GetHeader() *Header {
	return r.header
}

// GetPath 获取文件路径
func (r *Reader) GetPath() string {
	return r.path
}

// GetAllKeys 获取文件中所有的 key（按顺序）
func (r *Reader) GetAllKeys() []int64 {
	return r.btReader.GetAllKeys()
}

// Close 关闭读取器
func (r *Reader) Close() error {
	if r.mmap != nil {
		r.mmap.Unmap()
	}
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// decodeRow 解码行数据
func decodeRow(data []byte) (*Row, error) {
	// 尝试使用二进制格式解码
	row, err := decodeRowBinary(data)
	if err == nil {
		return row, nil
	}

	// 降级到 JSON (兼容旧数据)
	var decoded map[string]interface{}
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		return nil, err
	}

	row = &Row{
		Seq:  int64(decoded["_seq"].(float64)),
		Time: int64(decoded["_time"].(float64)),
		Data: decoded["data"].(map[string]interface{}),
	}

	return row, nil
}
