package main

import (
	"os"

	v0 "github.com/Walter-Sparrow/go-socket/socket/v0"
)

func main() {
	arg1 := os.Args[1]

	if arg1 == "v0" {
		v0.Demo()
	}
}
