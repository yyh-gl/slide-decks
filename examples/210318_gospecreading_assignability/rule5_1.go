package main

type (
	Tp *int
	Tf func()
	Ts []int
	Tm map[int]int
	Tc chan int
	Ti interface{}
)

func main() {
	var _ Tp = nil // Tpはポインタ && 値（x）はnil
	var _ Tf = nil // Tfは関数 && 値（x）はnil
	var _ Ts = nil // Tsはスライス && 値（x）はnil
	var _ Tm = nil // Tmはマップ && 値（x）はnil
	var _ Tc = nil // Tcはチャネル && 値（x）はnil
	var _ Ti = nil // Tiはインターフェース && 値（x）はnil
}
