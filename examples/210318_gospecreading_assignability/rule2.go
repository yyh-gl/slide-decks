package main

import "fmt"

const one int = 1

type T int // Tのunderlying typeはint

func main() {
	var t T = 1    // 1のunderlying typeはint && 1は型無し定数でありdefined typeではない
	fmt.Println(t) // 1

	var y T = one // oneのunderlying typeはintだが、defined typeであるため代入不可能である
	fmt.Println(y)
}
