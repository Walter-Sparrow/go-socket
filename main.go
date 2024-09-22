package main

import "github.com/Walter-Sparrow/go-socket/socket"

func main() {
	var s socket.Socket
	s.Start()
	defer s.Stop()
}
