package srdb

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestSSTable(t *testing.T) {
	// 1. 创建测试文件
	file, err := os.Create("test.sst")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("test.sst")

	// 2. 写入数据
	writer := NewSSTableWriter(file, nil)

	// 添加 1000 行数据
	for i := int64(1); i <= 1000; i++ {
		row := &SSTableRow{
			Seq:  i,
			Time: 1000000 + i,
			Data: map[string]any{
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
	reader, err := NewSSTableReader("test.sst")
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

func TestSSTableHeaderSerialization(t *testing.T) {
	// 创建 Header
	header := &SSTableHeader{
		Magic:       SSTableMagicNumber,
		Version:     SSTableVersion,
		Compression: SSTableCompressionSnappy,
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
	if len(data) != SSTableHeaderSize {
		t.Errorf("Expected size %d, got %d", SSTableHeaderSize, len(data))
	}

	// 反序列化
	header2 := UnmarshalSSTableHeader(data)
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

func BenchmarkSSTableGet(b *testing.B) {
	// 创建测试文件
	file, _ := os.Create("bench.sst")
	defer os.Remove("bench.sst")

	writer := NewSSTableWriter(file, nil)
	for i := int64(1); i <= 10000; i++ {
		row := &SSTableRow{
			Seq:  i,
			Time: 1000000 + i,
			Data: map[string]any{
				"value": i,
			},
		}
		writer.Add(row)
	}
	writer.Finish()
	file.Close()

	// 打开读取器
	reader, _ := NewSSTableReader("bench.sst")
	defer reader.Close()

	// 性能测试

	for i := 0; b.Loop(); i++ {
		key := int64(i%10000 + 1)
		reader.Get(key)
	}
}

func TestSSTableBinaryEncoding(t *testing.T) {
	// 创建 Schema
	schema := &Schema{
		Name: "users",
		Fields: []Field{
			{Name: "name", Type: FieldTypeString},
			{Name: "age", Type: FieldTypeInt64},
			{Name: "email", Type: FieldTypeString},
		},
	}

	// 创建测试数据
	row := &SSTableRow{
		Seq:  12345,
		Time: 1234567890,
		Data: map[string]any{
			"name":  "test_user",
			"age":   int64(25),
			"email": "test@example.com",
		},
	}

	// 编码
	encoded, err := encodeSSTableRowBinary(row, schema)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Encoded size: %d bytes", len(encoded))

	// 解码
	decoded, err := decodeSSTableRowBinary(encoded, schema)
	if err != nil {
		t.Fatal(err)
	}

	// 验证
	if decoded.Seq != row.Seq {
		t.Errorf("Seq mismatch: expected %d, got %d", row.Seq, decoded.Seq)
	}
	if decoded.Time != row.Time {
		t.Errorf("Time mismatch: expected %d, got %d", row.Time, decoded.Time)
	}
	if decoded.Data["name"] != row.Data["name"] {
		t.Errorf("Name mismatch")
	}

	t.Log("Binary encoding test passed!")
}

func TestSSTableEncodingComparison(t *testing.T) {
	// 创建 Schema
	schema := &Schema{
		Name: "users",
		Fields: []Field{
			{Name: "name", Type: FieldTypeString},
			{Name: "age", Type: FieldTypeInt64},
			{Name: "email", Type: FieldTypeString},
		},
	}

	row := &SSTableRow{
		Seq:  12345,
		Time: 1234567890,
		Data: map[string]any{
			"name":  "test_user",
			"age":   int64(25),
			"email": "test@example.com",
		},
	}

	// 二进制编码
	binaryEncoded, _ := encodeSSTableRowBinary(row, schema)

	// JSON 编码 (旧方式)
	jsonData := map[string]any{
		"_seq":  row.Seq,
		"_time": row.Time,
		"data":  row.Data,
	}
	jsonEncoded, _ := json.Marshal(jsonData)

	t.Logf("Binary size: %d bytes", len(binaryEncoded))
	t.Logf("JSON size: %d bytes", len(jsonEncoded))
	t.Logf("Space saved: %.1f%%", float64(len(jsonEncoded)-len(binaryEncoded))/float64(len(jsonEncoded))*100)

	if len(binaryEncoded) >= len(jsonEncoded) {
		t.Error("Binary encoding should be smaller than JSON")
	}
}

func BenchmarkSSTableBinaryEncoding(b *testing.B) {
	// 创建 Schema
	schema := &Schema{
		Name: "users",
		Fields: []Field{
			{Name: "name", Type: FieldTypeString},
			{Name: "age", Type: FieldTypeInt64},
			{Name: "email", Type: FieldTypeString},
		},
	}

	row := &SSTableRow{
		Seq:  12345,
		Time: 1234567890,
		Data: map[string]any{
			"name":  "test_user",
			"age":   int64(25),
			"email": "test@example.com",
		},
	}

	for b.Loop() {
		encodeSSTableRowBinary(row, schema)
	}
}

func BenchmarkSSTableJSONEncoding(b *testing.B) {
	row := &SSTableRow{
		Seq:  12345,
		Time: 1234567890,
		Data: map[string]any{
			"name":  "test_user",
			"age":   25,
			"email": "test@example.com",
		},
	}

	data := map[string]any{
		"_seq":  row.Seq,
		"_time": row.Time,
		"data":  row.Data,
	}

	for b.Loop() {
		json.Marshal(data)
	}
}

func TestSSTablePerFieldCompression(t *testing.T) {
	// 创建 Schema
	schema := &Schema{
		Name: "users",
		Fields: []Field{
			{Name: "name", Type: FieldTypeString, Indexed: false},
			{Name: "age", Type: FieldTypeInt64, Indexed: false},
			{Name: "email", Type: FieldTypeString, Indexed: false},
			{Name: "score", Type: FieldTypeFloat, Indexed: false},
		},
	}

	// 创建测试数据
	row := &SSTableRow{
		Seq:  12345,
		Time: 1234567890,
		Data: map[string]any{
			"name":  "test_user",
			"age":   int64(25),
			"email": "test@example.com",
			"score": 95.5,
		},
	}

	// 使用 Schema 编码（按字段压缩）
	encoded, err := encodeSSTableRowBinary(row, schema)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Per-field compressed size: %d bytes", len(encoded))

	// 完整解码
	decoded, err := decodeSSTableRowBinary(encoded, schema)
	if err != nil {
		t.Fatal(err)
	}

	// 验证完整数据
	if decoded.Seq != row.Seq {
		t.Errorf("Seq mismatch: expected %d, got %d", row.Seq, decoded.Seq)
	}
	if decoded.Time != row.Time {
		t.Errorf("Time mismatch: expected %d, got %d", row.Time, decoded.Time)
	}
	if decoded.Data["name"] != row.Data["name"] {
		t.Errorf("Name mismatch")
	}
	if decoded.Data["age"] != row.Data["age"] {
		t.Errorf("Age mismatch")
	}

	// 部分解码 - 只读取 name 和 age 字段
	partialDecoded, err := decodeSSTableRowBinaryPartial(encoded, schema, []string{"name", "age"})
	if err != nil {
		t.Fatal(err)
	}

	// 验证只包含请求的字段
	if len(partialDecoded.Data) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(partialDecoded.Data))
	}
	if _, ok := partialDecoded.Data["name"]; !ok {
		t.Error("Missing field: name")
	}
	if _, ok := partialDecoded.Data["age"]; !ok {
		t.Error("Missing field: age")
	}
	if _, ok := partialDecoded.Data["email"]; ok {
		t.Error("Should not have field: email")
	}
	if _, ok := partialDecoded.Data["score"]; ok {
		t.Error("Should not have field: score")
	}

	// 验证字段值正确
	if partialDecoded.Data["name"] != "test_user" {
		t.Errorf("Name mismatch: got %v", partialDecoded.Data["name"])
	}
	if partialDecoded.Data["age"] != int64(25) {
		t.Errorf("Age mismatch: got %v", partialDecoded.Data["age"])
	}

	t.Log("Per-field compression test passed!")
}

func TestSSTablePartialReadingPerformance(t *testing.T) {
	// 创建包含多个字段的 Schema
	schema := &Schema{
		Name: "events",
		Fields: []Field{
			{Name: "field1", Type: FieldTypeString, Indexed: false},
			{Name: "field2", Type: FieldTypeString, Indexed: false},
			{Name: "field3", Type: FieldTypeString, Indexed: false},
			{Name: "field4", Type: FieldTypeString, Indexed: false},
			{Name: "field5", Type: FieldTypeString, Indexed: false},
			{Name: "field6", Type: FieldTypeString, Indexed: false},
			{Name: "field7", Type: FieldTypeString, Indexed: false},
			{Name: "field8", Type: FieldTypeString, Indexed: false},
			{Name: "field9", Type: FieldTypeString, Indexed: false},
			{Name: "field10", Type: FieldTypeString, Indexed: false},
		},
	}

	// 创建包含大量数据��行
	largeData := make(map[string]any)
	for i := 1; i <= 10; i++ {
		// 每个字段包含较大的字符串数据
		largeData[fmt.Sprintf("field%d", i)] = fmt.Sprintf("This is a large data field %d with lots of content to compress", i) +
			" Lorem ipsum dolor sit amet, consectetur adipiscing elit."
	}

	row := &SSTableRow{
		Seq:  12345,
		Time: 1234567890,
		Data: largeData,
	}

	// 编码
	encoded, err := encodeSSTableRowBinary(row, schema)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Total encoded size with 10 fields: %d bytes", len(encoded))

	// 完整解码
	fullDecoded, _ := decodeSSTableRowBinary(encoded, schema)
	t.Logf("Full decode: %d fields", len(fullDecoded.Data))

	// 部分解码 - 只读取 1 个字段
	partialDecoded, _ := decodeSSTableRowBinaryPartial(encoded, schema, []string{"field1"})
	t.Logf("Partial decode (1 field): %d fields", len(partialDecoded.Data))

	// 验证部分读取确实只返回请求的字段
	if len(partialDecoded.Data) != 1 {
		t.Errorf("Expected 1 field, got %d", len(partialDecoded.Data))
	}

	t.Log("Partial reading performance test passed!")
}
