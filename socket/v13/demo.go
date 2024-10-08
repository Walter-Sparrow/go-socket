package v13

import (
	"log"
	"net/http"
)

func Demo() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrade(w, r)
		if err != nil {
			log.Println(err)
			return
		}

		if err := conn.Write(OpText, []byte("Hello, world!")); err != nil {
			log.Println(err)
			return
		}

		for {
			_, message, err := conn.Read()
			if err != nil {
				log.Println(err)
				return
			}
			log.Printf("server: Received message: '%s'", message)
		}
	})
	http.ListenAndServe("127.0.0.1:6969", mux)
}
