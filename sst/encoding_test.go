package sst

import (
	"encoding/json"
	"testing"
)

func TestBinaryEncoding(t *testing.T) {
	// 创建测试数据
	row := &Row{
		Seq:  12345,
		Time: 1234567890,
		Data: map[string]interface{}{
			"name":  "test_user",
			"age":   25,
			"email": "test@example.com",
		},
	}

	// 编码
	encoded, err := encodeRowBinary(row)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Encoded size: %d bytes", len(encoded))

	// 解码
	decoded, err := decodeRowBinary(encoded)
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

func TestEncodingComparison(t *testing.T) {
	row := &Row{
		Seq:  12345,
		Time: 1234567890,
		Data: map[string]interface{}{
			"name":  "test_user",
			"age":   25,
			"email": "test@example.com",
		},
	}

	// 二进制编码
	binaryEncoded, _ := encodeRowBinary(row)

	// JSON 编码 (旧方式)
	jsonData := map[string]interface{}{
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

func BenchmarkBinaryEncoding(b *testing.B) {
	row := &Row{
		Seq:  12345,
		Time: 1234567890,
		Data: map[string]interface{}{
			"name":  "test_user",
			"age":   25,
			"email": "test@example.com",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encodeRowBinary(row)
	}
}

func BenchmarkJSONEncoding(b *testing.B) {
	row := &Row{
		Seq:  12345,
		Time: 1234567890,
		Data: map[string]interface{}{
			"name":  "test_user",
			"age":   25,
			"email": "test@example.com",
		},
	}

	data := map[string]interface{}{
		"_seq":  row.Seq,
		"_time": row.Time,
		"data":  row.Data,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(data)
	}
}
