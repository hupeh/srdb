package srdb

import (
	"os"
	"testing"
	"time"
)

func TestEngineClean(t *testing.T) {
	dir := "./test_clean_data"
	defer os.RemoveAll(dir)

	// 1. 创建 Engine 并插入数据
	engine, err := OpenEngine(&EngineOptions{
		Dir: dir,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 插入一些数据
	for i := 0; i < 100; i++ {
		err := engine.Insert(map[string]any{
			"id":   i,
			"name": "test",
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// 强制 flush
	engine.Flush()
	time.Sleep(500 * time.Millisecond)

	// 验证数据存在
	stats := engine.Stats()
	t.Logf("Before Clean: MemTable=%d, SST=%d, Total=%d",
		stats.MemTableCount, stats.SSTCount, stats.TotalRows)

	if stats.TotalRows == 0 {
		t.Errorf("Expected some rows, got 0")
	}

	// 2. 清除数据
	err = engine.Clean()
	if err != nil {
		t.Fatal(err)
	}

	// 3. 验证数据已清除
	stats = engine.Stats()
	t.Logf("After Clean: MemTable=%d, SST=%d, Total=%d",
		stats.MemTableCount, stats.SSTCount, stats.TotalRows)

	if stats.TotalRows != 0 {
		t.Errorf("Expected 0 rows after clean, got %d", stats.TotalRows)
	}

	// 4. 验证 Engine 仍然可用
	err = engine.Insert(map[string]any{
		"id":   1,
		"name": "after_clean",
	})
	if err != nil {
		t.Fatal(err)
	}

	stats = engine.Stats()
	if stats.TotalRows != 1 {
		t.Errorf("Expected 1 row after insert, got %d", stats.TotalRows)
	}

	engine.Close()
}

func TestEngineDestroy(t *testing.T) {
	dir := "./test_destroy_data"
	defer os.RemoveAll(dir)

	// 1. 创建 Engine 并插入数据
	engine, err := OpenEngine(&EngineOptions{
		Dir: dir,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 插入一些数据
	for i := 0; i < 50; i++ {
		err := engine.Insert(map[string]any{
			"id":   i,
			"name": "test",
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// 验证数据存在
	stats := engine.Stats()
	t.Logf("Before Destroy: MemTable=%d, SST=%d, Total=%d",
		stats.MemTableCount, stats.SSTCount, stats.TotalRows)

	// 2. 销毁 Engine
	err = engine.Destroy()
	if err != nil {
		t.Fatal(err)
	}

	// 3. 验证数据目录已删除
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Data directory should be deleted")
	}

	// 4. 验证 Engine 不可用（尝试插入会失败）
	err = engine.Insert(map[string]any{
		"id":   1,
		"name": "after_destroy",
	})
	if err == nil {
		t.Errorf("Insert should fail after destroy")
	}
}

func TestEngineCleanWithSchema(t *testing.T) {
	dir := "./test_clean_schema_data"
	defer os.RemoveAll(dir)

	// 定义 Schema
	schema := NewSchema("test", []Field{
		{Name: "id", Type: FieldTypeInt64, Indexed: true, Comment: "ID"},
		{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "Name"},
	})

	// 1. 创建 Engine 并插入数据
	engine, err := OpenEngine(&EngineOptions{
		Dir:    dir,
		Schema: schema,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 创建索引
	err = engine.CreateIndex("id")
	if err != nil {
		t.Fatal(err)
	}

	// 插入数据
	for i := 0; i < 50; i++ {
		err := engine.Insert(map[string]any{
			"id":   int64(i),
			"name": "test",
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// 验证索引存在
	indexes := engine.ListIndexes()
	if len(indexes) != 1 {
		t.Errorf("Expected 1 index, got %d", len(indexes))
	}

	// 2. 清除数据
	err = engine.Clean()
	if err != nil {
		t.Fatal(err)
	}

	// 3. 验证数据已清除但 Schema 和索引结构保留
	stats := engine.Stats()
	if stats.TotalRows != 0 {
		t.Errorf("Expected 0 rows after clean, got %d", stats.TotalRows)
	}

	// 验证可以继续插入（Schema 仍然有效）
	err = engine.Insert(map[string]any{
		"id":   int64(100),
		"name": "after_clean",
	})
	if err != nil {
		t.Fatal(err)
	}

	engine.Close()
}

func TestEngineCleanAndReopen(t *testing.T) {
	dir := "./test_clean_reopen_data"
	defer os.RemoveAll(dir)

	// 1. 创建 Engine 并插入数据
	engine, err := OpenEngine(&EngineOptions{
		Dir: dir,
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 100; i++ {
		engine.Insert(map[string]any{
			"id":   i,
			"name": "test",
		})
	}

	// 2. 清除数据
	engine.Clean()

	// 3. 关闭并重新打开
	engine.Close()

	engine2, err := OpenEngine(&EngineOptions{
		Dir: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine2.Close()

	// 4. 验证数据为空
	stats := engine2.Stats()
	if stats.TotalRows != 0 {
		t.Errorf("Expected 0 rows after reopen, got %d", stats.TotalRows)
	}

	// 5. 验证可以插入新数据
	err = engine2.Insert(map[string]any{
		"id":   1,
		"name": "new_data",
	})
	if err != nil {
		t.Fatal(err)
	}

	stats = engine2.Stats()
	if stats.TotalRows != 1 {
		t.Errorf("Expected 1 row, got %d", stats.TotalRows)
	}
}
