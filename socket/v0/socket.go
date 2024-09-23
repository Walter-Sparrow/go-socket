package v0

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type Socket struct {
	ln net.Listener
}

func (s *Socket) Start() {
	ln, err := net.Listen("tcp", ":6969")
	if err != nil {
		panic("Could not start socket: " + err.Error())
	}

	log.Printf("Listening on :6969")
	s.ln = ln

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", handleConnection)
	http.Serve(ln, mux)
}

func (s *Socket) Stop() {
	s.ln.Close()
}

func handleConnection(w http.ResponseWriter, r *http.Request) {
	if !validateHeaders(r.Header) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	key1 := r.Header.Get("Sec-WebSocket-Key1")
	key2 := r.Header.Get("Sec-WebSocket-Key2")

	appendix := make([]byte, 8)
	n, err := r.Body.Read(appendix)
	if (err != nil && err != io.EOF) || n != 8 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Could not read appendix, got %x", appendix)))
		return
	}

	challenge, err := getChallenge(key1, key2, appendix)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Could not get challenge"))
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	conn, buf, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeHandshakeResponse(buf, r.Header, challenge)

	go handleSocketMessage(conn)
}

func validateHeaders(headers http.Header) bool {
	upgradeHeader := headers.Get("Upgrade")
	connectionHeader := headers.Get("Connection")
	key1 := headers.Get("Sec-WebSocket-Key1")
	key2 := headers.Get("Sec-WebSocket-Key2")
	return strings.ToLower(upgradeHeader) == "websocket" && strings.ToLower(connectionHeader) == "upgrade" && key1 != "" && key2 != ""
}

func getChallenge(key1 string, key2 string, challenge []byte) ([16]byte, error) {
	number1, err := getNumberFromKey(key1)
	if err != nil {
		return [16]byte{}, err
	}

	number2, err := getNumberFromKey(key2)
	if err != nil {
		return [16]byte{}, err
	}

	spaces1 := countSpaces(key1)
	if spaces1 == 0 {
		return [16]byte{}, fmt.Errorf("key1 has no spaces")
	}
	spaces2 := countSpaces(key2)
	if spaces2 == 0 {
		return [16]byte{}, fmt.Errorf("key2 has no spaces")
	}

	key1Result := number1 / spaces1
	key2Result := number2 / spaces2

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, key1Result)
	binary.Write(buf, binary.BigEndian, key2Result)
	binary.Write(buf, binary.BigEndian, challenge)

	hash := md5.Sum(buf.Bytes())
	return hash, nil
}

func getNumberFromKey(key string) (int32, error) {
	result := ""
	for i := 0; i < len(key); i++ {
		if key[i] >= '0' && key[i] <= '9' {
			result += string(key[i])
		}
	}

	i64, err := strconv.ParseInt(result, 10, 32)
	if err != nil {
		return 0, err
	}

	return int32(i64), nil
}

func countSpaces(s string) int32 {
	var count int32
	for _, c := range s {
		if c == ' ' {
			count++
		}
	}
	return count
}

func writeHandshakeResponse(buf *bufio.ReadWriter, rHeaders http.Header, challenge [16]byte) {
	buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	writeHandshakeHeaders(buf, rHeaders)
	buf.WriteString("\r\n")
	buf.Write(challenge[:])
	buf.Flush()
}

func writeHandshakeHeaders(buf *bufio.ReadWriter, rHeaders http.Header) {
	addHeader(buf, "Upgrade", "WebSocket")
	addHeader(buf, "Connection", "Upgrade")

	subprotocol := rHeaders.Get("Sec-WebSocket-Protocol")
	if subprotocol != "" {
		addHeader(buf, "Sec-WebSocket-Protocol", subprotocol)
	}

	host := rHeaders.Get("Host")
	addHeader(buf, "Sec-WebSocket-Location", host)

	origin := rHeaders.Get("Origin")
	addHeader(buf, "Sec-WebSocket-Origin", origin)
}

func addHeader(buf *bufio.ReadWriter, header string, value string) {
	hString := fmt.Sprintf("%s: %s\r\n", header, value)
	buf.WriteString(hString)
}

func handleSocketMessage(conn net.Conn) {
	for {
		buf := make([]byte, 1024)
		_, err := conn.Read(buf)
		if err != nil && err != io.EOF {
			log.Printf("Error reading from socket: %s", err)
			conn.Close()
			return
		}

		if buf[0] == 0xFF && buf[1] == 0x00 {
			closeConnection(conn)
			return
		}
	}
}

func closeConnection(conn net.Conn) {
	conn.Write([]byte{0xFF, 0x00})
	conn.Close()
}
