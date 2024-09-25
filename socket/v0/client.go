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
	conn    net.Conn
	closing bool
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

	err := writeBytes(conn, buf)
	if err != nil {
		return err
	}

	var field []byte
	for {
		b, err := readByte(conn)
		if err != nil {
			return fmt.Errorf("client: Can't read handshake response")
		}

		field = append(field, b)

		if b == 0x0a {
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
			b, err := readByte(conn)
			if err != nil {
				return fmt.Errorf("client: Error reading handshake headers")
			}

			if b == 0x0d {
				if len(name) == 0 {
					break Fields
				} else {
					conn.Close()
					return fmt.Errorf("client: Error reading handshake headers")
				}
			}

			if b == 0x0a {
				conn.Close()
				return fmt.Errorf("client: Error reading handshake headers")
			}

			if b == 0x3a {
				break
			}

			if b >= 0x41 && b <= 0x5a {
				name = append(name, (b + 0x20))
			} else {
				name = append(name, b)
			}
		}

		count := 0
		for {
			b, err := readByte(conn)
			if err != nil {
				return fmt.Errorf("client: Error reading handshake headers")
			}
			count++

			if b == 0x20 && count == 1 {
				continue
			}

			if b == 0x0d {
				break
			}

			if b == 0x0a {
				conn.Close()
				return fmt.Errorf("client: Error reading handshake headers")
			} else {
				value = append(value, b)
			}
		}

		if b, err := readByte(conn); err != nil || b != 0x0a {
			return fmt.Errorf("client: Error reading handshake headers")
		}

		fields[string(name)] = string(value)
	}

	if b, err := readByte(conn); err != nil || b != 0x0a {
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

	reply, err := readBytes(conn, 16)
	if err != nil {
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
			char = rand.IntN(69 /* nice */) + 58
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

	if err := writeByte(c.conn, 0x00); err != nil {
		return fmt.Errorf("client: Failed to write message type")
	}

	if err := writeBytes(c.conn, message); err != nil {
		return fmt.Errorf("client: Failed to write message")
	}

	if err := writeByte(c.conn, 0xFF); err != nil {
		return fmt.Errorf("client: Failed to write message terminator")
	}

	return nil
}

func (c *Client) Read() (string, error) {
	frameType, err := readByte(c.conn)
	if err != nil {
		return "", fmt.Errorf("client: Failed to read message type")
	}

	var data string
	isError := false
	if frameType&0x80 == 0x80 {
		length := 0
		for {
			b, err := readByte(c.conn)
			if err != nil {
				return "", fmt.Errorf("client: Failed to read length")
			}

			bV := int(b & 0x7F)
			length = length*128 + bV
			if b&0x80 == 0x80 {
				continue
			}

			if _, err := readBytes(c.conn, length); err != nil {
				return "", fmt.Errorf("client: Failed to read message")
			}
			break
		}

		if frameType == 0xFF && length == 0 {
			if !c.closing {
				c.Close()
			}
			c.conn.Close()
			return "", fmt.Errorf("client: Connection closed")
		} else {
			isError = true
		}
	} else if frameType&0x80 == 0x00 {
		var rawData []byte
		for b, err := readByte(c.conn); b != 0xFF; b, err = readByte(c.conn) {
			if err != nil {
				return "", fmt.Errorf("client: Failed to read message")
			}

			rawData = append(rawData, b)
		}

		if !utf8.Valid(rawData) {
			isError = true
		} else {
			data = string(rawData)
		}

		if frameType != 0x00 {
			isError = true
		}
	}

	if isError {
		return "", fmt.Errorf("client: Invalid message")
	}

	return data, nil
}

func (c *Client) Close() error {
	if err := writeByte(c.conn, 0xFF); err != nil {
		return err
	}

	if err := writeByte(c.conn, 0x00); err != nil {
		return err
	}

	c.closing = true

	return nil
}

func readBytes(conn net.Conn, n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := conn.Read(buf); err != nil && err != io.EOF {
		return nil, err
	}
	return buf, nil
}

func readByte(conn net.Conn) (byte, error) {
	if buf, err := readBytes(conn, 1); err != nil {
		return 0, err
	} else {
		return buf[0], nil
	}
}

func writeBytes(conn net.Conn, b []byte) error {
	if _, err := conn.Write(b); err != nil {
		conn.Close()
		log.Printf("client: Failed to write bytes (%x): %v", b, err)
		return err
	}
	return nil
}

func writeByte(conn net.Conn, b byte) error {
	return writeBytes(conn, []byte{b})
}
