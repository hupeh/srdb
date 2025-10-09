package srdb

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestIndexBTreeBasic(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建 Schema
	schema, err := NewSchema("test", []Field{
		{Name: "id", Type: Int64},
		{Name: "name", Type: String},
		{Name: "city", Type: String},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 创建索引管理器
	mgr := NewIndexManager(tmpDir, schema)
	defer mgr.Close()

	// 创建索引
	err = mgr.CreateIndex("city")
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// 添加测试数据
	testData := []struct {
		city string
		seq  int64
	}{
		{"Beijing", 1},
		{"Shanghai", 2},
		{"Beijing", 3},
		{"Shenzhen", 4},
		{"Shanghai", 5},
		{"Beijing", 6},
	}

	for _, td := range testData {
		data := map[string]any{
			"id":   td.seq,
			"name": "user_" + string(rune(td.seq)),
			"city": td.city,
		}
		err := mgr.AddToIndexes(data, td.seq)
		if err != nil {
			t.Fatalf("Failed to add to index: %v", err)
		}
	}

	// 构建索引
	err = mgr.BuildAll()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	// 关闭并重新打开，测试持久化
	mgr.Close()

	// 重新打开
	mgr2 := NewIndexManager(tmpDir, schema)
	defer mgr2.Close()

	// 查询索引
	idx, exists := mgr2.GetIndex("city")
	if !exists {
		t.Fatal("Index not found after reload")
	}

	// 验证索引使用 B+Tree
	if !idx.useBTree {
		t.Error("Index should be using B+Tree format")
	}

	// 验证查询结果
	testCases := []struct {
		city         string
		expectedSeqs []int64
	}{
		{"Beijing", []int64{1, 3, 6}},
		{"Shanghai", []int64{2, 5}},
		{"Shenzhen", []int64{4}},
		{"Unknown", nil},
	}

	for _, tc := range testCases {
		seqs, err := idx.Get(tc.city)
		if err != nil {
			t.Errorf("Failed to query index for %s: %v", tc.city, err)
			continue
		}

		if len(seqs) != len(tc.expectedSeqs) {
			t.Errorf("City %s: expected %d seqs, got %d", tc.city, len(tc.expectedSeqs), len(seqs))
			continue
		}

		// 验证 seq 值
		seqMap := make(map[int64]bool)
		for _, seq := range seqs {
			seqMap[seq] = true
		}

		for _, expectedSeq := range tc.expectedSeqs {
			if !seqMap[expectedSeq] {
				t.Errorf("City %s: missing expected seq %d", tc.city, expectedSeq)
			}
		}
	}

	// 验证元数据
	metadata := idx.GetMetadata()
	if metadata.MinSeq != 1 || metadata.MaxSeq != 6 || metadata.RowCount != 6 {
		t.Errorf("Invalid metadata: MinSeq=%d, MaxSeq=%d, RowCount=%d",
			metadata.MinSeq, metadata.MaxSeq, metadata.RowCount)
	}
}

func TestIndexBTreeLargeDataset(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建 Schema
	schema, err := NewSchema("test", []Field{
		{Name: "id", Type: Int64},
		{Name: "category", Type: String},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 创建索引管理器
	mgr := NewIndexManager(tmpDir, schema)
	defer mgr.Close()

	// 创建索引
	err = mgr.CreateIndex("category")
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// 添加大量测试数据
	numRecords := 10000
	numCategories := 100

	for i := 1; i <= numRecords; i++ {
		category := "cat_" + string(rune('A'+(i%numCategories)))
		data := map[string]any{
			"id":       int64(i),
			"category": category,
		}
		err := mgr.AddToIndexes(data, int64(i))
		if err != nil {
			t.Fatalf("Failed to add to index: %v", err)
		}
	}

	// 构建索引
	startBuild := time.Now()
	err = mgr.BuildAll()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}
	buildTime := time.Since(startBuild)
	t.Logf("Built index with %d records in %v", numRecords, buildTime)

	// 获取索引文件大小
	idx, _ := mgr.GetIndex("category")
	stat, _ := idx.file.Stat()
	t.Logf("Index file size: %d bytes", stat.Size())

	// 关闭并重新打开
	mgr.Close()

	// 重新打开
	mgr2 := NewIndexManager(tmpDir, schema)
	defer mgr2.Close()

	idx2, exists := mgr2.GetIndex("category")
	if !exists {
		t.Fatal("Index not found after reload")
	}

	// 验证索引使用 B+Tree
	if !idx2.useBTree {
		t.Error("Index should be using B+Tree format")
	}

	// 随机查询测试
	startQuery := time.Now()
	for i := 0; i < 100; i++ {
		category := "cat_" + string(rune('A'+(i%numCategories)))
		seqs, err := idx2.Get(category)
		if err != nil {
			t.Errorf("Failed to query index for %s: %v", category, err)
		}
		// 验证返回的 seq 数量
		expectedCount := numRecords / numCategories
		if len(seqs) != expectedCount {
			t.Errorf("Category %s: expected %d seqs, got %d", category, expectedCount, len(seqs))
		}
	}
	queryTime := time.Since(startQuery)
	t.Logf("Queried 100 categories in %v (avg: %v per query)", queryTime, queryTime/100)
}

func TestIndexBTreeBackwardCompatibility(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建 Schema
	schema, err := NewSchema("test", []Field{
		{Name: "id", Type: Int64},
		{Name: "status", Type: String},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. 创建索引管理器并用旧方式（通过先禁用新格式）创建索引
	mgr := NewIndexManager(tmpDir, schema)

	// 创建索引
	err = mgr.CreateIndex("status")
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// 添加���据
	testData := map[string][]int64{
		"active":   {1, 3, 5},
		"inactive": {2, 4},
	}

	for status, seqs := range testData {
		for _, seq := range seqs {
			data := map[string]any{
				"id":     seq,
				"status": status,
			}
			err := mgr.AddToIndexes(data, seq)
			if err != nil {
				t.Fatalf("Failed to add to index: %v", err)
			}
		}
	}

	// 构建索引（会使用新的 B+Tree 格式）
	err = mgr.BuildAll()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	// 关闭
	mgr.Close()

	// 2. 重新加载并验证
	mgr2 := NewIndexManager(tmpDir, schema)
	defer mgr2.Close()

	idx, exists := mgr2.GetIndex("status")
	if !exists {
		t.Fatal("Failed to load index")
	}

	// 应该使用 B+Tree 格式
	if !idx.useBTree {
		t.Error("Index should be using B+Tree format")
	}

	// 验证查询结果
	seqs, err := idx.Get("active")
	if err != nil || len(seqs) != 3 {
		t.Errorf("Failed to query index: err=%v, seqs=%v", err, seqs)
	}

	seqs, err = idx.Get("inactive")
	if err != nil || len(seqs) != 2 {
		t.Errorf("Failed to query index: err=%v, seqs=%v", err, seqs)
	}

	t.Log("Successfully loaded and queried B+Tree format index")
}

func TestIndexBTreeIncrementalUpdate(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建 Schema
	schema, err := NewSchema("test", []Field{
		{Name: "id", Type: Int64},
		{Name: "tag", Type: String},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 创建索引管理器
	mgr := NewIndexManager(tmpDir, schema)
	defer mgr.Close()

	// 创建索引
	err = mgr.CreateIndex("tag")
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// 添加初始数据
	for i := 1; i <= 100; i++ {
		tag := "tag_" + string(rune('A'+(i%10)))
		data := map[string]any{
			"id":  int64(i),
			"tag": tag,
		}
		err := mgr.AddToIndexes(data, int64(i))
		if err != nil {
			t.Fatalf("Failed to add to index: %v", err)
		}
	}

	// 构建索引
	err = mgr.BuildAll()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	// 获取索引
	idx, _ := mgr.GetIndex("tag")

	// 验证初始元数据
	metadata := idx.GetMetadata()
	if metadata.MaxSeq != 100 {
		t.Errorf("Expected MaxSeq=100, got %d", metadata.MaxSeq)
	}

	// 增量更新：添加新数据
	getData := func(seq int64) (map[string]any, error) {
		tag := "tag_" + string(rune('A'+(int(seq)%10)))
		return map[string]any{
			"id":  seq,
			"tag": tag,
		}, nil
	}

	err = idx.IncrementalUpdate(getData, 101, 200)
	if err != nil {
		t.Fatalf("Failed to incremental update: %v", err)
	}

	// 验证更新后的元数据
	metadata = idx.GetMetadata()
	if metadata.MaxSeq != 200 {
		t.Errorf("Expected MaxSeq=200 after update, got %d", metadata.MaxSeq)
	}
	if metadata.RowCount != 200 {
		t.Errorf("Expected RowCount=200 after update, got %d", metadata.RowCount)
	}

	// 验证可以查询到新数据
	seqs, err := idx.Get("tag_A")
	if err != nil {
		t.Fatalf("Failed to query index: %v", err)
	}

	// tag_A 应该包含 seq 1, 11, 21, ..., 191 (20个)
	if len(seqs) != 20 {
		t.Errorf("Expected 20 seqs for tag_A, got %d", len(seqs))
	}

	t.Logf("Incremental update successful: %d records indexed", metadata.RowCount)
}

// TestIndexBTreeWriter 测试 B+Tree 写入器
func TestIndexBTreeWriter(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test_idx.sst"

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	defer file.Close()

	// 创建写入器
	metadata := IndexMetadata{
		MinSeq:    1,
		MaxSeq:    10,
		RowCount:  10,
		CreatedAt: time.Now().UnixNano(),
		UpdatedAt: time.Now().UnixNano(),
	}

	writer := NewIndexBTreeWriter(file, metadata)

	// 添加测试数据
	testData := map[string][]int64{
		"apple":  {1, 2, 3},
		"banana": {4, 5},
		"cherry": {6, 7, 8, 9},
		"date":   {10},
	}

	for value, seqs := range testData {
		writer.Add(value, seqs)
	}

	// 构建索引
	err = writer.Build()
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	t.Log("B+Tree index written successfully")

	// 关闭并重新打开文件
	file.Close()

	// 读取索引
	file, err = os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to reopen file: %v", err)
	}

	reader, err := NewIndexBTreeReader(file)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// 验证读取
	for value, expectedSeqs := range testData {
		seqs, err := reader.Get(value)
		if err != nil {
			t.Errorf("Failed to get %s: %v", value, err)
			continue
		}

		if len(seqs) != len(expectedSeqs) {
			t.Errorf("%s: expected %d seqs, got %d", value, len(expectedSeqs), len(seqs))
			t.Logf("  Expected: %v", expectedSeqs)
			t.Logf("  Got: %v", seqs)
		} else {
			// 验证每个 seq
			seqMap := make(map[int64]bool)
			for _, seq := range seqs {
				seqMap[seq] = true
			}
			for _, expectedSeq := range expectedSeqs {
				if !seqMap[expectedSeq] {
					t.Errorf("%s: missing seq %d", value, expectedSeq)
				}
			}
		}
	}

	// 测试不存在的值
	seqs, err := reader.Get("unknown")
	if err != nil {
		t.Errorf("Failed to get unknown: %v", err)
	}
	if seqs != nil {
		t.Errorf("Expected nil for unknown value, got %v", seqs)
	}

	t.Log("All B+Tree reads successful")
}

// TestValueToKey 测试哈希函数
func TestValueToKey(t *testing.T) {
	testCases := []string{
		"apple",
		"banana",
		"cherry",
		"Beijing",
		"Shanghai",
		"Shenzhen",
	}

	keyMap := make(map[int64]string)
	for _, value := range testCases {
		key := valueToKey(value)
		t.Logf("valueToKey(%s) = %d", value, key)

		// 检查哈希冲突
		if existingValue, exists := keyMap[key]; exists {
			t.Errorf("Hash collision: %s and %s both hash to %d", value, existingValue, key)
		}
		keyMap[key] = value

		// 验证哈希的一致性
		key2 := valueToKey(value)
		if key != key2 {
			t.Errorf("Hash inconsistency for %s: %d != %d", value, key, key2)
		}
	}
}

// TestIndexBTreeDataTypes 测试不同数据类型
func TestIndexBTreeDataTypes(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test_types.sst"

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	defer file.Close()

	metadata := IndexMetadata{
		MinSeq:    1,
		MaxSeq:    5,
		RowCount:  5,
		CreatedAt: time.Now().UnixNano(),
		UpdatedAt: time.Now().UnixNano(),
	}

	writer := NewIndexBTreeWriter(file, metadata)

	// 测试不同类型的值（都转换为字符串）
	testData := map[string][]int64{
		"123":   {1}, // 数字字符串
		"true":  {2}, // 布尔字符串
		"hello": {3}, // 普通字符串
		"世界":    {4}, // 中文
		"":      {5}, // 空字符串
	}

	for value, seqs := range testData {
		writer.Add(value, seqs)
	}

	err = writer.Build()
	if err != nil {
		t.Fatalf("Failed to build: %v", err)
	}

	file.Close()

	// 重新读取
	file, err = os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to reopen: %v", err)
	}

	reader, err := NewIndexBTreeReader(file)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// 验证
	for value, expectedSeqs := range testData {
		seqs, err := reader.Get(value)
		if err != nil {
			t.Errorf("Failed to get '%s': %v", value, err)
			continue
		}

		if len(seqs) != len(expectedSeqs) {
			t.Errorf("'%s': expected %d seqs, got %d", value, len(expectedSeqs), len(seqs))
		}
	}

	t.Log("All data types tested successfully")
}

// 测试大数据量
func TestIndexBTreeLargeData(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := tmpDir + "/test_large.sst"

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	defer file.Close()

	metadata := IndexMetadata{
		MinSeq:    1,
		MaxSeq:    1000,
		RowCount:  1000,
		CreatedAt: time.Now().UnixNano(),
		UpdatedAt: time.Now().UnixNano(),
	}

	writer := NewIndexBTreeWriter(file, metadata)

	// 添加 1000 个不同的值
	for i := range 1000 {
		value := fmt.Sprintf("value_%d", i)
		seqs := []int64{int64(i + 1)}
		writer.Add(value, seqs)
	}

	err = writer.Build()
	if err != nil {
		t.Fatalf("Failed to build: %v", err)
	}

	fileInfo, _ := file.Stat()
	t.Logf("Index file size: %d bytes", fileInfo.Size())

	file.Close()

	// 重新读取
	file, err = os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to reopen: %v", err)
	}

	reader, err := NewIndexBTreeReader(file)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// 随机验证 100 个值
	for i := range 100 {
		value := fmt.Sprintf("value_%d", i*10)
		seqs, err := reader.Get(value)
		if err != nil {
			t.Errorf("Failed to get %s: %v", value, err)
			continue
		}

		if len(seqs) != 1 || seqs[0] != int64(i*10+1) {
			t.Errorf("%s: expected [%d], got %v", value, i*10+1, seqs)
		}
	}

	t.Log("Large data test successful")
}
