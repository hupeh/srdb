package manifest

import (
	"io"
	"sync"
)

// Writer MANIFEST 写入器
type Writer struct {
	file io.Writer
	mu   sync.Mutex
}

// NewWriter 创建 MANIFEST 写入器
func NewWriter(file io.Writer) *Writer {
	return &Writer{
		file: file,
	}
}

// WriteEdit 写入版本变更
func (w *Writer) WriteEdit(edit *VersionEdit) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 编码
	data, err := edit.Encode()
	if err != nil {
		return err
	}

	// 写入
	_, err = w.file.Write(data)
	return err
}
