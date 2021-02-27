package main

import "fmt"

const num = 1 // 型が宣言されていない＝型無し定数

func main() {
	var i int = num     // 暗黙的な変換により定数numはint型として扱われる
	fmt.Printf("%T", i) // int
}
