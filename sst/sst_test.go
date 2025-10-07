package sst

import (
	"os"
	"testing"
)

func TestSST(t *testing.T) {
	// 1. 创建测试文件
	file, err := os.Create("test.sst")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("test.sst")

	// 2. 写入数据
	writer := NewWriter(file)

	// 添加 1000 行数据
	for i := int64(1); i <= 1000; i++ {
		row := &Row{
			Seq:  i,
			Time: 1000000 + i,
			Data: map[string]interface{}{
				"name": "user_" + string(rune(i)),
				"age":  20 + i%50,
			},
		}
		err := writer.Add(row)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 完成写入
	err = writer.Finish()
	if err != nil {
		t.Fatal(err)
	}

	file.Close()

	t.Logf("Written 1000 rows")

	// 3. 读取数据
	reader, err := NewReader("test.sst")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	// 验证 Header
	header := reader.GetHeader()
	if header.RowCount != 1000 {
		t.Errorf("Expected 1000 rows, got %d", header.RowCount)
	}
	if header.MinKey != 1 {
		t.Errorf("Expected MinKey=1, got %d", header.MinKey)
	}
	if header.MaxKey != 1000 {
		t.Errorf("Expected MaxKey=1000, got %d", header.MaxKey)
	}

	t.Logf("Header: RowCount=%d, MinKey=%d, MaxKey=%d",
		header.RowCount, header.MinKey, header.MaxKey)

	// 4. 查询测试
	for i := int64(1); i <= 1000; i++ {
		row, err := reader.Get(i)
		if err != nil {
			t.Errorf("Failed to get key %d: %v", i, err)
			continue
		}
		if row.Seq != i {
			t.Errorf("Key %d: expected Seq=%d, got %d", i, i, row.Seq)
		}
		if row.Time != 1000000+i {
			t.Errorf("Key %d: expected Time=%d, got %d", i, 1000000+i, row.Time)
		}
	}

	// 测试不存在的 key
	_, err = reader.Get(1001)
	if err == nil {
		t.Error("Key 1001 should not exist")
	}

	_, err = reader.Get(0)
	if err == nil {
		t.Error("Key 0 should not exist")
	}

	t.Log("All tests passed!")
}

func TestHeaderSerialization(t *testing.T) {
	// 创建 Header
	header := &Header{
		Magic:       MagicNumber,
		Version:     Version,
		Compression: CompressionSnappy,
		IndexOffset: 256,
		IndexSize:   1024,
		RootOffset:  512,
		DataOffset:  2048,
		DataSize:    10240,
		RowCount:    100,
		MinKey:      1,
		MaxKey:      100,
		MinTime:     1000000,
		MaxTime:     1000100,
	}

	// 序列化
	data := header.Marshal()
	if len(data) != HeaderSize {
		t.Errorf("Expected size %d, got %d", HeaderSize, len(data))
	}

	// 反序列化
	header2 := UnmarshalHeader(data)
	if header2 == nil {
		t.Fatal("Unmarshal failed")
	}

	// 验证
	if header2.Magic != header.Magic {
		t.Error("Magic mismatch")
	}
	if header2.Version != header.Version {
		t.Error("Version mismatch")
	}
	if header2.Compression != header.Compression {
		t.Error("Compression mismatch")
	}
	if header2.RowCount != header.RowCount {
		t.Error("RowCount mismatch")
	}
	if header2.MinKey != header.MinKey {
		t.Error("MinKey mismatch")
	}
	if header2.MaxKey != header.MaxKey {
		t.Error("MaxKey mismatch")
	}

	// 验证
	if !header2.Validate() {
		t.Error("Header validation failed")
	}

	t.Log("Header serialization test passed!")
}

func BenchmarkSSTGet(b *testing.B) {
	// 创建测试文件
	file, _ := os.Create("bench.sst")
	defer os.Remove("bench.sst")

	writer := NewWriter(file)
	for i := int64(1); i <= 10000; i++ {
		row := &Row{
			Seq:  i,
			Time: 1000000 + i,
			Data: map[string]interface{}{
				"value": i,
			},
		}
		writer.Add(row)
	}
	writer.Finish()
	file.Close()

	// 打开读取器
	reader, _ := NewReader("bench.sst")
	defer reader.Close()

	// 性能测试
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := int64(i%10000 + 1)
		reader.Get(key)
	}
}
