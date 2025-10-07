package btree

import (
	"os"
)

// Builder 从下往上构建 B+Tree
type Builder struct {
	order     int          // B+Tree 阶数
	file      *os.File     // 输出文件
	offset    int64        // 当前写入位置
	leafNodes []*BTreeNode // 叶子节点列表
}

// NewBuilder 创建构建器
func NewBuilder(file *os.File, startOffset int64) *Builder {
	return &Builder{
		order:     Order,
		file:      file,
		offset:    startOffset,
		leafNodes: make([]*BTreeNode, 0),
	}
}

// Add 添加一个 key-value 对 (数据必须已排序)
func (b *Builder) Add(key int64, dataOffset int64, dataSize int32) error {
	// 获取或创建当前叶子节点
	var leaf *BTreeNode
	if len(b.leafNodes) == 0 || b.leafNodes[len(b.leafNodes)-1].IsFull() {
		// 创建新的叶子节点
		leaf = NewLeafNode()
		b.leafNodes = append(b.leafNodes, leaf)
	} else {
		leaf = b.leafNodes[len(b.leafNodes)-1]
	}

	// 添加到叶子节点
	leaf.AddData(key, dataOffset, dataSize)

	return nil
}

// Build 构建完整的 B+Tree，返回根节点的 offset
func (b *Builder) Build() (rootOffset int64, err error) {
	if len(b.leafNodes) == 0 {
		return 0, nil
	}

	// 1. 写入所有叶子节点，记录它们的 offset
	leafOffsets := make([]int64, len(b.leafNodes))
	for i, leaf := range b.leafNodes {
		leafOffsets[i] = b.offset
		data := leaf.Marshal()
		_, err := b.file.WriteAt(data, b.offset)
		if err != nil {
			return 0, err
		}
		b.offset += NodeSize
	}

	// 2. 如果只有一个叶子节点，它就是根
	if len(b.leafNodes) == 1 {
		return leafOffsets[0], nil
	}

	// 3. 从下往上构建内部节点
	currentLevel := b.leafNodes
	currentOffsets := leafOffsets
	level := 1

	for len(currentLevel) > 1 {
		nextLevel, nextOffsets, err := b.buildLevel(currentLevel, currentOffsets, level)
		if err != nil {
			return 0, err
		}
		currentLevel = nextLevel
		currentOffsets = nextOffsets
		level++
	}

	// 4. 返回根节点的 offset
	return currentOffsets[0], nil
}

// buildLevel 构建一层内部节点
func (b *Builder) buildLevel(children []*BTreeNode, childOffsets []int64, level int) ([]*BTreeNode, []int64, error) {
	var parents []*BTreeNode
	var parentOffsets []int64

	// 每 order 个子节点创建一个父节点
	for i := 0; i < len(children); i += b.order {
		end := min(i+b.order, len(children))

		// 创建父节点
		parent := NewInternalNode(byte(level))

		// 添加第一个子节点 (没有对应的 key)
		parent.AddChild(childOffsets[i])

		// 添加剩余的子节点和分隔 key
		for j := i + 1; j < end; j++ {
			// 分隔 key 是子节点的第一个 key
			separatorKey := children[j].Keys[0]
			parent.AddKey(separatorKey)
			parent.AddChild(childOffsets[j])
		}

		// 写入父节点
		parentOffset := b.offset
		data := parent.Marshal()
		_, err := b.file.WriteAt(data, b.offset)
		if err != nil {
			return nil, nil, err
		}
		b.offset += NodeSize

		parents = append(parents, parent)
		parentOffsets = append(parentOffsets, parentOffset)
	}

	return parents, parentOffsets, nil
}
