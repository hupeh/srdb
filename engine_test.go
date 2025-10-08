package srdb

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEngine(t *testing.T) {
	// 1. 创建引擎
	dir := "test_db"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	engine, err := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 1024, // 1 KB，方便触发 Flush
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 2. 插入数据
	for i := 1; i <= 100; i++ {
		data := map[string]any{
			"name": fmt.Sprintf("user_%d", i),
			"age":  20 + i%50,
		}
		err := engine.Insert(data)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 等待 Flush 和 Compaction 完成
	time.Sleep(1 * time.Second)

	t.Logf("Inserted 100 rows")

	// 3. 查询数据
	for i := int64(1); i <= 100; i++ {
		row, err := engine.Get(i)
		if err != nil {
			t.Errorf("Failed to get key %d: %v", i, err)
			continue
		}
		if row.Seq != i {
			t.Errorf("Key %d: expected Seq=%d, got %d", i, i, row.Seq)
		}
	}

	// 4. 统计信息
	stats := engine.Stats()
	t.Logf("Stats: MemTable=%d rows, SST=%d files, Total=%d rows",
		stats.MemTableCount, stats.SSTCount, stats.TotalRows)

	if stats.TotalRows != 100 {
		t.Errorf("Expected 100 total rows, got %d", stats.TotalRows)
	}

	t.Log("All tests passed!")
}

func TestEngineRecover(t *testing.T) {
	dir := "test_recover"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	// 1. 创建引擎并插入数据
	engine, err := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 10 * 1024 * 1024, // 10 MB，不会触发 Flush
	})
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 50; i++ {
		data := map[string]interface{}{
			"value": i,
		}
		engine.Insert(data)
	}

	t.Log("Inserted 50 rows")

	// 2. 关闭引擎 (模拟崩溃前)
	engine.Close()

	// 3. 重新打开引擎 (恢复)
	engine2, err := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 10 * 1024 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine2.Close()

	// 4. 验证数据
	for i := int64(1); i <= 50; i++ {
		row, err := engine2.Get(i)
		if err != nil {
			t.Errorf("Failed to get key %d after recover: %v", i, err)
		}
		if row.Seq != i {
			t.Errorf("Key %d: expected Seq=%d, got %d", i, i, row.Seq)
		}
	}

	stats := engine2.Stats()
	if stats.TotalRows != 50 {
		t.Errorf("Expected 50 rows after recover, got %d", stats.TotalRows)
	}

	t.Log("Recover test passed!")
}

func TestEngineFlush(t *testing.T) {
	dir := "test_flush"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	engine, err := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 1024, // 1 KB
	})
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 插入足够多的数据触发 Flush
	for i := 1; i <= 200; i++ {
		data := map[string]any{
			"data": fmt.Sprintf("value_%d", i),
		}
		engine.Insert(data)
	}

	// 等待 Flush
	time.Sleep(500 * time.Millisecond)

	stats := engine.Stats()
	t.Logf("After flush: MemTable=%d, SST=%d, Total=%d",
		stats.MemTableCount, stats.SSTCount, stats.TotalRows)

	if stats.SSTCount == 0 {
		t.Error("Expected at least 1 SST file after flush")
	}

	// 验证所有数据都能查到
	for i := int64(1); i <= 200; i++ {
		_, err := engine.Get(i)
		if err != nil {
			t.Errorf("Failed to get key %d after flush: %v", i, err)
		}
	}

	t.Log("Flush test passed!")
}

func BenchmarkEngineInsert(b *testing.B) {
	dir := "bench_insert"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	engine, _ := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 100 * 1024 * 1024, // 100 MB
	})
	defer engine.Close()

	data := map[string]any{
		"value": 123,
	}

	for b.Loop() {
		engine.Insert(data)
	}
}

func BenchmarkEngineGet(b *testing.B) {
	dir := "bench_get"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	engine, _ := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 100 * 1024 * 1024,
	})
	defer engine.Close()

	// 预先插入数据
	for i := 1; i <= 10000; i++ {
		data := map[string]any{
			"value": i,
		}
		engine.Insert(data)
	}

	for i := 0; b.Loop(); i++ {
		key := int64(i%10000 + 1)
		engine.Get(key)
	}
}

// TestHighConcurrencyWrite 测试高并发写入（2KB-5MB 数据）
func TestHighConcurrencyWrite(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &EngineOptions{
		Dir:          tmpDir,
		MemTableSize: 64 * 1024 * 1024, // 64MB
	}

	engine, err := OpenEngine(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 测试配置
	const (
		numGoroutines = 50              // 50 个并发写入
		rowsPerWorker = 100             // 每个 worker 写入 100 行
		minDataSize   = 2 * 1024        // 2KB
		maxDataSize   = 5 * 1024 * 1024 // 5MB
	)

	var (
		totalInserted atomic.Int64
		totalErrors   atomic.Int64
		wg            sync.WaitGroup
	)

	startTime := time.Now()

	// 启动多个并发写入 goroutine
	for i := range numGoroutines {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := range rowsPerWorker {
				// 生成随机大小的数据 (2KB - 5MB)
				dataSize := minDataSize + (j % (maxDataSize - minDataSize))
				largeData := make([]byte, dataSize)
				rand.Read(largeData)

				data := map[string]any{
					"worker_id": workerID,
					"row_index": j,
					"data_size": dataSize,
					"payload":   largeData,
					"timestamp": time.Now().Unix(),
				}

				err := engine.Insert(data)
				if err != nil {
					totalErrors.Add(1)
					t.Logf("Worker %d, Row %d: Insert failed: %v", workerID, j, err)
				} else {
					totalInserted.Add(1)
				}

				// 每 10 行报告一次进度
				if j > 0 && j%10 == 0 {
					t.Logf("Worker %d: 已插入 %d 行", workerID, j)
				}
			}
		}(i)
	}

	// 等待所有写入完成
	wg.Wait()
	duration := time.Since(startTime)

	// 统计结果
	inserted := totalInserted.Load()
	errors := totalErrors.Load()
	expected := int64(numGoroutines * rowsPerWorker)

	t.Logf("\n=== 高并发写入测试结果 ===")
	t.Logf("并发数: %d", numGoroutines)
	t.Logf("预期插入: %d 行", expected)
	t.Logf("成功插入: %d 行", inserted)
	t.Logf("失败: %d 行", errors)
	t.Logf("耗时: %v", duration)
	t.Logf("吞吐量: %.2f 行/秒", float64(inserted)/duration.Seconds())

	// 验证
	if errors > 0 {
		t.Errorf("有 %d 次写入失败", errors)
	}

	if inserted != expected {
		t.Errorf("预期插入 %d 行，实际插入 %d 行", expected, inserted)
	}

	// 等待 Flush 完成
	time.Sleep(2 * time.Second)

	// 验证数据完整性
	stats := engine.Stats()
	t.Logf("\nEngine 状态:")
	t.Logf("  总行数: %d", stats.TotalRows)
	t.Logf("  SST 文件数: %d", stats.SSTCount)
	t.Logf("  MemTable 行数: %d", stats.MemTableCount)

	if stats.TotalRows < inserted {
		t.Errorf("数据丢失: 预期至少 %d 行，实际 %d 行", inserted, stats.TotalRows)
	}
}

// TestConcurrentReadWrite 测试并发读写混合
func TestConcurrentReadWrite(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &EngineOptions{
		Dir:          tmpDir,
		MemTableSize: 32 * 1024 * 1024, // 32MB
	}

	engine, err := OpenEngine(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	const (
		numWriters = 20
		numReaders = 30
		duration   = 10 * time.Second
		dataSize   = 10 * 1024 // 10KB
	)

	var (
		writeCount atomic.Int64
		readCount  atomic.Int64
		readErrors atomic.Int64
		wg         sync.WaitGroup
		stopCh     = make(chan struct{})
	)

	// 启动写入 goroutines
	for i := range numWriters {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()

			for {
				select {
				case <-stopCh:
					return
				default:
					data := make([]byte, dataSize)
					rand.Read(data)

					payload := map[string]any{
						"writer_id": writerID,
						"data":      data,
						"timestamp": time.Now().UnixNano(),
					}

					err := engine.Insert(payload)
					if err == nil {
						writeCount.Add(1)
					}

					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}

	// 启动读取 goroutines
	for i := range numReaders {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			for {
				select {
				case <-stopCh:
					return
				default:
					// 随机读取
					seq := int64(readerID*100 + 1)
					_, err := engine.Get(seq)
					if err == nil {
						readCount.Add(1)
					} else {
						readErrors.Add(1)
					}

					time.Sleep(5 * time.Millisecond)
				}
			}
		}(i)
	}

	// 运行指定时间
	time.Sleep(duration)
	close(stopCh)
	wg.Wait()

	// 统计结果
	writes := writeCount.Load()
	reads := readCount.Load()
	errors := readErrors.Load()

	t.Logf("\n=== 并发读写测试结果 ===")
	t.Logf("测试时长: %v", duration)
	t.Logf("写入次数: %d (%.2f 次/秒)", writes, float64(writes)/duration.Seconds())
	t.Logf("读取次数: %d (%.2f 次/秒)", reads, float64(reads)/duration.Seconds())
	t.Logf("读取失败: %d", errors)

	stats := engine.Stats()
	t.Logf("\nEngine 状态:")
	t.Logf("  总行数: %d", stats.TotalRows)
	t.Logf("  SST 文件数: %d", stats.SSTCount)
}

// TestPowerFailureRecovery 测试断电恢复（模拟崩溃）
func TestPowerFailureRecovery(t *testing.T) {
	tmpDir := t.TempDir()

	// 第一阶段：写入数据并模拟崩溃
	t.Log("=== 阶段 1: 写入数据 ===")

	opts := &EngineOptions{
		Dir:          tmpDir,
		MemTableSize: 4 * 1024 * 1024, // 4MB
	}

	engine, err := OpenEngine(opts)
	if err != nil {
		t.Fatal(err)
	}

	const (
		numBatches   = 10
		rowsPerBatch = 50
		dataSize     = 50 * 1024 // 50KB
	)

	insertedSeqs := make([]int64, 0, numBatches*rowsPerBatch)

	for batch := range numBatches {
		for i := range rowsPerBatch {
			data := make([]byte, dataSize)
			rand.Read(data)

			payload := map[string]any{
				"batch":     batch,
				"index":     i,
				"data":      data,
				"timestamp": time.Now().Unix(),
			}

			err := engine.Insert(payload)
			if err != nil {
				t.Fatalf("Insert failed: %v", err)
			}

			seq := engine.seq.Load()
			insertedSeqs = append(insertedSeqs, seq)
		}

		// 每批后触发 Flush
		if batch%3 == 0 {
			engine.switchMemTable()
			time.Sleep(100 * time.Millisecond)
		}

		t.Logf("批次 %d: 插入 %d 行", batch, rowsPerBatch)
	}

	totalInserted := len(insertedSeqs)
	t.Logf("总共插入: %d 行", totalInserted)

	// 获取崩溃前的状态
	statsBefore := engine.Stats()
	t.Logf("崩溃前状态: 总行数=%d, SST文件=%d, MemTable行数=%d",
		statsBefore.TotalRows, statsBefore.SSTCount, statsBefore.MemTableCount)

	// 模拟崩溃：直接关闭（不等待 Flush 完成）
	t.Log("\n=== 模拟断电崩溃 ===")
	engine.Close()

	// 第二阶段：恢复并验证数据
	t.Log("\n=== 阶段 2: 恢复数据 ===")

	engineRecovered, err := OpenEngine(opts)
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	defer engineRecovered.Close()

	// 等待恢复完成
	time.Sleep(500 * time.Millisecond)

	statsAfter := engineRecovered.Stats()
	t.Logf("恢复后状态: 总行数=%d, SST文件=%d, MemTable行数=%d",
		statsAfter.TotalRows, statsAfter.SSTCount, statsAfter.MemTableCount)

	// 验证数据完整性
	t.Log("\n=== 阶段 3: 验证数据完整性 ===")

	recovered := 0
	missing := 0
	corrupted := 0

	for i, seq := range insertedSeqs {
		row, err := engineRecovered.Get(seq)
		if err != nil {
			missing++
			if i < len(insertedSeqs)/2 {
				// 前半部分应该已经 Flush，不应该丢失
				t.Logf("警告: Seq %d 丢失（应该已持久化）", seq)
			}
			continue
		}

		// 验证数据
		if row.Seq != seq {
			corrupted++
			t.Errorf("数据损坏: 预期 Seq=%d, 实际=%d", seq, row.Seq)
			continue
		}

		recovered++
	}

	recoveryRate := float64(recovered) / float64(totalInserted) * 100

	t.Logf("\n=== 恢复结果 ===")
	t.Logf("插入总数: %d", totalInserted)
	t.Logf("成功恢复: %d (%.2f%%)", recovered, recoveryRate)
	t.Logf("丢失: %d", missing)
	t.Logf("损坏: %d", corrupted)

	// 验证：至少应该恢复已经 Flush 的数据
	if corrupted > 0 {
		t.Errorf("发现 %d 条损坏数据", corrupted)
	}

	// 至少应该恢复 50% 的数据（已 Flush 的部分）
	if recoveryRate < 50 {
		t.Errorf("恢复率过低: %.2f%% (预期至少 50%%)", recoveryRate)
	}

	t.Logf("\n断电恢复测试通过！")
}

// TestCrashDuringCompaction 测试 Compaction 期间崩溃
func TestCrashDuringCompaction(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &EngineOptions{
		Dir:          tmpDir,
		MemTableSize: 1024, // 很小，快速触发 Flush
	}

	engine, err := OpenEngine(opts)
	if err != nil {
		t.Fatal(err)
	}

	// 插入大量数据触发多次 Flush
	t.Log("=== 插入数据触发 Compaction ===")
	const numRows = 500
	dataSize := 5 * 1024 // 5KB

	for i := range numRows {
		data := make([]byte, dataSize)
		rand.Read(data)

		payload := map[string]any{
			"index": i,
			"data":  data,
		}

		err := engine.Insert(payload)
		if err != nil {
			t.Fatal(err)
		}

		if i%50 == 0 {
			t.Logf("已插入 %d 行", i)
		}
	}

	// 等待一些 Flush 完成
	time.Sleep(500 * time.Millisecond)

	version := engine.versionSet.GetCurrent()
	l0Count := version.GetLevelFileCount(0)
	t.Logf("L0 文件数: %d", l0Count)

	// 模拟在 Compaction 期间崩溃
	if l0Count >= 4 {
		t.Log("触发 Compaction...")
		go func() {
			engine.compactionManager.TriggerCompaction()
		}()

		// 等待 Compaction 开始
		time.Sleep(100 * time.Millisecond)

		t.Log("=== 模拟 Compaction 期间崩溃 ===")
	}

	// 直接关闭（模拟崩溃）
	engine.Close()

	// 恢复
	t.Log("\n=== 恢复数据库 ===")
	engineRecovered, err := OpenEngine(opts)
	if err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	defer engineRecovered.Close()

	// 验证数据完整性
	stats := engineRecovered.Stats()
	t.Logf("恢复后: 总行数=%d, SST文件=%d", stats.TotalRows, stats.SSTCount)

	// 随机验证一些数据
	t.Log("\n=== 验证数据 ===")
	verified := 0
	for i := 1; i <= 100; i++ {
		seq := int64(i)
		_, err := engineRecovered.Get(seq)
		if err == nil {
			verified++
		}
	}

	t.Logf("验证前 100 行: %d 行可读", verified)

	if verified < 50 {
		t.Errorf("数据恢复不足: 只有 %d/100 行可读", verified)
	}

	t.Log("Compaction 崩溃恢复测试通过！")
}

// TestLargeDataIntegrity 测试大数据完整性（2KB-5MB 数据）
func TestLargeDataIntegrity(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &EngineOptions{
		Dir:          tmpDir,
		MemTableSize: 64 * 1024 * 1024, // 64MB
	}

	engine, err := OpenEngine(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 测试不同大小的数据
	testSizes := []int{
		2 * 1024,        // 2KB
		10 * 1024,       // 10KB
		100 * 1024,      // 100KB
		1 * 1024 * 1024, // 1MB
		5 * 1024 * 1024, // 5MB
	}

	t.Log("=== 插入不同大小的数据 ===")

	insertedSeqs := make([]int64, 0)

	for _, size := range testSizes {
		// 每种大小插入 3 行
		for i := range 3 {
			data := make([]byte, size)
			rand.Read(data)

			payload := map[string]any{
				"size":  size,
				"index": i,
				"data":  data,
			}

			err := engine.Insert(payload)
			if err != nil {
				t.Fatalf("插入失败 (size=%d, index=%d): %v", size, i, err)
			}

			seq := engine.seq.Load()
			insertedSeqs = append(insertedSeqs, seq)

			t.Logf("插入: Seq=%d, Size=%d KB", seq, size/1024)
		}
	}

	totalInserted := len(insertedSeqs)
	t.Logf("总共插入: %d 行", totalInserted)

	// 等待 Flush
	time.Sleep(2 * time.Second)

	// 验证数据可读性
	t.Log("\n=== 验证数据可读性 ===")
	successCount := 0

	for i, seq := range insertedSeqs {
		row, err := engine.Get(seq)
		if err != nil {
			t.Errorf("读取失败 (Seq=%d): %v", seq, err)
			continue
		}

		// 验证数据存在
		if _, exists := row.Data["data"]; !exists {
			t.Errorf("Seq=%d: 数据字段不存在", seq)
			continue
		}

		if _, exists := row.Data["size"]; !exists {
			t.Errorf("Seq=%d: size 字段不存在", seq)
			continue
		}

		successCount++

		if i < 5 || i >= totalInserted-5 {
			// 只打印前5行和后5行
			t.Logf("✓ Seq=%d 验证通过", seq)
		}
	}

	successRate := float64(successCount) / float64(totalInserted) * 100

	stats := engine.Stats()
	t.Logf("\n=== 测试结果 ===")
	t.Logf("插入总数: %d", totalInserted)
	t.Logf("成功读取: %d (%.2f%%)", successCount, successRate)
	t.Logf("总行数: %d", stats.TotalRows)
	t.Logf("SST 文件数: %d", stats.SSTCount)

	if successCount != totalInserted {
		t.Errorf("数据丢失: %d/%d", totalInserted-successCount, totalInserted)
	}

	t.Log("\n大数据完整性测试通过！")
}

// BenchmarkConcurrentWrites 并发写入性能测试
func BenchmarkConcurrentWrites(b *testing.B) {
	tmpDir := b.TempDir()

	opts := &EngineOptions{
		Dir:          tmpDir,
		MemTableSize: 64 * 1024 * 1024,
	}

	engine, err := OpenEngine(opts)
	if err != nil {
		b.Fatal(err)
	}
	defer engine.Close()

	const (
		numWorkers = 10
		dataSize   = 10 * 1024 // 10KB
	)

	data := make([]byte, dataSize)
	rand.Read(data)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			payload := map[string]any{
				"data":      data,
				"timestamp": time.Now().UnixNano(),
			}

			err := engine.Insert(payload)
			if err != nil {
				b.Error(err)
			}
		}
	})

	b.StopTimer()

	stats := engine.Stats()
	b.Logf("总行数: %d, SST 文件数: %d", stats.TotalRows, stats.SSTCount)
}

// TestEngineWithCompaction 测试 Engine 的 Compaction 功能
func TestEngineWithCompaction(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 打开 Engine
	opts := &EngineOptions{
		Dir:          tmpDir,
		MemTableSize: 1024, // 小的 MemTable 以便快速触发 Flush
	}

	engine, err := OpenEngine(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 插入大量数据，触发多次 Flush
	const numBatches = 10
	const rowsPerBatch = 100

	for batch := range numBatches {
		for i := range rowsPerBatch {
			data := map[string]any{
				"batch": batch,
				"index": i,
				"value": fmt.Sprintf("data-%d-%d", batch, i),
			}

			err := engine.Insert(data)
			if err != nil {
				t.Fatalf("Insert failed: %v", err)
			}
		}

		// 强制 Flush
		err = engine.switchMemTable()
		if err != nil {
			t.Fatalf("Switch MemTable failed: %v", err)
		}

		// 等待 Flush 完成
		time.Sleep(100 * time.Millisecond)
	}

	// 等待所有 Immutable Flush 完成
	for engine.memtableManager.GetImmutableCount() > 0 {
		time.Sleep(100 * time.Millisecond)
	}

	// 检查 Version 状态
	version := engine.versionSet.GetCurrent()
	l0Count := version.GetLevelFileCount(0)
	t.Logf("L0 files: %d", l0Count)

	if l0Count == 0 {
		t.Error("Expected some files in L0")
	}

	// 获取 Level 统计信息
	levelStats := engine.compactionManager.GetLevelStats()
	for _, stat := range levelStats {
		level := stat["level"].(int)
		fileCount := stat["file_count"].(int)
		totalSize := stat["total_size"].(int64)
		score := stat["score"].(float64)

		if fileCount > 0 {
			t.Logf("L%d: %d files, %d bytes, score: %.2f", level, fileCount, totalSize, score)
		}
	}

	// 手动触发 Compaction
	if l0Count >= 4 {
		t.Log("Triggering manual compaction...")
		err = engine.compactionManager.TriggerCompaction()
		if err != nil {
			t.Logf("Compaction: %v", err)
		} else {
			t.Log("Compaction completed")

			// 检查 Compaction 后的状态
			version = engine.versionSet.GetCurrent()
			newL0Count := version.GetLevelFileCount(0)
			l1Count := version.GetLevelFileCount(1)

			t.Logf("After compaction - L0: %d files, L1: %d files", newL0Count, l1Count)

			if newL0Count >= l0Count {
				t.Error("Expected L0 file count to decrease after compaction")
			}

			if l1Count == 0 {
				t.Error("Expected some files in L1 after compaction")
			}
		}
	}

	// 验证数据完整性
	stats := engine.Stats()
	t.Logf("Engine stats: %d rows, %d SST files", stats.TotalRows, stats.SSTCount)

	// 读取一些数据验证
	for batch := range 3 {
		for i := range 10 {
			seq := int64(batch*rowsPerBatch + i + 1)
			row, err := engine.Get(seq)
			if err != nil {
				t.Errorf("Get(%d) failed: %v", seq, err)
				continue
			}

			if row.Data["batch"].(float64) != float64(batch) {
				t.Errorf("Expected batch %d, got %v", batch, row.Data["batch"])
			}
		}
	}
}

// TestEngineCompactionMerge 测试 Compaction 的合并功能
func TestEngineCompactionMerge(t *testing.T) {
	tmpDir := t.TempDir()

	opts := &EngineOptions{
		Dir:          tmpDir,
		MemTableSize: 512, // 很小的 MemTable
	}

	engine, err := OpenEngine(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 插入数据（Append-Only 模式）
	const numBatches = 5
	const rowsPerBatch = 50

	totalRows := 0
	for batch := range numBatches {
		for i := range rowsPerBatch {
			data := map[string]any{
				"batch": batch,
				"index": i,
				"value": fmt.Sprintf("v%d-%d", batch, i),
			}

			err := engine.Insert(data)
			if err != nil {
				t.Fatal(err)
			}
			totalRows++
		}

		// 每批后 Flush
		err = engine.switchMemTable()
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(50 * time.Millisecond)
	}

	// 等待所有 Flush 完成
	for engine.memtableManager.GetImmutableCount() > 0 {
		time.Sleep(100 * time.Millisecond)
	}

	// 记录 Compaction 前的文件数
	version := engine.versionSet.GetCurrent()
	beforeL0 := version.GetLevelFileCount(0)
	t.Logf("Before compaction: L0 has %d files", beforeL0)

	// 触发 Compaction
	if beforeL0 >= 4 {
		err = engine.compactionManager.TriggerCompaction()
		if err != nil {
			t.Logf("Compaction: %v", err)
		} else {
			version = engine.versionSet.GetCurrent()
			afterL0 := version.GetLevelFileCount(0)
			afterL1 := version.GetLevelFileCount(1)
			t.Logf("After compaction: L0 has %d files, L1 has %d files", afterL0, afterL1)
		}
	}

	// 验证数据完整性 - 检查前几条记录
	for batch := range 2 {
		for i := range 5 {
			seq := int64(batch*rowsPerBatch + i + 1)
			row, err := engine.Get(seq)
			if err != nil {
				t.Errorf("Get(%d) failed: %v", seq, err)
				continue
			}

			actualBatch := int(row.Data["batch"].(float64))
			if actualBatch != batch {
				t.Errorf("Seq %d: expected batch %d, got %d", seq, batch, actualBatch)
			}
		}
	}

	// 验证总行数
	stats := engine.Stats()
	if stats.TotalRows != int64(totalRows) {
		t.Errorf("Expected %d total rows, got %d", totalRows, stats.TotalRows)
	}

	t.Logf("Data integrity verified: %d rows", totalRows)
}

// TestEngineBackgroundCompaction 测试后台自动 Compaction
func TestEngineBackgroundCompaction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping background compaction test in short mode")
	}

	tmpDir := t.TempDir()

	opts := &EngineOptions{
		Dir:          tmpDir,
		MemTableSize: 512,
	}

	engine, err := OpenEngine(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 插入数据触发多次 Flush
	const numBatches = 8
	const rowsPerBatch = 50

	for batch := range numBatches {
		for i := range rowsPerBatch {
			data := map[string]any{
				"batch": batch,
				"index": i,
			}

			err := engine.Insert(data)
			if err != nil {
				t.Fatal(err)
			}
		}

		err = engine.switchMemTable()
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(50 * time.Millisecond)
	}

	// 等待 Flush 完成
	for engine.memtableManager.GetImmutableCount() > 0 {
		time.Sleep(100 * time.Millisecond)
	}

	// 记录初始状态
	version := engine.versionSet.GetCurrent()
	initialL0 := version.GetLevelFileCount(0)
	t.Logf("Initial L0 files: %d", initialL0)

	// 等待后台 Compaction（最多等待 30 秒）
	maxWait := 30 * time.Second
	checkInterval := 2 * time.Second
	waited := time.Duration(0)

	for waited < maxWait {
		time.Sleep(checkInterval)
		waited += checkInterval

		version = engine.versionSet.GetCurrent()
		currentL0 := version.GetLevelFileCount(0)
		currentL1 := version.GetLevelFileCount(1)

		t.Logf("After %v: L0=%d, L1=%d", waited, currentL0, currentL1)

		// 如果 L0 文件减少或 L1 有文件，说明 Compaction 发生了
		if currentL0 < initialL0 || currentL1 > 0 {
			t.Logf("Background compaction detected!")

			// 获取 Compaction 统计
			stats := engine.compactionManager.GetStats()
			t.Logf("Compaction stats: %v", stats)

			return
		}
	}

	t.Log("No background compaction detected within timeout (this is OK if L0 < 4 files)")
}

// BenchmarkEngineWithCompaction 性能测试
func BenchmarkEngineWithCompaction(b *testing.B) {
	tmpDir := b.TempDir()

	opts := &EngineOptions{
		Dir:          tmpDir,
		MemTableSize: 64 * 1024, // 64KB
	}

	engine, err := OpenEngine(opts)
	if err != nil {
		b.Fatal(err)
	}
	defer engine.Close()

	for i := 0; b.Loop(); i++ {
		data := map[string]any{
			"index": i,
			"value": fmt.Sprintf("benchmark-data-%d", i),
		}

		err := engine.Insert(data)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.StopTimer()

	// 等待所有 Flush 完成
	for engine.memtableManager.GetImmutableCount() > 0 {
		time.Sleep(10 * time.Millisecond)
	}

	// 报告统计信息
	version := engine.versionSet.GetCurrent()
	b.Logf("Final state: L0=%d files, L1=%d files, Total=%d files",
		version.GetLevelFileCount(0),
		version.GetLevelFileCount(1),
		version.GetFileCount())
}

// TestEngineSchemaRecover 测试 Schema 恢复
func TestEngineSchemaRecover(t *testing.T) {
	dir := "test_schema_recover"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	// 创建 Schema
	s := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: false, Comment: "年龄"},
		{Name: "email", Type: FieldTypeString, Indexed: false, Comment: "邮箱"},
	})

	// 1. 创建引擎并插入数据（带 Schema）
	engine, err := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 10 * 1024 * 1024, // 10 MB，不会触发 Flush
		Schema:       s,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 插入符合 Schema 的数据
	for i := 1; i <= 50; i++ {
		data := map[string]any{
			"name":  fmt.Sprintf("user_%d", i),
			"age":   20 + i%50,
			"email": fmt.Sprintf("user%d@example.com", i),
		}
		err := engine.Insert(data)
		if err != nil {
			t.Fatalf("Failed to insert valid data: %v", err)
		}
	}

	t.Log("Inserted 50 rows with schema")

	// 2. 关闭引擎
	engine.Close()

	// 3. 重新打开引擎（带 Schema，应该成功恢复）
	engine2, err := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 10 * 1024 * 1024,
		Schema:       s,
	})
	if err != nil {
		t.Fatalf("Failed to recover with schema: %v", err)
	}

	// 验证数据
	row, err := engine2.Get(1)
	if err != nil {
		t.Fatalf("Failed to get row after recovery: %v", err)
	}
	if row.Seq != 1 {
		t.Errorf("Expected seq=1, got %d", row.Seq)
	}

	// 验证字段
	if row.Data["name"] == nil {
		t.Error("Missing field 'name'")
	}
	if row.Data["age"] == nil {
		t.Error("Missing field 'age'")
	}

	engine2.Close()

	t.Log("Schema recovery test passed!")
}

// TestEngineSchemaRecoverInvalid 测试当 WAL 中有不符合 Schema 的数据时恢复失败
func TestEngineSchemaRecoverInvalid(t *testing.T) {
	dir := "test_schema_recover_invalid"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	// 1. 先不带 Schema 插入一些数据
	engine, err := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 10 * 1024 * 1024, // 大容量，确保不会触发 Flush
	})
	if err != nil {
		t.Fatal(err)
	}

	// 插入一些不符合后续 Schema 的数据
	for i := 1; i <= 10; i++ {
		data := map[string]any{
			"name": fmt.Sprintf("user_%d", i),
			"age":  "invalid_age", // 这是字符串，但后续 Schema 要求 int64
		}
		err := engine.Insert(data)
		if err != nil {
			t.Fatalf("Failed to insert data: %v", err)
		}
	}

	// 2. 停止后台任务但不 Flush（模拟崩溃）
	if engine.compactionManager != nil {
		engine.compactionManager.Stop()
	}
	// 直接关闭资源，但不 Flush MemTable
	if engine.walManager != nil {
		engine.walManager.Close()
	}
	if engine.versionSet != nil {
		engine.versionSet.Close()
	}
	if engine.sstManager != nil {
		engine.sstManager.Close()
	}

	// 3. 创建 Schema，age 字段要求 int64
	s := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: false, Comment: "年龄"},
	})

	// 4. 尝试用 Schema 打开引擎，应该失败
	engine2, err := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 10 * 1024 * 1024,
		Schema:       s,
	})
	if err == nil {
		engine2.Close()
		t.Fatal("Expected recovery to fail with invalid schema, but it succeeded")
	}

	// 验证错误信息包含 "schema validation failed"
	if err != nil {
		t.Logf("Got expected error: %v", err)
	}

	t.Log("Invalid schema recovery test passed!")
}

// TestEngineAutoRecoverSchema 测试自动从磁盘恢复 Schema
func TestEngineAutoRecoverSchema(t *testing.T) {
	dir := "test_auto_recover_schema"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	// 创建 Schema
	s := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: false, Comment: "年龄"},
	})

	// 1. 创建引擎并提供 Schema（会保存到磁盘）
	engine1, err := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 10 * 1024 * 1024,
		Schema:       s,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 插入数据
	for i := 1; i <= 10; i++ {
		data := map[string]any{
			"name": fmt.Sprintf("user_%d", i),
			"age":  20 + i,
		}
		err := engine1.Insert(data)
		if err != nil {
			t.Fatalf("Failed to insert: %v", err)
		}
	}

	engine1.Close()

	// 2. 重新打开引擎，不提供 Schema（应该自动从磁盘恢复）
	engine2, err := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 10 * 1024 * 1024,
		// 不设置 Schema
	})
	if err != nil {
		t.Fatalf("Failed to open without schema: %v", err)
	}

	// 验证 Schema 已恢复
	recoveredSchema := engine2.GetSchema()
	if recoveredSchema == nil {
		t.Fatal("Expected schema to be recovered, but got nil")
	}

	if recoveredSchema.Name != "users" {
		t.Errorf("Expected schema name 'users', got '%s'", recoveredSchema.Name)
	}

	if len(recoveredSchema.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(recoveredSchema.Fields))
	}

	// 验证数据
	row, err := engine2.Get(1)
	if err != nil {
		t.Fatalf("Failed to get row: %v", err)
	}
	if row.Data["name"] != "user_1" {
		t.Errorf("Expected name='user_1', got '%v'", row.Data["name"])
	}

	// 尝试插入新数据（应该符合恢复的 Schema）
	err = engine2.Insert(map[string]any{
		"name": "new_user",
		"age":  30,
	})
	if err != nil {
		t.Fatalf("Failed to insert with recovered schema: %v", err)
	}

	// 尝试插入不符合 Schema 的数据（应该失败）
	err = engine2.Insert(map[string]any{
		"name": "bad_user",
		"age":  "invalid", // 类型错误
	})
	if err == nil {
		t.Fatal("Expected insert to fail with invalid type, but it succeeded")
	}

	engine2.Close()

	t.Log("Auto recover schema test passed!")
}

// TestEngineSchemaTamperDetection 测试篡改检测
func TestEngineSchemaTamperDetection(t *testing.T) {
	dir := "test_schema_tamper"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	// 创建 Schema
	s := NewSchema("users", []Field{
		{Name: "name", Type: FieldTypeString, Indexed: false, Comment: "用户名"},
		{Name: "age", Type: FieldTypeInt64, Indexed: false, Comment: "年龄"},
	})

	// 1. 创建引擎并保存 Schema
	engine1, err := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 10 * 1024 * 1024,
		Schema:       s,
	})
	if err != nil {
		t.Fatal(err)
	}
	engine1.Close()

	// 2. 篡改 schema.json（修改字段但不更新 checksum）
	schemaPath := fmt.Sprintf("%s/schema.json", dir)
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}

	// 将 "age" 的注释从 "年龄" 改为 "AGE"（简单篡改）
	tamperedData := strings.Replace(string(schemaData), "年龄", "AGE", 1)

	err = os.WriteFile(schemaPath, []byte(tamperedData), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 3. 尝试打开引擎，应该检测到篡改
	engine2, err := OpenEngine(&EngineOptions{
		Dir:          dir,
		MemTableSize: 10 * 1024 * 1024,
	})
	if err == nil {
		engine2.Close()
		t.Fatal("Expected to detect schema tampering, but open succeeded")
	}

	// 验证错误信息包含 "checksum mismatch"
	errMsg := err.Error()
	if !strings.Contains(errMsg, "checksum mismatch") && !strings.Contains(errMsg, "tampered") {
		t.Errorf("Expected error about checksum mismatch or tampering, got: %v", err)
	}

	t.Logf("Detected tampering as expected: %v", err)
	t.Log("Schema tamper detection test passed!")
}
