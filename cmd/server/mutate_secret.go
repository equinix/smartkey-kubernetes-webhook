package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func mutateSecretRequest(w http.ResponseWriter, r *http.Request) {
	log.Print("Handling webhook secret request ...")

	var writeErr error
	if bytes, err := mutateSecret(w, r); err != nil {
		log.Printf("Error handling webhook request: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr = w.Write([]byte(err.Error()))
	} else {
		log.Print("Webhook request secret handled successfully")
		_, writeErr = w.Write(bytes)
	}

	if writeErr != nil {
		log.Printf("Could not write response: %v", writeErr)
	}
}

func mutateSecret(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("could not read request body: %v", err)
	}

	var admissionReviewReq v1.AdmissionReview

	if _, _, err := universalDeserializer.Decode(body, nil, &admissionReviewReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("could not deserialize request: %v", err)
	} else if admissionReviewReq.Request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, errors.New("malformed admission review: request is nil")
	}

	admissionReviewResponse := v1.AdmissionResponse{
		UID: admissionReviewReq.Request.UID,
	}

	patchOps, err := encryptSecret(admissionReviewReq.Request)
	if err != nil {
		admissionReviewResponse.Allowed = false
		admissionReviewResponse.Result = &metav1.Status{
			Message: err.Error(),
		}
	} else {
		patchBytes, err := json.Marshal(patchOps)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return nil, fmt.Errorf("could not marshal JSON patch: %v", err)
		}

		patchType := v1.PatchTypeJSONPatch
		admissionReviewResponse.PatchType = &patchType
		admissionReviewResponse.Allowed = true
		admissionReviewResponse.Patch = patchBytes
		admissionReviewResponse.Result = &metav1.Status{
			Status: "Success",
		}
	}

	admissionReviewReq.Response = &admissionReviewResponse
	bytes, err := json.Marshal(&admissionReviewReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling response: %v", err)
	}

	return bytes, nil
}

func encryptSecret(req *v1.AdmissionRequest) ([]patchOperation, error) {
	raw := req.Object.Raw
	secret := corev1.Secret{}
	if _, _, err := universalDeserializer.Decode(raw, nil, &secret); err != nil {
		return nil, fmt.Errorf("could not deserialize secret object: %v", err)
	}

	config := loadSmartKeyConfig()

	var patches []patchOperation
	for key, data := range secret.Data {
		dataEncrypt, err := encryptSmartKey(config, string(data))
		if err != nil {
			log.Fatalf("Error encrypting secret(%s): %v", data, err)
		}

		dataJSON, _ := json.Marshal(dataEncrypt)
		patches = append(patches, patchOperation{
			Op:    "replace",
			Path:  "/data/" + key,
			Value: []byte(dataJSON),
		})
	}

	return patches, nil
}
