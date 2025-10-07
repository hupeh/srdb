package btree

import (
	"sort"

	"github.com/edsrzf/mmap-go"
)

// Reader 用于查询 B+Tree (mmap)
type Reader struct {
	mmap       mmap.MMap
	rootOffset int64
}

// NewReader 创建查询器
func NewReader(mmap mmap.MMap, rootOffset int64) *Reader {
	return &Reader{
		mmap:       mmap,
		rootOffset: rootOffset,
	}
}

// Get 查询 key，返回数据位置
func (r *Reader) Get(key int64) (dataOffset int64, dataSize int32, found bool) {
	if r.rootOffset == 0 {
		return 0, 0, false
	}

	nodeOffset := r.rootOffset

	for {
		// 读取节点 (零拷贝)
		if nodeOffset+NodeSize > int64(len(r.mmap)) {
			return 0, 0, false
		}

		nodeData := r.mmap[nodeOffset : nodeOffset+NodeSize]
		node := Unmarshal(nodeData)

		if node == nil {
			return 0, 0, false
		}

		// 叶子节点
		if node.NodeType == NodeTypeLeaf {
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
func (r *Reader) GetAllKeys() []int64 {
	if r.rootOffset == 0 {
		return nil
	}

	var keys []int64
	r.traverseLeafNodes(r.rootOffset, func(node *BTreeNode) {
		keys = append(keys, node.Keys...)
	})
	return keys
}

// traverseLeafNodes 遍历所有叶子节点
func (r *Reader) traverseLeafNodes(nodeOffset int64, callback func(*BTreeNode)) {
	if nodeOffset+NodeSize > int64(len(r.mmap)) {
		return
	}

	nodeData := r.mmap[nodeOffset : nodeOffset+NodeSize]
	node := Unmarshal(nodeData)

	if node == nil {
		return
	}

	if node.NodeType == NodeTypeLeaf {
		// 叶子节点，执行回调
		callback(node)
	} else {
		// 内部节点，递归遍历所有子节点
		for _, childOffset := range node.Children {
			r.traverseLeafNodes(childOffset, callback)
		}
	}
}
