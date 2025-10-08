package srdb

import (
	"os"
	"testing"
)

func TestVersionSetBasic(t *testing.T) {
	dir := "./test_manifest"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	// 创建 VersionSet
	vs, err := NewVersionSet(dir)
	if err != nil {
		t.Fatalf("NewVersionSet failed: %v", err)
	}
	defer vs.Close()

	// 检查初始状态
	version := vs.GetCurrent()
	if version.GetFileCount() != 0 {
		t.Errorf("Expected 0 files, got %d", version.GetFileCount())
	}

	t.Log("VersionSet basic test passed!")
}

func TestVersionSetAddFile(t *testing.T) {
	dir := "./test_manifest_add"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	vs, err := NewVersionSet(dir)
	if err != nil {
		t.Fatalf("NewVersionSet failed: %v", err)
	}
	defer vs.Close()

	// 添加文件
	edit := NewVersionEdit()
	edit.AddFile(&FileMetadata{
		FileNumber: 1,
		FileSize:   1024,
		MinKey:     1,
		MaxKey:     100,
		RowCount:   100,
	})

	err = vs.LogAndApply(edit)
	if err != nil {
		t.Fatalf("LogAndApply failed: %v", err)
	}

	// 检查
	version := vs.GetCurrent()
	if version.GetFileCount() != 1 {
		t.Errorf("Expected 1 file, got %d", version.GetFileCount())
	}

	files := version.GetSSTFiles()
	if files[0].FileNumber != 1 {
		t.Errorf("Expected file number 1, got %d", files[0].FileNumber)
	}

	t.Log("VersionSet add file test passed!")
}

func TestVersionSetDeleteFile(t *testing.T) {
	dir := "./test_manifest_delete"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	vs, err := NewVersionSet(dir)
	if err != nil {
		t.Fatalf("NewVersionSet failed: %v", err)
	}
	defer vs.Close()

	// 添加两个文件
	edit1 := NewVersionEdit()
	edit1.AddFile(&FileMetadata{FileNumber: 1, FileSize: 1024, MinKey: 1, MaxKey: 100, RowCount: 100})
	edit1.AddFile(&FileMetadata{FileNumber: 2, FileSize: 2048, MinKey: 101, MaxKey: 200, RowCount: 100})
	vs.LogAndApply(edit1)

	// 删除一个文件
	edit2 := NewVersionEdit()
	edit2.DeleteFile(1)
	err = vs.LogAndApply(edit2)
	if err != nil {
		t.Fatalf("LogAndApply failed: %v", err)
	}

	// 检查
	version := vs.GetCurrent()
	if version.GetFileCount() != 1 {
		t.Errorf("Expected 1 file, got %d", version.GetFileCount())
	}

	files := version.GetSSTFiles()
	if files[0].FileNumber != 2 {
		t.Errorf("Expected file number 2, got %d", files[0].FileNumber)
	}

	t.Log("VersionSet delete file test passed!")
}

func TestVersionSetRecover(t *testing.T) {
	dir := "./test_manifest_recover"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	// 第一次：创建并添加文件
	vs1, err := NewVersionSet(dir)
	if err != nil {
		t.Fatalf("NewVersionSet failed: %v", err)
	}

	edit := NewVersionEdit()
	edit.AddFile(&FileMetadata{FileNumber: 1, FileSize: 1024, MinKey: 1, MaxKey: 100, RowCount: 100})
	edit.AddFile(&FileMetadata{FileNumber: 2, FileSize: 2048, MinKey: 101, MaxKey: 200, RowCount: 100})
	vs1.LogAndApply(edit)
	vs1.Close()

	// 第二次：重新打开并恢复
	vs2, err := NewVersionSet(dir)
	if err != nil {
		t.Fatalf("NewVersionSet recover failed: %v", err)
	}
	defer vs2.Close()

	// 检查恢复的数据
	version := vs2.GetCurrent()
	if version.GetFileCount() != 2 {
		t.Errorf("Expected 2 files after recover, got %d", version.GetFileCount())
	}

	files := version.GetSSTFiles()
	if files[0].FileNumber != 1 || files[1].FileNumber != 2 {
		t.Errorf("File numbers not correct after recover")
	}

	t.Log("VersionSet recover test passed!")
}

func TestVersionSetMultipleEdits(t *testing.T) {
	dir := "./test_manifest_multiple"
	os.RemoveAll(dir)
	defer os.RemoveAll(dir)

	vs, err := NewVersionSet(dir)
	if err != nil {
		t.Fatalf("NewVersionSet failed: %v", err)
	}
	defer vs.Close()

	// 多次变更
	for i := int64(1); i <= 10; i++ {
		edit := NewVersionEdit()
		edit.AddFile(&FileMetadata{
			FileNumber: i,
			FileSize:   1024 * i,
			MinKey:     (i-1)*100 + 1,
			MaxKey:     i * 100,
			RowCount:   100,
		})
		err = vs.LogAndApply(edit)
		if err != nil {
			t.Fatalf("LogAndApply failed: %v", err)
		}
	}

	// 检查
	version := vs.GetCurrent()
	if version.GetFileCount() != 10 {
		t.Errorf("Expected 10 files, got %d", version.GetFileCount())
	}

	t.Log("VersionSet multiple edits test passed!")
}

func TestVersionEditEncodeDecode(t *testing.T) {
	// 创建 VersionEdit
	edit1 := NewVersionEdit()
	edit1.AddFile(&FileMetadata{FileNumber: 1, FileSize: 1024, MinKey: 1, MaxKey: 100, RowCount: 100})
	edit1.DeleteFile(2)
	nextFile := int64(10)
	edit1.SetNextFileNumber(nextFile)
	lastSeq := int64(1000)
	edit1.SetLastSequence(lastSeq)

	// 编码
	data, err := edit1.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// 解码
	edit2 := NewVersionEdit()
	err = edit2.Decode(data)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// 检查
	if len(edit2.AddedFiles) != 1 {
		t.Errorf("Expected 1 added file, got %d", len(edit2.AddedFiles))
	}
	if len(edit2.DeletedFiles) != 1 {
		t.Errorf("Expected 1 deleted file, got %d", len(edit2.DeletedFiles))
	}
	if *edit2.NextFileNumber != 10 {
		t.Errorf("Expected NextFileNumber 10, got %d", *edit2.NextFileNumber)
	}
	if *edit2.LastSequence != 1000 {
		t.Errorf("Expected LastSequence 1000, got %d", *edit2.LastSequence)
	}

	t.Log("VersionEdit encode/decode test passed!")
}
