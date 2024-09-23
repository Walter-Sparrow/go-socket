package v0

import (
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestConnection(t *testing.T) {
	var s Socket
	defer s.Stop()
	go s.Start()

	time.Sleep(4 * time.Second)

	conn, err := net.Dial("tcp", ":6969")
	if err != nil {
		t.Errorf("Could not connect to socket: %s", err)
	}

	fmt.Fprint(conn, getHandshakeRequest())

	buf := make([]byte, 1024)
	_, err = conn.Read(buf)

	if err != nil && err != io.EOF {
		t.Errorf("Could not read from socket: %s", err)
	}

	stringifyBuf := string(buf)

	if !strings.Contains(stringifyBuf, "101 Switching Protocols") {
		t.Error("Wrong handshake response status")
	}

	if !strings.Contains(stringifyBuf, "fQJ,fN/4F4!~K~MH") {
		t.Error("Wrong handshake response aknowledgement")
	}

	conn.Write([]byte{0xFF, 0x00})
	buf = make([]byte, 2)
	_, err = conn.Read(buf)
	if err != nil {
		t.Error("Could not read from socket")
	}

	if buf[0] != 0xFF || buf[1] != 0x00 {
		t.Error("Failed closing connection")
	}

	conn.Close()
}

func getHandshakeRequest() string {
	request := "GET /ws HTTP/1.1\r\n"
	request += "Host: example.com\r\n"
	request += "Origin: http://example.com\r\n"
	request += "Upgrade: WebSocket\r\n"
	request += "Connection: Upgrade\r\n"
	request += "Sec-WebSocket-Protocol: sub\r\n"
	request += "Sec-WebSocket-Key1: 18x 6]8vM;54 *(5:  {   U1]8  z [  8\r\n"
	request += "Sec-WebSocket-Key2: 1_ tx7X d  <  nw  334J702) 7]o}` 0\r\n"
	request += "Content-Length: 8\r\n"
	request += "\r\n"
	request += "Tm[K T2u"

	return request
}
