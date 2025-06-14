package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/miajio/nla/pkg/badger"
	"github.com/miajio/nla/pkg/participle"
)

// Region 表示地区信息
type Region struct {
	Name string `json:"name"`
	GB   string `json:"gb"`
}

// AddressInfo 表示分析后的地址信息
type AddressInfo struct {
	Name     string
	Contact  string
	Province string
	City     string
	County   string
	Detailed string
}

// loadRegions 从文件中加载地区信息
func loadRegions(filePath string) ([]Region, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var regions []Region
	err = json.Unmarshal(data, &regions)
	if err != nil {
		return nil, err
	}
	return regions, nil
}

// analyzeAddress 分析地址信息
func analyzeAddress(input string, engine *participle.Engine, provinces, cities, counties []Region) AddressInfo {
	// 分词处理
	words := engine.Segment(input)

	contact := ""
	// 匹配联系方式，假设联系方式为连续的数字
	reContact := regexp.MustCompile(`^\d+$`)
	for _, word := range words {
		if reContact.MatchString(word) {
			contact = word
			break
		}
	}

	// 去除联系方式，得到剩余部分
	remaining := strings.ReplaceAll(input, contact, "")

	province := ""
	city := ""
	county := ""
	detailed := ""

	// 匹配省份
	for _, p := range provinces {
		if strings.Contains(remaining, p.Name) {
			province = p.Name
			remaining = strings.ReplaceAll(remaining, province, "")
			break
		}
	}

	// 匹配城市
	for _, c := range cities {
		if strings.Contains(remaining, c.Name) {
			city = c.Name
			remaining = strings.ReplaceAll(remaining, city, "")
			break
		}
	}

	// 匹配区县
	for _, c := range counties {
		if strings.Contains(remaining, c.Name) {
			county = c.Name
			remaining = strings.ReplaceAll(remaining, county, "")
			break
		}
	}

	// 去除省、市、区县后，剩余部分为详细地址和可能的姓名
	detailedAndName := strings.TrimSpace(remaining)

	// 尝试提取姓名，假设姓名为连续的中文且在详细地址之后
	reName := regexp.MustCompile(`[\p{Han}]+$`)
	match := reName.FindString(detailedAndName)
	name := ""
	if match != "" {
		name = match
		detailed = strings.TrimSpace(strings.ReplaceAll(detailedAndName, name, ""))
	} else {
		detailed = detailedAndName
	}

	return AddressInfo{
		Name:     name,
		Contact:  contact,
		Province: province,
		City:     city,
		County:   county,
		Detailed: detailed,
	}
}

func main() {
	// 初始化数据库引擎
	dbEngine, err := badger.Default("address_db")
	if err != nil {
		fmt.Println("Failed to initialize database engine:", err)
		return
	}
	defer dbEngine.Close()

	// 初始化分词引擎
	engine, err := participle.New(dbEngine)
	if err != nil {
		fmt.Println("Failed to initialize participle engine:", err)
		return
	}

	// 加载省、市、区县信息
	provinces, err := loadRegions("../dict/province.json")
	if err != nil {
		fmt.Println("Failed to load provinces:", err)
		return
	}
	cities, err := loadRegions("../dict/city.json")
	if err != nil {
		fmt.Println("Failed to load cities:", err)
		return
	}
	counties, err := loadRegions("../dict/county.json")
	if err != nil {
		fmt.Println("Failed to load counties:", err)
		return
	}

	// 示例输入
	input := "张三13800138000广东省深圳市南山区科技园"
	info := analyzeAddress(input, engine, provinces, cities, counties)

	fmt.Printf("姓名: %s\n", info.Name)
	fmt.Printf("联系方式: %s\n", info.Contact)
	fmt.Printf("省份: %s\n", info.Province)
	fmt.Printf("城市: %s\n", info.City)
	fmt.Printf("区县: %s\n", info.County)
	fmt.Printf("详细地址: %s\n", info.Detailed)
}
