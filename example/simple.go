package main

import "github.com/danielwiratman/gologger"

var L *gologger.Log = gologger.L

func main() {
	defer L.Close()

	L.INF("Hello World")
}
