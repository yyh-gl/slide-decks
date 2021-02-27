package main

import "fmt"

type (
	T1 interface{}
	T2 interface {
		String() string
	}
)

func main() {
	x := 1
	var t1 T1 = x   // xはインターフェースT1を満たしている
	fmt.Println(t1) // 1

	var t2 T2 = x // xはインターフェースT2を満たしていないので代入不可能である
	fmt.Println(t2)
}
