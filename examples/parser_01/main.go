package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-ego/gse"
)

// DictEntry 表示词典中的一个词条
type DictEntry struct {
	Word      string  `json:"word"`      // 词
	Frequency float64 `json:"frequency"` // 词频
	Pos       string  `json:"pos"`       // 词性
}

// Dictionary 管理分词字典
type Dictionary struct {
	segmenter gse.Segmenter // GSE分词器
	db        *badger.DB    // Badger数据库实例
}

// NewDictionary 创建一个新的词典实例
func NewDictionary(dbPath string) (*Dictionary, error) {
	// 创建数据库目录
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dbPath, 0755); err != nil {
			return nil, fmt.Errorf("无法创建数据库目录: %v", err)
		}
	}

	// 打开Badger数据库
	opts := badger.DefaultOptions(dbPath)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("无法打开Badger数据库: %v", err)
	}

	// 初始化GSE分词器
	seg, err := gse.New()
	if err != nil {
		return nil, fmt.Errorf("无法初始化GSE分词器: %v", err)
	}
	// 从数据库加载已有词典
	if err := loadDictionaryFromDB(db, &seg); err != nil {
		return nil, fmt.Errorf("从数据库加载词典失败: %v", err)
	}

	return &Dictionary{
		segmenter: seg,
		db:        db,
	}, nil
}

// 从数据库加载词典到GSE分词器
func loadDictionaryFromDB(db *badger.DB, seg *gse.Segmenter) error {
	words := make([]string, 0)

	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			// _ := item.Key()
			err := item.Value(func(val []byte) error {
				var entry DictEntry
				if err := json.Unmarshal(val, &entry); err != nil {
					return err
				}

				// 构建GSE格式的词条
				wordWithFreq := fmt.Sprintf("%s %d %s", entry.Word, entry.Frequency, entry.Pos)
				words = append(words, wordWithFreq)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// 如果有词条，加载到GSE分词器
	if len(words) > 0 {
		dictData := strings.Join(words, "\n")
		seg.LoadDictStr(dictData)
	}

	return nil
}

// AddWord 添加一个新词到词典
func (d *Dictionary) AddWord(word string, frequency float64, pos string) error {
	entry := DictEntry{
		Word:      word,
		Frequency: frequency,
		Pos:       pos,
	}

	// 序列化词条
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("序列化词条失败: %v", err)
	}

	// 存储到数据库
	err = d.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(word), data)
	})
	if err != nil {
		return fmt.Errorf("保存词条到数据库失败: %v", err)
	}

	// 更新GSE分词器
	d.segmenter.AddToken(
		word,
		frequency,
		pos,
	)

	return nil
}

// LearnFromText 从文本中学习新词汇
func (d *Dictionary) LearnFromText(text string) error {
	// 分词
	words := d.segmenter.Cut(text, true)

	// 分析新词
	for _, word := range words {
		// 跳过单字词和标点符号
		if len(word) <= 1 || isPunctuation(word) {
			continue
		}

		// 检查是否已存在
		exists := false
		err := d.db.View(func(txn *badger.Txn) error {
			_, err := txn.Get([]byte(word))
			if err == nil {
				exists = true
			} else if err == badger.ErrKeyNotFound {
				exists = false
			} else {
				return err
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("检查词条是否存在失败: %v", err)
		}

		// 如果不存在，添加到词典
		if !exists {
			// 默认频率为1000，词性为"nz"（其他专名）
			if err := d.AddWord(word, 1000, "nz"); err != nil {
				return fmt.Errorf("添加新词失败: %v", err)
			}
			fmt.Printf("学习到新词: %s\n", word)
		}
	}

	return nil
}

// Segment 对文本进行分词
func (d *Dictionary) Segment(text string) []string {
	return d.segmenter.Cut(text, true)
}

// Close 关闭词典
func (d *Dictionary) Close() error {
	return d.db.Close()
}

// 检查是否为标点符号
func isPunctuation(s string) bool {
	// 检查是否为空字符串
	if s == "" {
		return false
	}

	// 使用正则表达式检查是否为标点符号或特殊符号
	// 匹配中文标点符号
	chinesePunctuation := regexp.MustCompile(`[\p{P}\p{S}\p{Z}]+`)

	// 匹配所有Unicode标点符号和符号类别
	// \p{P} 标点符号: Pc, Pd, Ps, Pe, Pi, Pf, Po
	// \p{S} 符号: Sm, Sc, Sk, So
	// \p{Z} 分隔符: Zs, Zl, Zp

	// 如果整个字符串都是特殊符号，则返回true
	return chinesePunctuation.MatchString(s)
}

func main() {
	// 创建词典实例
	// dir, err := os.UserHomeDir()
	// if err != nil {
	// 	log.Fatalf("无法获取用户主目录: %v", err)
	// }

	// dbPath := filepath.Join(dir, "gse_dict_db")
	dict, err := NewDictionary("gse_dict_db")
	if err != nil {
		log.Fatalf("创建词典失败: %v", err)
	}
	defer dict.Close()

	// 分词示例
	text := `
	“欢迎来到啵啵间，今天煮啵来给大家送浮力”、“今天这款产品不要19.9米，只要9.9米”、“现现，今天下单，3个太阳内飞走”......在过去很长一段时间，抖音等平台的直播间和短视频中，充斥着这类令人迷惑的黑话。
	这些黑话的形式多样，包括用另一词语替代，比如用“米”、“达不溜“来表达钱；中英混杂，比如“独one无two”；使用数字、符号替代，比如“8+1“代表酒、“+V”表示加微信;将词语的其中一个字换成“某“或者“什么”， 比如“某宝“是指淘宝等。
	据悉，6月12日，抖音发布关于治理网络“黑话烂梗”的公告，近期平台发现，有少数账号仍然试图通过“谐音梗”“缩写字”“拆解词”“图文结合”等形式，发布“黑话烂梗”，造成公众对信息的理解障碍。上述信息和行为，很多并非语言文字正常、合理的发展与更新，而是故意利用“黑话烂梗”等不规范表达言行，传播色情低俗、不良文化、脏话污语，或者煽动对立矛盾等违法违规不良信息。对于上述内容，平台将持续予以处置。
	`
	words := dict.Segment(text)
	fmt.Println("分词结果:", words)

	text = `张扬不如克制，放纵不如收敛。
微信，已经成为网络时代人们无法离开的社交软件。
有一些人，无论大事小事，都喜欢分享到朋友圈，恨不得把自己所有的生活都搬进去。
记录生活本无可厚非，但也要知道，人外有人，天外有天，这些东西千万不要随意在朋友圈炫耀。否则，轻则招人耻笑轻视，重则遭祸破财。
杨绛先生就曾说：
“别晒幸福，别晒甜蜜，别晒发达，也别晒成功，物理常识告诉我们，晒总会失去水分，藏才是保鲜的最好方式。”
`

	// 从文本学习新词
	if err := dict.LearnFromText(text); err != nil {
		log.Printf("学习新词失败: %v", err)
	}

	// 再次分词，查看学习效果
	newText := `平常中藏财气《西游记》中有这样一段故事，我至今印象深刻：
唐僧与孙悟空二人来到一座观音禅院，院里的老和尚拿出自己的宝贝向师徒二人炫耀。
孙悟空一看便按捺不住，忙拿出唐僧的袈裟来与老和尚一较高下。
唐僧很惶恐，告诫悟空：“徒儿，莫要与人斗富，你我是单身在外，只恐有错。”
涉世未深的孙悟空却不解：“看看袈裟，有何差错？”
果不其然，那老和尚看了唐僧的袈裟后起了歹心，想将其据为己有，与众僧合计要夜里火烧唐僧师徒二人。
后来，师徒二人虽然脱离了火海，但袈裟被黑熊岭的黑熊精给偷走了。
常听老人说，财不外露，富不露相。大抵就是这个道理。
千万不要低估人性的贪婪。
虽然这世上的不少关系，都得好好谈钱，但跟别人谈自己的钱，就大可不必。
挣钱与花钱，克制与享受，都是自己的私事。在明面上摆的钱，都是日后的麻烦。
	`
	newWords := dict.Segment(newText)
	fmt.Println("学习后的分词结果:", newWords)
}
