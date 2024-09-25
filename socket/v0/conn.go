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

// Close closes the underlying connection without a close frame
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

	if err := c.writeByte(0x00); err != nil {
		return fmt.Errorf("conn: Failed to write message type")
	}

	if err := c.writeBytes(message); err != nil {
		return fmt.Errorf("conn: Failed to write message")
	}

	if err := c.writeByte(0xFF); err != nil {
		return fmt.Errorf("conn: Failed to write message terminator")
	}

	return nil
}

func (c *Connection) writeCloseMessage() error {
	if err := c.writeByte(0xFF); err != nil {
		return fmt.Errorf("conn: Failed to write close message")
	}

	if err := c.writeByte(0x00); err != nil {
		return fmt.Errorf("conn: Failed to write close code")
	}

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

func (c *Connection) readByte() (byte, error) {
	buf := make([]byte, 1)

	if _, err := c.conn.Read(buf); err != nil && err != io.EOF {
		c.Close()
		log.Printf("conn: Failed to read byte: %v", err)
		return 0, err
	}

	return buf[0], nil
}

func (c *Connection) writeBytes(b []byte) error {
	if _, err := c.conn.Write(b); err != nil {
		c.Close()
		log.Printf("conn: Failed to write bytes: %v", err)
		return err
	}

	return nil
}

func (c *Connection) writeByte(b byte) error {
	return c.writeBytes([]byte{b})
}
