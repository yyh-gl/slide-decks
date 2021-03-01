package main

import "fmt"

const one int = 1

type (
	T1 struct{ number int }
	T2 int // T2のunderlying typeはint
)

func main() {
	// t1ののunderlying typeはstruct{ number int } && defined typeではないため代入可能である
	var t1 T1 = struct{ number int }{number: 1}
	fmt.Println(t1) // {1}

	var t2 T2 = one // oneのunderlying typeはintだが、defined typeであるため代入不可能である
	fmt.Println(t2)
}
