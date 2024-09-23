package main

import v0 "github.com/Walter-Sparrow/go-socket/socket/v0"

func main() {
	var s v0.Socket
	s.Start()
	defer s.Stop()
}
