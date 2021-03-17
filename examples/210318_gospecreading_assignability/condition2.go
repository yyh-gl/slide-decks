package main

import "fmt"

const one int = 1

type (
	T1 struct{ number int } // T1のunderlying typeはstruct{ number int }
	T2 int                  // T2のunderlying typeはint
)

func main() {
	// underlying typeが同じ（struct{ number int }）であり、
	// なおかつ、一方（struct{ number int }{number: 1}）がdefined typeではないため代入可能である
	var t1 T1 = struct{ number int }{number: 1}
	fmt.Println(t1) // {1}

	// underlying typeは同じ（int）だが、T2およびintがともにdefined typeであるため代入不可能である
	var t2 T2 = one
}
