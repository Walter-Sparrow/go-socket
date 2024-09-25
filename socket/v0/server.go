package v0

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

func Upgrade(w http.ResponseWriter, r *http.Request) (*Connection, error) {
	if !validateHeaders(r.Header) {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("server: Invalid headers")
	}

	log.Print("server: Starting handshake")

	key1 := r.Header.Get("Sec-WebSocket-Key1")
	key2 := r.Header.Get("Sec-WebSocket-Key2")

	challengeClient := make([]byte, 8)
	n, err := r.Body.Read(challengeClient)
	if (err != nil && err != io.EOF) || n != 8 {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("server: Could not read challenge")
	}

	challenge, err := computeChallenge(key1, key2, challengeClient)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, err
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return nil, fmt.Errorf("server: Hijacking not supported")
	}

	conn, buf, err := hj.Hijack()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return nil, err
	}

	location := conn.LocalAddr().String() + r.URL.Path
	serverHandshake(buf, r.Header, location, challenge)

	return NewConnection(conn), nil
}

func validateHeaders(headers http.Header) bool {
	upgradeHeader := headers.Get("Upgrade")
	connectionHeader := headers.Get("Connection")
	key1 := headers.Get("Sec-WebSocket-Key1")
	key2 := headers.Get("Sec-WebSocket-Key2")
	return strings.ToLower(upgradeHeader) == "websocket" && strings.ToLower(connectionHeader) == "upgrade" && key1 != "" && key2 != ""
}

func computeChallenge(key1 string, key2 string, challenge []byte) ([16]byte, error) {
	number1, err := numberFromKey(key1)
	if err != nil {
		return [16]byte{}, err
	}

	number2, err := numberFromKey(key2)
	if err != nil {
		return [16]byte{}, err
	}

	spaces1 := countSpaces(key1)
	if spaces1 == 0 {
		return [16]byte{}, fmt.Errorf("server: Key1 has no spaces")
	}
	spaces2 := countSpaces(key2)
	if spaces2 == 0 {
		return [16]byte{}, fmt.Errorf("server: Key2 has no spaces")
	}

	key1Result := number1 / uint32(spaces1)
	key2Result := number2 / uint32(spaces2)

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, key1Result)
	binary.Write(buf, binary.BigEndian, key2Result)
	binary.Write(buf, binary.BigEndian, challenge)

	hash := md5.Sum(buf.Bytes())
	return hash, nil
}

func numberFromKey(key string) (uint32, error) {
	var result uint32
	for i := 0; i < len(key); i++ {
		if key[i] >= '0' && key[i] <= '9' {
			result = result*10 + uint32(key[i]-'0')
		}
	}

	if result == 0 {
		return 0, fmt.Errorf("server: Key has no number")
	}

	return result, nil
}

func countSpaces(s string) uint8 {
	var count uint8
	for _, c := range s {
		if c == ' ' {
			count++
		}
	}
	return count
}

func serverHandshake(buf *bufio.ReadWriter, rHeaders http.Header, location string, challenge [16]byte) {
	buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	writeHandshakeHeaders(buf, rHeaders, location)
	buf.WriteString("\r\n")
	buf.Write(challenge[:])
	buf.Flush()
}

func writeHandshakeHeaders(buf *bufio.ReadWriter, rHeaders http.Header, location string) {
	addHeader(buf, "Upgrade", "WebSocket")
	addHeader(buf, "Connection", "Upgrade")

	subprotocol := rHeaders.Get("Sec-WebSocket-Protocol")
	if subprotocol != "" {
		addHeader(buf, "Sec-WebSocket-Protocol", subprotocol)
	}

	addHeader(buf, "Sec-WebSocket-Location", location)

	origin := rHeaders.Get("Origin")
	addHeader(buf, "Sec-WebSocket-Origin", origin)
}

func addHeader(buf *bufio.ReadWriter, header string, value string) {
	hString := fmt.Sprintf("%s: %s\r\n", header, value)
	buf.WriteString(hString)
}
