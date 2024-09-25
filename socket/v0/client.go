package v0

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Client struct {
	conn net.Conn
}

func NewClient(address string, pattern string, headers http.Header) (*Client, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	log.Print("client: Preparing handshake")
	if err = handshake(conn, pattern, headers); err != nil {
		return nil, err
	}

	return &Client{conn: conn}, nil
}

func handshake(conn net.Conn, pattern string, headers http.Header) error {
	var buf []byte

	buf = append(buf, fmt.Sprintf("GET %s HTTP/1.1\r\nUpgrade: WebSocket\r\nConnection: Upgrade\r\nContent-Length: 8\r\n", pattern)...)
	key1, number1 := websocketKey()
	key2, number2 := websocketKey()
	buf = append(buf, fmt.Sprintf("Sec-WebSocket-Key1: %s\r\nSec-WebSocket-Key2: %s\r\n", key1, key2)...)
	for key, values := range headers {
		if key == "Upgrade" || key == "Connection" || key == "Content-Length" || key == "Sec-WebSocket-Key1" || key == "Sec-WebSocket-Key2" {
			continue
		}

		for _, value := range values {
			buf = append(buf, key...)
			buf = append(buf, ": "...)
			buf = append(buf, value...)
			buf = append(buf, "\r\n"...)
		}
	}
	buf = append(buf, "\r\n"...)
	key3 := challengeKey()
	buf = append(buf, key3[:]...)

	_, err := conn.Write(buf)
	if err != nil {
		return err
	}

	var field []byte
	var rb [1]byte
	for {
		if _, err := conn.Read(rb[:]); err != nil && err != io.EOF {
			conn.Close()
			return fmt.Errorf("client: Can't read handshake response")
		}

		field = append(field, rb[:]...)

		if rb[0] == 0x0a {
			break
		}
	}

	fieldLen := len(field)
	if fieldLen < 7 || field[fieldLen-2] != 0x0d || field[fieldLen-1] != 0x0a || bytes.Count(field, []byte{0x20}) < 2 {
		conn.Close()
		return fmt.Errorf("client: Unexpected server response")
	}

	_, code, _ := bytes.Cut(field, []byte{0x20})
	code, _, _ = bytes.Cut(code, []byte{0x20})

	if len(code) != 3 || string(code) != "101" {
		conn.Close()
		return fmt.Errorf("client: Wrong server reponse code")
	}

	fields := make(map[string]string)
Fields:
	for {
		var name []byte
		var value []byte

		for {
			if _, err := conn.Read(rb[:]); err != nil && err != io.EOF {
				conn.Close()
				return fmt.Errorf("client: Error reading handshake headers")
			}

			if rb[0] == 0x0d {
				if len(name) == 0 {
					break Fields
				} else {
					conn.Close()
					return fmt.Errorf("client: Error reading handshake headers")
				}
			}

			if rb[0] == 0x0a {
				conn.Close()
				return fmt.Errorf("client: Error reading handshake headers")
			}

			if rb[0] == 0x3a {
				break
			}

			if rb[0] >= 0x41 && rb[0] <= 0x5a {
				name = append(name, (rb[0] + 0x20))
			} else {
				name = append(name, rb[0])
			}
		}

		count := 0
		for {
			if _, err := conn.Read(rb[:]); err != nil && err != io.EOF {
				conn.Close()
				return fmt.Errorf("client: Error reading handshake headers")
			}
			count++

			if rb[0] == 0x20 && count == 1 {
				continue
			}

			if rb[0] == 0x0d {
				break
			}

			if rb[0] == 0x0a {
				conn.Close()
				return fmt.Errorf("client: Error reading handshake headers")
			} else {
				value = append(value, rb[0])
			}
		}

		if _, err := conn.Read(rb[:]); (err != nil && err != io.EOF) || rb[0] != 0x0a {
			conn.Close()
			return fmt.Errorf("client: Error reading handshake headers")
		}

		fields[string(name)] = string(value)
	}

	if _, err := conn.Read(rb[:]); (err != nil && err != io.EOF) || rb[0] != 0x0a {
		conn.Close()
		return fmt.Errorf("client: Error reading handshake headers")
	}

	_, upgradePresent := fields["upgrade"]
	_, connectionPresent := fields["connection"]
	_, secWOPresent := fields["sec-websocket-origin"]
	_, secWLPresent := fields["sec-websocket-location"]
	subprotocol, secWPPresent := fields["sec-websocket-protocol"]
	_, emptyPresent := fields[""]

	isValidProtocol := true
	if secWPPresent {
		isValidProtocol = subprotocol == headers.Get("Sec-WebSocket-Protocol")
	}

	if !upgradePresent || !connectionPresent || !secWOPresent || !secWLPresent || !isValidProtocol || emptyPresent {
		conn.Close()
		return fmt.Errorf("client: Invalid handshake headers")
	}

	for name, value := range fields {
		if name == "upgrade" && value != "WebSocket" {
			conn.Close()
			return fmt.Errorf("client: Wrong Upgrade header in handshake response")
		}

		if name == "connection" && strings.ToLower(value) != "upgrade" {
			conn.Close()
			return fmt.Errorf("client: Wrong Connection header in handshake response")
		}

		if name == "sec-websocket-origin" && value != headers.Get("Origin") {
			conn.Close()
			return fmt.Errorf("client: Wrong Origin header in handshake response")
		}

		// TODO: Sec-WebSocket-Location
	}

	challenge := new(bytes.Buffer)
	binary.Write(challenge, binary.BigEndian, number1)
	binary.Write(challenge, binary.BigEndian, number2)
	binary.Write(challenge, binary.BigEndian, key3)
	expected := md5.Sum(challenge.Bytes())

	var reply [16]byte
	if _, err := conn.Read(reply[:]); err != nil && err != io.EOF {
		conn.Close()
		return fmt.Errorf("client: Could not read challenge")
	}

	if !bytes.Equal(expected[:], reply[:]) {
		conn.Close()
		return fmt.Errorf("client: Wrong challenge in reply, expected: %x, got: %x", expected, reply)
	}

	log.Print("client: Handshake complete")
	return nil
}

func websocketKey() (string, uint32) {
	spaces := rand.IntN(12) + 1
	max := math.MaxUint32 / spaces
	number := rand.IntN(max + 1)
	product := number * spaces
	key := strconv.FormatUint(uint64(product), 10)
	charactersAmount := rand.IntN(12) + 1

	for i := 0; i < charactersAmount; i++ {
		char := -1
		position := rand.IntN(len(key) + 1)
		rangeCoinFlip := rand.IntN(2)
		if rangeCoinFlip == 0 {
			char = rand.IntN(15) + 33
		} else {
			char = rand.IntN(69) + 58
		}

		key = key[:position] + string(char) + key[position:]
	}

	for i := 0; i < spaces; i++ {
		position := rand.IntN(len(key)-1) + 1
		key = key[:position] + " " + key[position:]
	}

	return key, uint32(number)
}

func challengeKey() [8]byte {
	var key [8]byte
	integer := rand.Int64()
	binary.BigEndian.PutUint64(key[:], uint64(integer))
	return key
}

func (c *Client) Send(message []byte) error {
	if !utf8.Valid(message) {
		return fmt.Errorf("client: Message is not valid UTF-8")
	}

	var buf []byte
	buf = append(buf, 0x00)
	buf = append(buf, message...)
	buf = append(buf, 0xFF)

	if _, err := c.conn.Write(buf); err != nil {
		c.conn.Close()
		return err
	}

	return nil
}

func (c *Client) Close() error {
	if _, err := c.conn.Write([]byte{0xFF, 0x00}); err != nil {
		c.conn.Close()
		return err
	}

	// TODO: add read with close message handling
	go func() {
		for {
			var buf [1]byte

			if _, err := c.conn.Read(buf[:]); err != nil {
				c.conn.Close()
				return
			}

			if buf[0] == 0xFF {
				log.Println("client: Connection closed")
				c.conn.Close()
				return
			}
		}
	}()

	return nil
}
