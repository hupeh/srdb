package srdb

import (
	"encoding/binary"
	"os"
	"slices"
	"sort"

	"github.com/edsrzf/mmap-go"
)

const (
	BTreeNodeSize         = 4096 // 节点大小 (4 KB)
	BTreeOrder            = 200  // B+Tree 阶数 (保守估计，叶子节点每个entry 20 bytes)
	BTreeHeaderSize       = 32   // 节点头大小
	BTreeNodeTypeInternal = 0    // 内部节点
	BTreeNodeTypeLeaf     = 1    // 叶子节点
)

// BTreeNode 表示一个 B+Tree 节点 (4 KB)
type BTreeNode struct {
	// Header (32 bytes)
	NodeType byte     // 0=Internal, 1=Leaf
	KeyCount uint16   // key 数量
	Level    byte     // 层级 (0=叶子层)
	Reserved [28]byte // 预留字段

	// Keys (variable, 最多 256 个)
	Keys []int64 // key 数组

	// Values (variable)
	// Internal Node: 子节点指针
	Children []int64 // 子节点的文件 offset

	// Leaf Node: 数据位置
	DataOffsets []int64 // 数据块的文件 offset
	DataSizes   []int32 // 数据块大小
}

// NewInternalNode 创建内部节点
func NewInternalNode(level byte) *BTreeNode {
	return &BTreeNode{
		NodeType: BTreeNodeTypeInternal,
		Level:    level,
		Keys:     make([]int64, 0, BTreeOrder),
		Children: make([]int64, 0, BTreeOrder+1),
	}
}

// NewLeafNode 创建叶子节点
func NewLeafNode() *BTreeNode {
	return &BTreeNode{
		NodeType:    BTreeNodeTypeLeaf,
		Level:       0,
		Keys:        make([]int64, 0, BTreeOrder),
		DataOffsets: make([]int64, 0, BTreeOrder),
		DataSizes:   make([]int32, 0, BTreeOrder),
	}
}

// Marshal 序列化节点到 4 KB
func (n *BTreeNode) Marshal() []byte {
	buf := make([]byte, BTreeNodeSize)

	// 写入 Header (32 bytes)
	buf[0] = n.NodeType
	binary.LittleEndian.PutUint16(buf[1:3], n.KeyCount)
	buf[3] = n.Level
	copy(buf[4:32], n.Reserved[:])

	// 写入 Keys
	offset := BTreeHeaderSize
	for _, key := range n.Keys {
		if offset+8 > BTreeNodeSize {
			break
		}
		binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(key))
		offset += 8
	}

	// 写入 Values
	if n.NodeType == BTreeNodeTypeInternal {
		// Internal Node: 写入子节点指针
		for _, child := range n.Children {
			if offset+8 > BTreeNodeSize {
				break
			}
			binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(child))
			offset += 8
		}
	} else {
		// Leaf Node: 写入数据位置
		for i := 0; i < len(n.Keys); i++ {
			if offset+12 > BTreeNodeSize {
				break
			}
			binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(n.DataOffsets[i]))
			offset += 8
			binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(n.DataSizes[i]))
			offset += 4
		}
	}

	return buf
}

// UnmarshalBTree 从字节数组反序列化节点
func UnmarshalBTree(data []byte) *BTreeNode {
	if len(data) < BTreeNodeSize {
		return nil
	}

	node := &BTreeNode{}

	// 读取 Header
	node.NodeType = data[0]
	node.KeyCount = binary.LittleEndian.Uint16(data[1:3])
	node.Level = data[3]
	copy(node.Reserved[:], data[4:32])

	// 读取 Keys
	offset := BTreeHeaderSize
	node.Keys = make([]int64, node.KeyCount)
	for i := 0; i < int(node.KeyCount); i++ {
		if offset+8 > len(data) {
			break
		}
		node.Keys[i] = int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
		offset += 8
	}

	// 读取 Values
	if node.NodeType == BTreeNodeTypeInternal {
		// Internal Node: 读取子节点指针
		childCount := int(node.KeyCount) + 1
		node.Children = make([]int64, childCount)
		for i := range childCount {
			if offset+8 > len(data) {
				break
			}
			node.Children[i] = int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
			offset += 8
		}
	} else {
		// Leaf Node: 读取数据位置
		node.DataOffsets = make([]int64, node.KeyCount)
		node.DataSizes = make([]int32, node.KeyCount)
		for i := 0; i < int(node.KeyCount); i++ {
			if offset+12 > len(data) {
				break
			}
			node.DataOffsets[i] = int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
			offset += 8
			node.DataSizes[i] = int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
			offset += 4
		}
	}

	return node
}

// IsFull 检查节点是否已满
func (n *BTreeNode) IsFull() bool {
	return len(n.Keys) >= BTreeOrder
}

// AddKey 添加 key (仅用于构建)
func (n *BTreeNode) AddKey(key int64) {
	n.Keys = append(n.Keys, key)
	n.KeyCount = uint16(len(n.Keys))
}

// AddChild 添加子节点 (仅用于内部节点)
func (n *BTreeNode) AddChild(offset int64) {
	if n.NodeType != BTreeNodeTypeInternal {
		panic("AddChild called on leaf node")
	}
	n.Children = append(n.Children, offset)
}

// AddData 添加数据位置 (仅用于叶子节点)
func (n *BTreeNode) AddData(key int64, offset int64, size int32) {
	if n.NodeType != BTreeNodeTypeLeaf {
		panic("AddData called on internal node")
	}
	n.Keys = append(n.Keys, key)
	n.DataOffsets = append(n.DataOffsets, offset)
	n.DataSizes = append(n.DataSizes, size)
	n.KeyCount = uint16(len(n.Keys))
}

// BTreeBuilder 从下往上构建 B+Tree
type BTreeBuilder struct {
	order     int          // B+Tree 阶数
	file      *os.File     // 输出文件
	offset    int64        // 当前写入位置
	leafNodes []*BTreeNode // 叶子节点列表
}

// NewBTreeBuilder 创建构建器
func NewBTreeBuilder(file *os.File, startOffset int64) *BTreeBuilder {
	return &BTreeBuilder{
		order:     BTreeOrder,
		file:      file,
		offset:    startOffset,
		leafNodes: make([]*BTreeNode, 0),
	}
}

// Add 添加一个 key-value 对 (数据必须已排序)
func (b *BTreeBuilder) Add(key int64, dataOffset int64, dataSize int32) error {
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
func (b *BTreeBuilder) Build() (rootOffset int64, err error) {
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
		b.offset += BTreeNodeSize
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
func (b *BTreeBuilder) buildLevel(children []*BTreeNode, childOffsets []int64, level int) ([]*BTreeNode, []int64, error) {
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
		b.offset += BTreeNodeSize

		parents = append(parents, parent)
		parentOffsets = append(parentOffsets, parentOffset)
	}

	return parents, parentOffsets, nil
}

// BTreeReader 用于查询 B+Tree (mmap)
type BTreeReader struct {
	mmap       mmap.MMap
	rootOffset int64
}

// NewBTreeReader 创建查询器
func NewBTreeReader(mmap mmap.MMap, rootOffset int64) *BTreeReader {
	return &BTreeReader{
		mmap:       mmap,
		rootOffset: rootOffset,
	}
}

// Get 查询 key，返回数据位置
func (r *BTreeReader) Get(key int64) (dataOffset int64, dataSize int32, found bool) {
	if r.rootOffset == 0 {
		return 0, 0, false
	}

	nodeOffset := r.rootOffset

	for {
		// 读取节点 (零拷贝)
		if nodeOffset+BTreeNodeSize > int64(len(r.mmap)) {
			return 0, 0, false
		}

		nodeData := r.mmap[nodeOffset : nodeOffset+BTreeNodeSize]
		node := UnmarshalBTree(nodeData)

		if node == nil {
			return 0, 0, false
		}

		// 叶子节点
		if node.NodeType == BTreeNodeTypeLeaf {
			// 二分查找
			idx := sort.Search(len(node.Keys), func(i int) bool {
				return node.Keys[i] >= key
			})
			if idx < len(node.Keys) && node.Keys[idx] == key {
				return node.DataOffsets[idx], node.DataSizes[idx], true
			}
			return 0, 0, false
		}

		// 内部节点，继续向下
		// keys[i] 是分隔符，children[i] 包含 < keys[i] 的数据
		// children[i+1] 包含 >= keys[i] 的数据
		idx := sort.Search(len(node.Keys), func(i int) bool {
			return node.Keys[i] > key
		})
		// idx 现在指向第一个 > key 的位置
		// 我们应该走 children[idx]
		if idx >= len(node.Children) {
			idx = len(node.Children) - 1
		}
		nodeOffset = node.Children[idx]
	}
}

// GetAllKeys 获取 B+Tree 中所有的 key（按顺序）
func (r *BTreeReader) GetAllKeys() []int64 {
	if r.rootOffset == 0 {
		return nil
	}

	var keys []int64
	r.traverseLeafNodes(r.rootOffset, func(node *BTreeNode) {
		keys = append(keys, node.Keys...)
	})

	// 显式排序以确保返回的 keys 严格有序
	// 虽然 B+Tree 构建时应该已经是有序的，但这是一个安全保障
	// 特别是在 compaction 后，确保查询结果正确排序
	slices.Sort(keys)

	return keys
}

// traverseLeafNodes 遍历所有叶子节点
func (r *BTreeReader) traverseLeafNodes(nodeOffset int64, callback func(*BTreeNode)) {
	if nodeOffset+BTreeNodeSize > int64(len(r.mmap)) {
		return
	}

	nodeData := r.mmap[nodeOffset : nodeOffset+BTreeNodeSize]
	node := UnmarshalBTree(nodeData)

	if node == nil {
		return
	}

	if node.NodeType == BTreeNodeTypeLeaf {
		// 叶子节点，执行回调
		callback(node)
	} else {
		// 内部节点，递归遍历所有子节点
		for _, childOffset := range node.Children {
			r.traverseLeafNodes(childOffset, callback)
		}
	}
}
