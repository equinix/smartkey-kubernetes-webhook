package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	b64 "encoding/base64"
	"encoding/json"
)

// Secret structure
type Secret struct {
	Name, Path string
}

const (
	secretsEncryptPath = "/smk/encrypt/secrets"
	secretsPath        = "/smk/secrets"
)

func main() {
	secrets := []*Secret{}
	secretsBase64, _ := b64.StdEncoding.DecodeString(os.Getenv("SECRETS"))
	err := json.Unmarshal(secretsBase64, &secrets)
	if err != nil {
		panic(err)
	}

	if len(secrets) == 0 {
		fmt.Println("There's nothing to to do here, bye!")
		return
	}

	files := getSecretFiles()
	for _, file := range files {
		encryptedData := getRawValue(file)
		decryptedData := decryptSecret(encryptedData)
		persistSecretValue(file, secrets, decryptedData)
	}

	fmt.Println("Done decrypting the secrets")
}

func getSecretFiles() []string {
	var files []string
	err := filepath.Walk(secretsEncryptPath, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			return nil
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		panic(err)
	}

	return files
}

func getRawValue(file string) string {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	return string(content)
}

func persistSecretValue(file string, secrets []*Secret, content string) {
	file = strings.Replace(file, "encrypt/", "", 1)
	file = strings.Replace(file, filepath.Base(filepath.Dir(file))+"/", "", 1)

	for _, secret := range secrets {
		if strings.Contains(file, secret.Path) {
			file = strings.Replace(file, secret.Path, secret.Name, 1)
			break
		}
	}

	os.MkdirAll(filepath.Dir(file), 0700)

	secretFile, err := os.Create(file)
	if err != nil {
		panic(err)
	}

	defer secretFile.Close()

	secretFile.Write([]byte(content))
	secretFile.Sync()
}

func decryptSecret(data string) string {
	svcEndpoint := fmt.Sprintf("https://%s/decrypt", os.Getenv("SVC_ENDPOINT"))
	req, err := http.NewRequest("POST", svcEndpoint, bytes.NewBuffer([]byte(data)))
	if err != nil {
		log.Fatal("Error reading request. ", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// TODO: Need a workaround as the certiticate shouldn't be ignored
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Error reading response. ", err)
	}

	defer resp.Body.Close()

	secret, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Couldn't process response from the decrypt service")
	}

	return string(secret)
}
