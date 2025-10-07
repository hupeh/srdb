package btree

import (
	"encoding/binary"
)

const (
	NodeSize         = 4096 // 节点大小 (4 KB)
	Order            = 200  // B+Tree 阶数 (保守估计，叶子节点每个entry 20 bytes)
	HeaderSize       = 32   // 节点头大小
	NodeTypeInternal = 0    // 内部节点
	NodeTypeLeaf     = 1    // 叶子节点
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
		NodeType: NodeTypeInternal,
		Level:    level,
		Keys:     make([]int64, 0, Order),
		Children: make([]int64, 0, Order+1),
	}
}

// NewLeafNode 创建叶子节点
func NewLeafNode() *BTreeNode {
	return &BTreeNode{
		NodeType:    NodeTypeLeaf,
		Level:       0,
		Keys:        make([]int64, 0, Order),
		DataOffsets: make([]int64, 0, Order),
		DataSizes:   make([]int32, 0, Order),
	}
}

// Marshal 序列化节点到 4 KB
func (n *BTreeNode) Marshal() []byte {
	buf := make([]byte, NodeSize)

	// 写入 Header (32 bytes)
	buf[0] = n.NodeType
	binary.LittleEndian.PutUint16(buf[1:3], n.KeyCount)
	buf[3] = n.Level
	copy(buf[4:32], n.Reserved[:])

	// 写入 Keys
	offset := HeaderSize
	for _, key := range n.Keys {
		if offset+8 > NodeSize {
			break
		}
		binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(key))
		offset += 8
	}

	// 写入 Values
	if n.NodeType == NodeTypeInternal {
		// Internal Node: 写入子节点指针
		for _, child := range n.Children {
			if offset+8 > NodeSize {
				break
			}
			binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(child))
			offset += 8
		}
	} else {
		// Leaf Node: 写入数据位置
		for i := 0; i < len(n.Keys); i++ {
			if offset+12 > NodeSize {
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

// Unmarshal 从字节数组反序列化节点
func Unmarshal(data []byte) *BTreeNode {
	if len(data) < NodeSize {
		return nil
	}

	node := &BTreeNode{}

	// 读取 Header
	node.NodeType = data[0]
	node.KeyCount = binary.LittleEndian.Uint16(data[1:3])
	node.Level = data[3]
	copy(node.Reserved[:], data[4:32])

	// 读取 Keys
	offset := HeaderSize
	node.Keys = make([]int64, node.KeyCount)
	for i := 0; i < int(node.KeyCount); i++ {
		if offset+8 > len(data) {
			break
		}
		node.Keys[i] = int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
		offset += 8
	}

	// 读取 Values
	if node.NodeType == NodeTypeInternal {
		// Internal Node: 读取子节点指针
		childCount := int(node.KeyCount) + 1
		node.Children = make([]int64, childCount)
		for i := 0; i < childCount; i++ {
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
	return len(n.Keys) >= Order
}

// AddKey 添加 key (仅用于构建)
func (n *BTreeNode) AddKey(key int64) {
	n.Keys = append(n.Keys, key)
	n.KeyCount = uint16(len(n.Keys))
}

// AddChild 添加子节点 (仅用于内部节点)
func (n *BTreeNode) AddChild(offset int64) {
	if n.NodeType != NodeTypeInternal {
		panic("AddChild called on leaf node")
	}
	n.Children = append(n.Children, offset)
}

// AddData 添加数据位置 (仅用于叶子节点)
func (n *BTreeNode) AddData(key int64, offset int64, size int32) {
	if n.NodeType != NodeTypeLeaf {
		panic("AddData called on internal node")
	}
	n.Keys = append(n.Keys, key)
	n.DataOffsets = append(n.DataOffsets, offset)
	n.DataSizes = append(n.DataSizes, size)
	n.KeyCount = uint16(len(n.Keys))
}
