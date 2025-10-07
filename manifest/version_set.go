package manifest

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// VersionSet 版本集合管理器
type VersionSet struct {
	// 当前版本
	current *Version

	// MANIFEST 文件
	manifestFile   *os.File
	manifestWriter *Writer
	manifestNumber int64

	// 下一个文件编号
	nextFileNumber atomic.Int64

	// 最后序列号
	lastSequence atomic.Int64

	// 目录
	dir string

	// 锁
	mu sync.RWMutex
}

// NewVersionSet 创建版本集合
func NewVersionSet(dir string) (*VersionSet, error) {
	vs := &VersionSet{
		dir: dir,
	}

	// 确保目录存在
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	// 读取 CURRENT 文件
	currentFile := filepath.Join(dir, "CURRENT")
	data, err := os.ReadFile(currentFile)

	if err != nil {
		// CURRENT 不存在，创建新的 MANIFEST
		return vs, vs.createNewManifest()
	}

	// 读取 MANIFEST 文件
	manifestName := strings.TrimSpace(string(data))
	manifestPath := filepath.Join(dir, manifestName)

	// 恢复版本信息
	version, err := vs.recoverFromManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	vs.current = version
	vs.nextFileNumber.Store(version.NextFileNumber)
	vs.lastSequence.Store(version.LastSequence)

	// 解析 MANIFEST 编号
	fmt.Sscanf(manifestName, "MANIFEST-%d", &vs.manifestNumber)

	// 打开 MANIFEST 用于追加
	file, err := os.OpenFile(manifestPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	vs.manifestFile = file
	vs.manifestWriter = NewWriter(file)

	return vs, nil
}

// createNewManifest 创建新的 MANIFEST
func (vs *VersionSet) createNewManifest() error {
	// 生成新的 MANIFEST 文件名
	vs.manifestNumber = vs.nextFileNumber.Add(1)
	manifestName := fmt.Sprintf("MANIFEST-%06d", vs.manifestNumber)
	manifestPath := filepath.Join(vs.dir, manifestName)

	// 创建 MANIFEST 文件
	file, err := os.Create(manifestPath)
	if err != nil {
		return err
	}

	vs.manifestFile = file
	vs.manifestWriter = NewWriter(file)

	// 创建初始版本
	vs.current = NewVersion()

	// 写入初始版本
	edit := NewVersionEdit()
	nextFile := vs.manifestNumber
	edit.SetNextFileNumber(nextFile)
	lastSeq := int64(0)
	edit.SetLastSequence(lastSeq)

	err = vs.manifestWriter.WriteEdit(edit)
	if err != nil {
		return err
	}

	// 同步到磁盘
	err = vs.manifestFile.Sync()
	if err != nil {
		return err
	}

	// 更新 CURRENT 文件
	return vs.updateCurrent(manifestName)
}

// recoverFromManifest 从 MANIFEST 恢复版本
func (vs *VersionSet) recoverFromManifest(manifestPath string) (*Version, error) {
	// 打开 MANIFEST 文件
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := NewReader(file)

	// 创建初始版本
	version := NewVersion()

	// 读取所有 VersionEdit
	for {
		edit, err := reader.ReadEdit()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// 应用变更
		version.Apply(edit)
	}

	return version, nil
}

// updateCurrent 更新 CURRENT 文件
func (vs *VersionSet) updateCurrent(manifestName string) error {
	currentPath := filepath.Join(vs.dir, "CURRENT")
	tmpPath := currentPath + ".tmp"

	// 1. 写入临时文件
	err := os.WriteFile(tmpPath, []byte(manifestName+"\n"), 0644)
	if err != nil {
		return err
	}

	// 2. 原子性重命名
	err = os.Rename(tmpPath, currentPath)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

// LogAndApply 记录并应用版本变更
func (vs *VersionSet) LogAndApply(edit *VersionEdit) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	// 1. 创建新版本
	newVersion := vs.current.Clone()

	// 2. 应用变更
	newVersion.Apply(edit)

	// 3. 写入 MANIFEST
	err := vs.manifestWriter.WriteEdit(edit)
	if err != nil {
		return err
	}

	// 4. 同步到磁盘
	err = vs.manifestFile.Sync()
	if err != nil {
		return err
	}

	// 5. 更新当前版本
	vs.current = newVersion

	// 6. 更新原子变量
	if edit.NextFileNumber != nil {
		vs.nextFileNumber.Store(*edit.NextFileNumber)
	}
	if edit.LastSequence != nil {
		vs.lastSequence.Store(*edit.LastSequence)
	}

	return nil
}

// GetCurrent 获取当前版本
func (vs *VersionSet) GetCurrent() *Version {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.current
}

// GetNextFileNumber 获取下一个文件编号
func (vs *VersionSet) GetNextFileNumber() int64 {
	return vs.nextFileNumber.Load()
}

// AllocateFileNumber 分配文件编号
func (vs *VersionSet) AllocateFileNumber() int64 {
	return vs.nextFileNumber.Add(1)
}

// GetLastSequence 获取最后序列号
func (vs *VersionSet) GetLastSequence() int64 {
	return vs.lastSequence.Load()
}

// SetLastSequence 设置最后序列号
func (vs *VersionSet) SetLastSequence(seq int64) {
	vs.lastSequence.Store(seq)
}

// Close 关闭 VersionSet
func (vs *VersionSet) Close() error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if vs.manifestFile != nil {
		return vs.manifestFile.Close()
	}
	return nil
}
