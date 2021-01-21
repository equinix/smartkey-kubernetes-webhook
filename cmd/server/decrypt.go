package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func decryptText(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)

	var secret EncryptResponse
	err := decoder.Decode(&secret)
	if err != nil {
		log.Fatal("Error decoding payload: ", err)
	}

	var writeErr error
	config := loadSmartKeyConfig()
	if decryptedData, err := decryptSmartKey(config, secret); err != nil {
		log.Printf("Error handling decryption request: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr = w.Write([]byte(err.Error()))
	} else {
		log.Print("Decryption request handled successfully")
		_, writeErr = w.Write([]byte(decryptedData))
	}

	if writeErr != nil {
		log.Printf("Could not write response: %v", writeErr)
	}
}
