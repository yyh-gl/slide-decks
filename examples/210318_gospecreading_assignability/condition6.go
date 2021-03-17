package main

import "fmt"

// 型が宣言されていない＝型無し定数
const x = 1000

func main() {
	var t1 int = x  // x=1000はintで表現可能である
	fmt.Println(t1) // 1000

	var t2 int8 = x // x=1000はint8（-128~127）で表現できないため代入不可能である
}
