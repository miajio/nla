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

// isAddress 判断字符串是否包含地址信息
func isAddress(s string, provinces, cities, counties []Region) bool {
	for _, p := range provinces {
		if strings.Contains(s, p.Name) {
			return true
		}
	}
	for _, c := range cities {
		if strings.Contains(s, c.Name) {
			return true
		}
	}
	for _, c := range counties {
		if strings.Contains(s, c.Name) {
			return true
		}
	}
	return false
}

// analyzeAddress 分析地址信息
func analyzeAddress(input string, engine *participle.Engine, provinces, cities, counties []Region) AddressInfo {
	// 匹配联系方式，假设联系方式为连续的数字
	reContact := regexp.MustCompile(`\d+`)
	contactMatches := reContact.FindString(input)
	contact := ""
	if len(contactMatches) > 0 {
		contact = contactMatches
		input = strings.ReplaceAll(input, contact, "")
	}

	// 基于任意符号进行分割
	parts := splitBySpecialChar(input)
	parts = removeEmptyStrings(parts)

	name := ""
	addressPart := ""

	if len(parts) > 0 {
		firstPart := parts[0]
		remainingParts := strings.Join(parts[1:], "")

		if !isAddress(firstPart, provinces, cities, counties) {
			name = firstPart
			addressPart = remainingParts
		} else {
			if len(parts) > 1 {
				addressPart = firstPart + parts[len(parts)-2]
				name = parts[len(parts)-1]
			} else {
				addressPart = firstPart
			}
		}
	}

	province := ""
	city := ""
	county := ""
	detailed := ""

	// 匹配省份
	for _, p := range provinces {
		if strings.Contains(addressPart, p.Name) {
			province = p.Name
			addressPart = strings.ReplaceAll(addressPart, province, "")
			break
		}
	}

	// 匹配城市
	for _, c := range cities {
		if strings.Contains(addressPart, c.Name) {
			city = c.Name
			addressPart = strings.ReplaceAll(addressPart, city, "")
			break
		}
	}

	// 匹配区县
	for _, c := range counties {
		if strings.Contains(addressPart, c.Name) {
			county = c.Name
			addressPart = strings.ReplaceAll(addressPart, county, "")
			break
		}
	}

	detailed = strings.TrimSpace(addressPart)

	return AddressInfo{
		Name:     strings.TrimSpace(name),
		Contact:  contact,
		Province: province,
		City:     city,
		County:   county,
		Detailed: detailed,
	}
}

// splitBySpecialChar 基于特殊字符分割字符串
func splitBySpecialChar(s string) []string {
	var parts []string
	var currentPart strings.Builder

	for _, r := range s {
		if participle.IsSpecialChar(string(r)) {
			if currentPart.Len() > 0 {
				parts = append(parts, currentPart.String())
				currentPart.Reset()
			}
		} else {
			currentPart.WriteRune(r)
		}
	}

	if currentPart.Len() > 0 {
		parts = append(parts, currentPart.String())
	}

	return parts
}

// removeEmptyStrings 移除字符串切片中的空字符串
func removeEmptyStrings(slice []string) []string {
	var result []string
	for _, str := range slice {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}

func main() {
	// 初始化数据库引擎
	dbEngine, err := badger.Default("../address_01/address_db")
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
	inputs := []string{
		"张三13800138000广东省深圳市南山区科技园",
		"13800138000广东省深圳市南山区科技园，张",
		"13800138000, 广东省深圳市南山区科技园, 张三",
		"广东省深圳市南山区科技园,张三13800138000",
	}

	for _, input := range inputs {
		info := analyzeAddress(input, engine, provinces, cities, counties)

		fmt.Printf("输入: %s\n", input)
		fmt.Printf("姓名: %s\n", info.Name)
		fmt.Printf("联系方式: %s\n", info.Contact)
		fmt.Printf("省份: %s\n", info.Province)
		fmt.Printf("城市: %s\n", info.City)
		fmt.Printf("区县: %s\n", info.County)
		fmt.Printf("详细地址: %s\n", info.Detailed)
		fmt.Println()
	}
}
