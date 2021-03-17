package main

import "fmt"

func main() {
	var x int = 1
	var t1 int = x  // t1およびxの型はint
	fmt.Println(t1) // 1

	var y string = "yyy"
	var t2 int = y // t2とyの型が異なるのでエラー
}
