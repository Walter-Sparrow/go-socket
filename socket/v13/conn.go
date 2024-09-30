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
	log.Printf("server: Sending frame: '%s'", frame)
	_, err := c.conn.Write(frame)

	return err
}

func (c *Connection) Read() (*Frame, error) {
	buf := make([]byte, 1024)
	n, err := c.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return ParseFrame(buf[:n])
}
