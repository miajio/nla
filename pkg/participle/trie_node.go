package participle

// TrieNode 前缀树节点
type TrieNode struct {
	Children map[string]*TrieNode // 子节点，使用完整字符作为键
	IsEnd    bool                 // 是否是一个词的结尾
	Entry    *DictEntry           // 如果是词尾，存储词条信息
}

// NewTrieNode 创建一个新的前缀树节点
func NewTrieNode() *TrieNode {
	return &TrieNode{
		Children: make(map[string]*TrieNode),
		IsEnd:    false,
		Entry:    nil,
	}
}
