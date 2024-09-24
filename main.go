package main

import (
	"log"
	"net/http"
	"time"

	v0 "github.com/Walter-Sparrow/go-socket/socket/v0"
)

func main() {
	go func() {
		time.Sleep(5 * time.Second)
		client, err := v0.NewClient(":6969", "/ws", http.Header{
			"Host":   []string{"localhost"},
			"Origin": []string{"http://localhost"},
		})
		if err != nil {
			log.Println(err)
		}
		defer client.Close()

		message := []byte("hello")
		log.Printf("client: sending message: %s", message)
		client.Send(message)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := v0.Upgrade(w, r)
		if err != nil {
			log.Println(err)
			return
		}
		defer conn.Close()

		for {
			message, err := conn.Read()
			if err != nil {
				log.Println(err)
				break
			}

			log.Printf("server: recv %s", message)
		}
	})
	http.ListenAndServe(":6969", mux)
}
