package v0

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
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
	if err = sendHandshake(conn, pattern, headers); err != nil {
		return nil, err
	}
	log.Print("client: Handshake frame sent")

	return &Client{conn: conn}, nil
}

func sendHandshake(conn net.Conn, pattern string, headers http.Header) error {
	var buf []byte

	buf = append(buf, fmt.Sprintf("GET %s HTTP/1.1\r\nUpgrade: WebSocket\r\nConnection: Upgrade\r\nContent-Length: 8\r\n", pattern)...)
	buf = append(buf, fmt.Sprintf("Sec-WebSocket-Key1: %s\r\nSec-WebSocket-Key2: %s\r\n", websocketKey(), websocketKey())...)
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
	challenge := challengeKey()
	buf = append(buf, challenge[:]...)

	_, err := conn.Write(buf)
	if err != nil {
		return err
	}

	return nil
}

func websocketKey() string {
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
		position := rand.IntN(len(key)) + 1
		key = key[:position] + " " + key[position:]
	}

	return key
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

	if _, err := c.conn.Write([]byte{0x00}); err != nil {
		c.conn.Close()
		return err
	}

	if _, err := c.conn.Write(message); err != nil {
		c.conn.Close()
		return err
	}

	if _, err := c.conn.Write([]byte{0xFF}); err != nil {
		c.conn.Close()
		return err
	}

	return nil
}

func (c *Client) Close() error {
	if _, err := c.conn.Write([]byte{0xFF}); err != nil {
		c.conn.Close()
		return err
	}

	if _, err := c.conn.Write([]byte{0x00}); err != nil {
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
