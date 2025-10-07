package manifest

import (
	"fmt"
	"sync"
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
	for i := 0; i < NumLevels; i++ {
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
	for level := 0; level < NumLevels; level++ {
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
		for level := 0; level < NumLevels; level++ {
			newFiles := make([]*FileMetadata, 0)
			deletedCount := 0
			for _, file := range v.Levels[level] {
				if !deleteSet[file.FileNumber] {
					newFiles = append(newFiles, file)
				} else {
					deletedCount++
				}
			}
			if deletedCount > 0 {
				fmt.Printf("[Version.Apply] L%d: deleted %d files\n", level, deletedCount)
			}
			v.Levels[level] = newFiles
		}
	}

	// 添加文件（按层级添加）
	if len(edit.AddedFiles) > 0 {
		for _, file := range edit.AddedFiles {
			if file.Level >= 0 && file.Level < NumLevels {
				fmt.Printf("[Version.Apply] Adding file #%d to L%d (keys %d-%d)\n",
					file.FileNumber, file.Level, file.MinKey, file.MaxKey)
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
	for level := 0; level < NumLevels; level++ {
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
	for level := 0; level < NumLevels; level++ {
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
