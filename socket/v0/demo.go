package v0

import (
	"log"
	"net/http"
	"time"
)

func Demo() {
	go func() {
		time.Sleep(5 * time.Second)
		client, err := NewClient(":6969", "/ws", http.Header{
			"Host":   []string{"localhost"},
			"Origin": []string{"http://localhost"},
		})
		if err != nil {
			log.Println(err)
			return
		}

		quit := make(chan bool)
		defer func() {
			client.Close()
			quit <- true
		}()

		message := []byte("hello")
		log.Printf("client: Sending message: '%s'", message)
		if err := client.Send(message); err != nil {
			log.Println(err)
		}

		time.Sleep(1 * time.Second)

		go func() {
			for {
				select {
				case <-quit:
					return
				default:
					message, err := client.Read()
					if err != nil {
						log.Println(err)
						break
					}

					log.Printf("client: Recv '%s'", message)
				}
			}
		}()
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrade(w, r)
		if err != nil {
			log.Println(err)
			return
		}

		message := []byte("hello from server")
		log.Printf("server: Sending message: '%s'", message)
		if err := conn.Write(TextMessage, message); err != nil {
			log.Println(err)
		}

		go func() {
			for {
				message, err := conn.Read()
				if err != nil {
					log.Println(err)
					break
				}

				log.Printf("server: Recv '%s'", message)
			}

		}()
	})
	http.ListenAndServe(":6969", mux)
}
