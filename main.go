package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	muxhttp := http.NewServeMux()
	muxhttp.Handle("/.well-known/", http.FileServer(http.Dir("public")))
	muxhttp.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "https://"+req.Host+req.RequestURI, http.StatusMovedPermanently)
	})
	httpsmux := http.NewServeMux()
	httpsmux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Welcome to aureliar's website")

	})
	go func() {
		log.Println(http.ListenAndServe(":8080", muxhttp))
	}()
	log.Println(http.ListenAndServeTLS(":4443", "fullchain.pem", "privkey.pem", httpsmux))
}
