package main

import (
	"os"

	v0 "github.com/Walter-Sparrow/go-socket/socket/v0"
	v13 "github.com/Walter-Sparrow/go-socket/socket/v13"
)

func main() {
	arg1 := os.Args[1]

	switch arg1 {
	case "v0":
		v0.Demo()
	case "v13":
		v13.Demo()
	}
}
