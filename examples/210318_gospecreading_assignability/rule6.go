package main

import "fmt"

const (
	// 型が宣言されていない＝型無し定数
	x1 = 1
	x2 = "hoge"
)

type T int

func main() {
	var t1 T = x1   // x1（1）はintに変換される→intはTで表現可能である
	fmt.Println(t1) // 1

	var t2 T = x2 // x2（"hoge"）はstringに変換される→stringはTで表現できないため代入不可能である
	fmt.Println(t2)
}
