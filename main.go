package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
)

func main() {
	muxhttp := http.NewServeMux()
	muxhttp.Handle("/.well-known/", http.FileServer(http.Dir("public")))
	muxhttp.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "https://"+req.Host+req.RequestURI, http.StatusMovedPermanently)
	})

	muxhttps := http.NewServeMux()
	muxhttps.Handle("/", http.FileServer(http.Dir("public")))
	server := http.Server{
		Addr:    ":4443",
		Handler: muxhttps,
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
	g, ctx := errgroup.WithContext(context.Background())
	g.Go(func() error {
		if err := server.ListenAndServeTLS("fullchain.pem", "privkey.pem"); err != http.ErrServerClosed {
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := http.ListenAndServe(":8000", muxhttp); err != http.ErrServerClosed {
			return err
		}
		return nil
	})
	go func() {
		<-ctx.Done()
		server.Close()
		//HttpServer.Close()
	}()
	log.Println(g.Wait())
}
