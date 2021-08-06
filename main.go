package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
)

func main() {
	muxhttp := mux.NewRouter()
	muxhttp.Handle("/.well-known/", http.FileServer(http.Dir("public")))
	muxhttp.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "https://"+req.Host+req.RequestURI, http.StatusMovedPermanently)
	})

	muxhttps := mux.NewRouter()
	muxhttps.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found")
	})
	muxhttps.Handle("/", http.FileServer(http.Dir("public")))
	serverHttps := NewServer(":4443", handlers.CombinedLoggingHandler(os.Stdout, muxhttps))
	serverHttp := NewServer(":8000", muxhttp)
	g, ctx := errgroup.WithContext(context.Background())
	g.Go(func() error {
		if err := serverHttps.ListenAndServeTLS("fullchain.pem", "privkey.pem"); err != http.ErrServerClosed {
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := serverHttp.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
		return nil
	})
	go func() {
		<-ctx.Done()
		serverHttps.Close()
		serverHttp.Close()
	}()
	log.Println(g.Wait())
}

func NewServer(addr string, handler http.Handler) http.Server {
	return http.Server{
		Addr:    addr,
		Handler: handler,
		// https://blog.cloudflare.com/exposing-go-on-the-internet/
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		TLSConfig: &tls.Config{
			NextProtos:       []string{"h2", "http/1.1"},
			MinVersion:       tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{tls.CurveP256, tls.X25519},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
			PreferServerCipherSuites: true,
		},
	}
}
