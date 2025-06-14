package main

import (
	"fmt"

	"github.com/miajio/nla/pkg/badger"
)

type UserModel struct {
	Id       int
	Name     string
	Password string
}

func main() {
	be, err := badger.Default("../parser_03/gse_dict_db")
	defer be.Close()
	if err != nil {
		fmt.Println(err)
		return
	}

	kb, _ := be.GetKey(nil)
	for _, k := range kb {
		val, _ := be.Get(k)
		fmt.Printf("%s: %s\n", k, val)
	}

	// if err := be.SetAny("user", &UserModel{Id: 1, Name: "miajio", Password: "123456"}); err != nil {
	// 	fmt.Println("set fail:", err)
	// 	return
	// }

	// if err := be.Backup("bdg.backup"); err != nil {
	// 	fmt.Println("backup fail:", err)
	// 	return
	// }
}
