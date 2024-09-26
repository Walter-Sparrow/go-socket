package v13

import (
	"log"
	"net/http"
)

func Demo() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		_, err := Upgrade(w, r)
		if err != nil {
			log.Println(err)
			return
		}
	})
	http.ListenAndServe("127.0.0.1:6969", mux)
}
