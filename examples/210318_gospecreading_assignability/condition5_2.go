package main

import "fmt"

type T *int

func main() {
	var x func() // 関数型のゼロ値はnil
	var t T = x  // xはnilだが事前宣言された識別子としてのnilではないため代入不可能である
	fmt.Println(t)
}
