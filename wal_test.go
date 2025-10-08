package srdb

import (
	"os"
	"testing"
)

func TestWAL(t *testing.T) {
	// 1. 创建 WAL
	wal, err := OpenWAL("test.wal")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("test.wal")

	// 2. 写入数据
	for i := int64(1); i <= 100; i++ {
		entry := &WALEntry{
			Type: WALEntryTypePut,
			Seq:  i,
			Data: []byte("value_" + string(rune(i))),
		}
		err := wal.Append(entry)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 3. Sync
	err = wal.Sync()
	if err != nil {
		t.Fatal(err)
	}

	wal.Close()

	t.Log("Written 100 entries")

	// 4. 读取数据
	reader, err := NewWALReader("test.wal")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	entries, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 100 {
		t.Errorf("Expected 100 entries, got %d", len(entries))
	}

	// 验证数据
	for i, entry := range entries {
		expectedSeq := int64(i + 1)
		if entry.Seq != expectedSeq {
			t.Errorf("Entry %d: expected Seq=%d, got %d", i, expectedSeq, entry.Seq)
		}
		if entry.Type != WALEntryTypePut {
			t.Errorf("Entry %d: expected Type=%d, got %d", i, WALEntryTypePut, entry.Type)
		}
	}

	t.Log("All tests passed!")
}

func TestWALTruncate(t *testing.T) {
	// 创建 WAL
	wal, err := OpenWAL("test_truncate.wal")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove("test_truncate.wal")

	// 写入数据
	for i := int64(1); i <= 10; i++ {
		entry := &WALEntry{
			Type: WALEntryTypePut,
			Seq:  i,
			Data: []byte("value"),
		}
		wal.Append(entry)
	}

	// Truncate
	err = wal.Truncate()
	if err != nil {
		t.Fatal(err)
	}

	wal.Close()

	// 验证文件为空
	reader, err := NewWALReader("test_truncate.wal")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	entries, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries after truncate, got %d", len(entries))
	}

	t.Log("Truncate test passed!")
}

func BenchmarkWALAppend(b *testing.B) {
	wal, _ := OpenWAL("bench.wal")
	defer os.Remove("bench.wal")
	defer wal.Close()

	entry := &WALEntry{
		Type: WALEntryTypePut,
		Seq:  1,
		Data: make([]byte, 100),
	}

	for i := 0; b.Loop(); i++ {
		entry.Seq = int64(i)
		wal.Append(entry)
	}
}
