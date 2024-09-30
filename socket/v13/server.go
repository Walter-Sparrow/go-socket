package v13

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
)

func Upgrade(w http.ResponseWriter, r *http.Request) (*Connection, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return nil, fmt.Errorf("server: Hijacking not supported")
	}

	conn, buf, err := hj.Hijack()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return nil, fmt.Errorf("server: Could not hijack connection: %v", err)
	}

	sHost := conn.LocalAddr().String()
	if !validateHeaders(conn, buf, r.Header, sHost, r.Host) {
		return nil, fmt.Errorf("server: Invalid headers")
	}

	serverHandshake(buf, r.Header)
	return NewConnection(conn), nil
}

func validateHeaders(conn net.Conn, buf *bufio.ReadWriter, headers http.Header, sHost string, cHost string) bool {
	cHost = strings.Replace(cHost, "localhost", "127.0.0.1", 1)

	if cHost != sHost {
		errorWithStatus(conn, buf, http.StatusBadRequest, http.Header{})
		log.Printf("server: Invalid host: %s, expected: %s", cHost, sHost)
		return false
	}

	if strings.ToLower(headers.Get("Upgrade")) != "websocket" {
		errorWithStatus(conn, buf, http.StatusBadRequest, http.Header{})
		return false
	}

	if strings.ToLower(headers.Get("Connection")) != "upgrade" {
		errorWithStatus(conn, buf, http.StatusBadRequest, http.Header{})
		return false
	}

	key := headers.Get("Sec-WebSocket-Key")
	if keyBytes, err := base64.StdEncoding.DecodeString(key); err != nil || len(keyBytes) != 16 {
		errorWithStatus(conn, buf, http.StatusBadRequest, http.Header{})
		log.Printf("server: Invalid key: %s", key)
		return false
	}

	if headers.Get("Sec-WebSocket-Version") != "13" {
		errorWithStatus(conn, buf, http.StatusUpgradeRequired, http.Header{
			"Sec-WebSocket-Version": {"13"},
		})
		return false
	}

	return true
}

func serverHandshake(buf *bufio.ReadWriter, headers http.Header) {
	key := headers.Get("Sec-WebSocket-Key")

	subprotocol := headers.Get("Sec-WebSocket-Protocol")
	if subprotocol != "" {
		subprotocol, _, _ = strings.Cut(subprotocol, ",")
	}

	extensions := headers.Get("Sec-WebSocket-Extensions")
	if extensions != "" {
		extensions, _, _ = strings.Cut(extensions, ";")
	}

	buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	buf.WriteString("Upgrade: websocket\r\n")
	buf.WriteString("Connection: Upgrade\r\n")
	buf.WriteString(fmt.Sprintf("Sec-WebSocket-Accept: %s\r\n", computeAcceptKey(key)))
	if subprotocol != "" {
		buf.WriteString(fmt.Sprintf("Sec-WebSocket-Protocol: %s\r\n", subprotocol))
	}
	if extensions != "" {
		buf.WriteString(fmt.Sprintf("Sec-WebSocket-Extensions: %s\r\n", extensions))
	}
	buf.WriteString("\r\n")
	buf.Flush()
}

func computeAcceptKey(key string) string {
	hash := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func errorWithStatus(conn net.Conn, buf *bufio.ReadWriter, status int, header http.Header) {
	buf.Write([]byte(fmt.Sprintf("HTTP/1.1 %d %s\r\n", status, http.StatusText(status))))
	for key, values := range header {
		for _, value := range values {
			buf.Write([]byte(fmt.Sprintf("%s: %s\r\n", key, value)))
		}
	}
	buf.Write([]byte("\r\n"))
	buf.Flush()
	conn.Close()
}
