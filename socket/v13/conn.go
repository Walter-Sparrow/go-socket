package v13

import (
	"bufio"
	"log"
	"net"
)

const (
	defaultReadBufferSize = 4096
)

type Connection struct {
	conn net.Conn
	br   *bufio.Reader
}

func NewConnection(conn net.Conn) *Connection {
	br := bufio.NewReaderSize(conn, defaultReadBufferSize)
	return &Connection{conn: conn, br: br}
}

func (c *Connection) Close() error {
	return c.conn.Close()
}

func (c *Connection) Write(message []byte) error {
	frame := NewTextFrame(message).Bytes()
	log.Printf("server: Sending frame: '%s'", frame)
	_, err := c.conn.Write(frame)

	return err
}

func (c *Connection) Read() (messageType byte, message []byte, err error) {
	frame, err := ReadFrame(c.br)
	if err != nil {
		return 0, nil, err
	}
	frame.MaskPayload()
	return frame.Opcode, frame.Payload, nil
}
