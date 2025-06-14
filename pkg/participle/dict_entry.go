package participle

// DictEntry 字典词条
type DictEntry struct {
	Content   string  `json:"content"`   // 词条内容
	Frequency float64 `json:"frequency"` // 词频
	Pos       string  `json:"pos"`       // 词性
}
