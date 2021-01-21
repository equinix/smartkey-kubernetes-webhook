package main

import (
	"log"
	"net/http"
	"path/filepath"
)

const (
	tlsDir      = `/run/secrets/tls`
	tlsCertFile = `tls.crt`
	tlsKeyFile  = `tls.key`
)

func main() {
	certPath := filepath.Join(tlsDir, tlsCertFile)
	keyPath := filepath.Join(tlsDir, tlsKeyFile)

	mux := http.NewServeMux()
	mux.HandleFunc("/mutatesecret", mutateSecretRequest)
	mux.HandleFunc("/mutatepod", mutatePodRequest)
	mux.HandleFunc("/decrypt", decryptText)
	server := &http.Server{
		Addr:    ":8443",
		Handler: mux,
	}

	log.Fatal(server.ListenAndServeTLS(certPath, keyPath))
}
