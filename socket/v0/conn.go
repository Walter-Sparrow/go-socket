package v0

import (
	"fmt"
	"io"
	"log"
	"net"
	"unicode/utf8"
)

type MessageType int

const (
	TextMessage  = 1
	CloseMessage = 2
)

type Connection struct {
	conn net.Conn
}

func NewConnection(conn net.Conn) *Connection {
	return &Connection{conn: conn}
}

// Close closes the underlying connection with a close frame
func (c *Connection) Close() error {
	return c.conn.Close()
}

func (c *Connection) Write(messageType MessageType, message []byte) error {
	if messageType == TextMessage {
		return c.writeTextMessage(message)
	} else {
		return c.writeCloseMessage()
	}
}

func (c *Connection) writeTextMessage(message []byte) error {
	if !utf8.Valid(message) {
		return fmt.Errorf("conn: Message is not valid UTF-8")
	}

	c.conn.Write([]byte{0x00})
	c.conn.Write(message)
	c.conn.Write([]byte{0xFF})

	return nil
}

func (c *Connection) writeCloseMessage() error {
	c.conn.Write([]byte{0xFF})
	c.conn.Write([]byte{0x00})
	return nil
}

func (c *Connection) Read() ([]byte, error) {
	typeByte, err := c.readByte()
	if err != nil {
		return nil, fmt.Errorf("conn: Failed to read message type")
	}

	if typeByte>>7 == 0 {
		if typeByte != 0x00 {
			c.Close()
			return nil, fmt.Errorf("conn: Invalid message type 0x%02x", typeByte)
		}

		return c.readTextMessage()
	} else {
		if typeByte != 0xFF {
			c.Close()
			return nil, fmt.Errorf("conn: Invalid message type")
		}

		b, err := c.readByte()
		if err != nil {
			return nil, fmt.Errorf("conn: Failed to read close code")
		}

		if b != 0x00 {
			c.Close()
			return nil, fmt.Errorf("conn: Invalid close code")
		}

		c.writeCloseMessage()
		c.Close()
		return nil, fmt.Errorf("conn: Connection closed")
	}
}

func (c *Connection) readByte() (byte, error) {
	buf := make([]byte, 1)

	_, err := c.conn.Read(buf)
	if err != nil && err != io.EOF {
		log.Printf("conn: Error reading from socket: %s", err)
		c.Close()
		return 0, err
	}

	return buf[0], nil
}

func (c *Connection) readTextMessage() ([]byte, error) {
	rawData := new([]byte)

	for {
		b, err := c.readByte()
		if err != nil {
			return nil, fmt.Errorf("conn: Failed to read message")
		}

		if b == 0xFF {
			break
		}

		*rawData = append(*rawData, b)
	}

	return *rawData, nil
}
