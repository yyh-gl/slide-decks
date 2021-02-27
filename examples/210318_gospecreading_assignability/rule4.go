package main

import "fmt"

type T <-chan int // Tはchannel type

func main() {
	x1 := make(chan int, 1) // x1は要素の型がintの双方向チャネル
	var t1 T = x1           // t1およびx1の要素の型は同じ（int）
	fmt.Println(t1)         // アドレス情報

	x2 := make(chan string, 1) // x2は要素の型がstringの双方向チャネル
	var t2 T = x2              // t2およびx2の要素の型が異なるので代入不可能である
	fmt.Println(t2)
}
