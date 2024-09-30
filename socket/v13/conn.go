package v13

import (
	"log"
	"net"
)

type Connection struct {
	conn net.Conn
}

func NewConnection(conn net.Conn) *Connection {
	return &Connection{conn: conn}
}

func (c *Connection) Close() error {
	return c.conn.Close()
}

func (c *Connection) Write(message []byte) error {
	frame := NewTextFrame(message).Bytes()
	log.Printf("client: Sending frame: '%x'", frame)
	_, err := c.conn.Write(frame)

	return err
}
