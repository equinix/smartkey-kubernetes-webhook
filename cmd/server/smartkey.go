package main

import (
	"bytes"
	"encoding/base64"
	b64 "encoding/base64"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

/*EncryptResponse response from SmartKey for encrypt API Call*/
type EncryptResponse struct {
	Cipher string
	Iv     string
}

/*DecryptResponse response from SmartKey for decrypt API Call*/
type DecryptResponse struct {
	Plain string
	Iv    string
}

func encryptSmartKey(config map[string]string, input string) (*EncryptResponse, error) {
	var base64Input = base64.StdEncoding.EncodeToString([]byte(input))
	encryptURL := config["smartkeyURL"] + "/crypto/v1/keys/" + config["encryptionKeyUuid"] + "/encrypt"

	var body, err = smkAPICall(config["smartkeyApiKey"], encryptURL, []byte(`{
		"alg":   "AES",
		"mode":  "CBC",
		"plain": "`+base64Input+`"
	}`))

	if err != nil {
		log.Print("Error reading body. ", err)
		return nil, err
	}

	var response EncryptResponse
	json.Unmarshal([]byte(body), &response)

	return &response, nil
}

func decryptSmartKey(config map[string]string, data EncryptResponse) (string, error) {
	encryptURL := config["smartkeyURL"] + "/crypto/v1/keys/" + config["encryptionKeyUuid"] + "/decrypt"

	var body, err = smkAPICall(config["smartkeyApiKey"], encryptURL, []byte(`{
		"alg":   "AES",
		"mode":  "CBC",
		"iv":    "`+data.Iv+`",
		"cipher": "`+data.Cipher+`"
	}`))

	if err != nil {
		log.Print("Error reading body. ", err)
		return "", err
	}

	var response DecryptResponse
	json.Unmarshal([]byte(body), &response)

	var decodedPlain, _ = b64.StdEncoding.DecodeString(response.Plain)

	return string(decodedPlain), nil
}

func smkAPICall(apikey string, url string, data []byte) ([]byte, error) {

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		log.Fatal("Error reading request. ", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+apikey)

	client := &http.Client{}

	resp, err := client.Do(req)

	if err != nil {
		log.Fatal("Error reading response. ", err)
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func loadSmartKeyConfig() map[string]string {
	config := make(map[string]string)
	config["smartkeyURL"] = readFromFile("/smk/credentials/SMARTKEY_URL")
	config["encryptionKeyUuid"] = readFromFile("/smk/credentials/SMARTKEY_OBJECT_UUID")
	config["smartkeyApiKey"] = readFromFile("/smk/credentials/SMARTKEY_API_KEY")

	return config
}

func readFromFile(path string) string {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		log.Println(err)
		return "N/A"
	}

	return string(content)
}
