package participle

import (
	"encoding/json"
	"fmt"
	"strings"

	bd "github.com/dgraph-io/badger/v4"

	"github.com/go-ego/gse"
	"github.com/miajio/nla/pkg/badger"
)

// Engine 分词引擎
type Engine struct {
	dbEngine  *badger.Engine // 数据库
	segmenter gse.Segmenter  // 分词器
	root      *TrieNode      // 前缀树根节点
}

// New 创建分词引擎
func New(dbEngine *badger.Engine) (*Engine, error) {
	// 初始化前缀树根节点
	root := NewTrieNode()

	// 从数据库加载已有词典到前缀树
	if err := loadDictionaryFromDB(dbEngine.DB(), root); err != nil {
		return nil, fmt.Errorf("read db load dict fail: %v", err)
	}

	// 初始化GSE分词器
	seg, err := gse.New()
	if err != nil {
		return nil, fmt.Errorf("无法初始化GSE分词器: %v", err)
	}

	// 从前缀树加载词典到GSE
	loadDictionaryFromTrie(root, &seg)

	return &Engine{
		segmenter: seg,
		dbEngine:  dbEngine,
		root:      root,
	}, nil
}

// 从数据库加载词典到前缀树
func loadDictionaryFromDB(db *bd.DB, root *TrieNode) error {
	err := db.View(func(txn *bd.Txn) error {
		opts := bd.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			content := string(key)

			err := item.Value(func(val []byte) error {
				var entry DictEntry
				if err := json.Unmarshal(val, &entry); err != nil {
					return err
				}

				// 将词条添加到前缀树
				node := root
				chars := SplitString(content)

				for _, char := range chars {
					if _, ok := node.Children[char]; !ok {
						node.Children[char] = NewTrieNode()
					}
					node = node.Children[char]
				}

				node.IsEnd = true
				node.Entry = &entry
				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

// 从前缀树加载词典到GSE
func loadDictionaryFromTrie(root *TrieNode, seg *gse.Segmenter) {
	contents := make([]string, 0)

	// 遍历前缀树，收集所有词条
	var collectContents func(node *TrieNode, prefix string)
	collectContents = func(node *TrieNode, prefix string) {
		if node.IsEnd && node.Entry != nil {
			contents = append(contents, fmt.Sprintf("%s %f %s", node.Entry.Content, node.Entry.Frequency, node.Entry.Pos))
		}

		for char, child := range node.Children {
			collectContents(child, prefix+char)
		}
	}

	collectContents(root, "")

	// 如果有词条，加载到GSE分词器
	if len(contents) > 0 {
		dictData := strings.Join(contents, "\n")
		seg.LoadDictStr(dictData)
	}
}

// 将词条插入前缀树并保存到数据库
func (d *Engine) insertIntoTrieAndDB(content string, entry DictEntry) error {
	// 添加到前缀树
	node := d.root
	chars := SplitString(content)

	for _, char := range chars {
		if _, ok := node.Children[char]; !ok {
			node.Children[char] = NewTrieNode()
		}
		node = node.Children[char]
	}

	node.IsEnd = true
	node.Entry = &entry

	// 保存到数据库
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return d.dbEngine.Set([]byte(content), data)
}

// AddWord 添加一个新词到词典
func (d *Engine) AddWord(content string, frequency float64, pos string) error {
	entry := DictEntry{
		Content:   content,
		Frequency: frequency,
		Pos:       pos,
	}

	// 添加到前缀树并保存到数据库
	if err := d.insertIntoTrieAndDB(content, entry); err != nil {
		return fmt.Errorf("save content to db fail: %v", err)
	}

	// 更新GSE分词器
	d.segmenter.AddToken(content, frequency, pos)

	return nil
}

// LearnFromText 从文本中学习新词汇
func (d *Engine) LearnFromText(text string) error {
	// 分词
	contents := d.segmenter.Cut(text, true)

	// 分析新词
	for _, content := range contents {
		// 跳过特殊符号和单字词
		if len(content) <= 1 || IsSpecialChar(content) {
			continue
		}

		// 检查是否已存在于前缀树中
		if !d.containsWord(content) {
			// 默认频率为1000.0，词性为"nz"（其他专名）
			if err := d.AddWord(content, 1000.0, "nz"); err != nil {
				return fmt.Errorf("添加新词失败: %v", err)
			}
			fmt.Printf("学习到新词: %s\n", content)
		}
	}

	return nil
}

// containsWord 检查前缀树中是否包含指定的词
func (d *Engine) containsWord(content string) bool {
	node := d.root
	chars := SplitString(content)

	for _, char := range chars {
		if _, ok := node.Children[char]; !ok {
			return false
		}
		node = node.Children[char]
	}
	return node.IsEnd
}

// Segment 对文本进行分词
func (d *Engine) Segment(text string) []string {
	return d.segmenter.Cut(text, true)
}

// Close 关闭词典
func (d *Engine) Close() error {
	return d.dbEngine.Close()
}
