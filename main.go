package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	http.Handle("/.well-known/", http.FileServer(http.Dir("public")))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Welcome to aureliar's website")
	})
	port := os.Getenv("PORT")
	if port == "" {
		log.Println("PORT environement variable not found ")
	}
	log.Println(http.ListenAndServe("localhost:"+port, nil))
}
