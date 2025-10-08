package srdb

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// FileMetadata SST 文件元数据
type FileMetadata struct {
	FileNumber int64 // 文件编号
	Level      int   // 所在层级 (0-6)
	FileSize   int64 // 文件大小
	MinKey     int64 // 最小 key
	MaxKey     int64 // 最大 key
	RowCount   int64 // 行数
}

const (
	NumLevels = 7 // L0-L6
)

// Version 数据库的一个版本快照
type Version struct {
	// 分层存储 SST 文件 (L0-L6)
	Levels [NumLevels][]*FileMetadata

	// 下一个文件编号
	NextFileNumber int64

	// 最后序列号
	LastSequence int64

	// 版本号
	VersionNumber int64

	mu sync.RWMutex
}

// NewVersion 创建新版本
func NewVersion() *Version {
	v := &Version{
		NextFileNumber: 1,
		LastSequence:   0,
		VersionNumber:  0,
	}
	// 初始化每一层
	for i := range NumLevels {
		v.Levels[i] = make([]*FileMetadata, 0)
	}
	return v
}

// Clone 克隆版本
func (v *Version) Clone() *Version {
	v.mu.RLock()
	defer v.mu.RUnlock()

	newVersion := &Version{
		NextFileNumber: v.NextFileNumber,
		LastSequence:   v.LastSequence,
		VersionNumber:  v.VersionNumber + 1,
	}

	// 克隆每一层
	for level := range NumLevels {
		newVersion.Levels[level] = make([]*FileMetadata, len(v.Levels[level]))
		copy(newVersion.Levels[level], v.Levels[level])
	}

	return newVersion
}

// Apply 应用版本变更
func (v *Version) Apply(edit *VersionEdit) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// 删除文件（按层级删除）
	if len(edit.DeletedFiles) > 0 {
		deleteSet := make(map[int64]bool)
		for _, fileNum := range edit.DeletedFiles {
			deleteSet[fileNum] = true
		}

		// 遍历每一层，删除文件
		for level := range NumLevels {
			newFiles := make([]*FileMetadata, 0)
			for _, file := range v.Levels[level] {
				if !deleteSet[file.FileNumber] {
					newFiles = append(newFiles, file)
				}
			}
			v.Levels[level] = newFiles
		}
	}

	// 添加文件（按层级添加）
	if len(edit.AddedFiles) > 0 {
		for _, file := range edit.AddedFiles {
			if file.Level >= 0 && file.Level < NumLevels {
				v.Levels[file.Level] = append(v.Levels[file.Level], file)
			}
		}
	}

	// 更新下一个文件编号
	if edit.NextFileNumber != nil {
		v.NextFileNumber = *edit.NextFileNumber
	}

	// 更新最后序列号
	if edit.LastSequence != nil {
		v.LastSequence = *edit.LastSequence
	}
}

// GetLevel 获取指定层级的文件
func (v *Version) GetLevel(level int) []*FileMetadata {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if level < 0 || level >= NumLevels {
		return nil
	}

	files := make([]*FileMetadata, len(v.Levels[level]))
	copy(files, v.Levels[level])
	return files
}

// GetSSTFiles 获取所有 SST 文件（副本，兼容旧接口）
func (v *Version) GetSSTFiles() []*FileMetadata {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// 收集所有层级的文件
	allFiles := make([]*FileMetadata, 0)
	for level := range NumLevels {
		allFiles = append(allFiles, v.Levels[level]...)
	}
	return allFiles
}

// GetNextFileNumber 获取下一个文件编号
func (v *Version) GetNextFileNumber() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.NextFileNumber
}

// GetLastSequence 获取最后序列号
func (v *Version) GetLastSequence() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.LastSequence
}

// GetFileCount 获取文件数量
func (v *Version) GetFileCount() int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	total := 0
	for level := range NumLevels {
		total += len(v.Levels[level])
	}
	return total
}

// GetLevelFileCount 获取指定层级的文件数量
func (v *Version) GetLevelFileCount(level int) int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if level < 0 || level >= NumLevels {
		return 0
	}
	return len(v.Levels[level])
}

// ManifestReader MANIFEST 读取器
type ManifestReader struct {
	file io.Reader
}

// NewManifestReader 创建 MANIFEST 读取器
func NewManifestReader(file io.Reader) *ManifestReader {
	return &ManifestReader{
		file: file,
	}
}

// ReadEdit 读取版本变更
func (r *ManifestReader) ReadEdit() (*VersionEdit, error) {
	// 读取 CRC32 和 Length
	header := make([]byte, 8)
	_, err := io.ReadFull(r.file, header)
	if err != nil {
		return nil, err
	}

	// 读取长度
	length := binary.LittleEndian.Uint32(header[4:8])

	// 读取数据
	data := make([]byte, 8+length)
	copy(data[0:8], header)
	_, err = io.ReadFull(r.file, data[8:])
	if err != nil {
		return nil, err
	}

	// 解码
	edit := NewVersionEdit()
	err = edit.Decode(data)
	if err != nil {
		return nil, err
	}

	return edit, nil
}

// ManifestWriter MANIFEST 写入器
type ManifestWriter struct {
	file io.Writer
	mu   sync.Mutex
}

// NewManifestWriter 创建 MANIFEST 写入器
func NewManifestWriter(file io.Writer) *ManifestWriter {
	return &ManifestWriter{
		file: file,
	}
}

// WriteEdit 写入版本变更
func (w *ManifestWriter) WriteEdit(edit *VersionEdit) error {
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

// EditType 变更类型
type EditType byte

const (
	EditTypeAddFile     EditType = 1 // 添加文件
	EditTypeDeleteFile  EditType = 2 // 删除文件
	EditTypeSetNextFile EditType = 3 // 设置下一个文件编号
	EditTypeSetLastSeq  EditType = 4 // 设置最后序列号
)

// VersionEdit 版本变更记录
type VersionEdit struct {
	// 添加的文件
	AddedFiles []*FileMetadata

	// 删除的文件（文件编号列表）
	DeletedFiles []int64

	// 下一个文件编号
	NextFileNumber *int64

	// 最后序列号
	LastSequence *int64
}

// NewVersionEdit 创建版本变更
func NewVersionEdit() *VersionEdit {
	return &VersionEdit{
		AddedFiles:   make([]*FileMetadata, 0),
		DeletedFiles: make([]int64, 0),
	}
}

// AddFile 添加文件
func (e *VersionEdit) AddFile(file *FileMetadata) {
	e.AddedFiles = append(e.AddedFiles, file)
}

// DeleteFile 删除文件
func (e *VersionEdit) DeleteFile(fileNumber int64) {
	e.DeletedFiles = append(e.DeletedFiles, fileNumber)
}

// SetNextFileNumber 设置下一个文件编号
func (e *VersionEdit) SetNextFileNumber(num int64) {
	e.NextFileNumber = &num
}

// SetLastSequence 设置最后序列号
func (e *VersionEdit) SetLastSequence(seq int64) {
	e.LastSequence = &seq
}

// Encode 编码为字节
func (e *VersionEdit) Encode() ([]byte, error) {
	// 使用 JSON 编码（简单实现）
	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}

	// 格式: CRC32(4) + Length(4) + Data
	totalLen := 8 + len(data)
	buf := make([]byte, totalLen)

	// 计算 CRC32
	crc := crc32.ChecksumIEEE(data)
	binary.LittleEndian.PutUint32(buf[0:4], crc)

	// 写入长度
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(data)))

	// 写入数据
	copy(buf[8:], data)

	return buf, nil
}

// Decode 从字节解码
func (e *VersionEdit) Decode(data []byte) error {
	if len(data) < 8 {
		return io.ErrUnexpectedEOF
	}

	// 读取 CRC32
	crc := binary.LittleEndian.Uint32(data[0:4])

	// 读取长度
	length := binary.LittleEndian.Uint32(data[4:8])

	if len(data) < int(8+length) {
		return io.ErrUnexpectedEOF
	}

	// 读取数据
	editData := data[8 : 8+length]

	// 验证 CRC32
	if crc32.ChecksumIEEE(editData) != crc {
		return io.ErrUnexpectedEOF
	}

	// JSON 解码
	return json.Unmarshal(editData, e)
}

// VersionSet 版本集合管理器
type VersionSet struct {
	// 当前版本
	current *Version

	// MANIFEST 文件
	manifestFile   *os.File
	manifestWriter *ManifestWriter
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
	vs.manifestWriter = NewManifestWriter(file)

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
	vs.manifestWriter = NewManifestWriter(file)

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

	reader := NewManifestReader(file)

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
		// 使用 CAS 循环确保只在新值更大时更新，避免并发回退
		for {
			old := vs.nextFileNumber.Load()
			if *edit.NextFileNumber <= old {
				break // 新值不大于当前值，不更新
			}
			if vs.nextFileNumber.CompareAndSwap(old, *edit.NextFileNumber) {
				break // 更新成功
			}
			// CAS 失败，重试
		}
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
