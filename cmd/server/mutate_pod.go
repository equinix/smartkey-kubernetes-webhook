package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	b64 "encoding/base64"

	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Secret struct {
	Name, Path string
}

func mutatePodRequest(w http.ResponseWriter, r *http.Request) {
	log.Print("Handling webhook pod request ...")

	var writeErr error
	if bytes, err := mutatePod(w, r); err != nil {
		log.Printf("Error handling webhook request: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr = w.Write([]byte(err.Error()))
	} else {
		log.Print("Webhook request pod handled successfully")
		_, writeErr = w.Write(bytes)
	}

	if writeErr != nil {
		log.Printf("Could not write response: %v", writeErr)
	}
}

func mutatePod(w http.ResponseWriter, r *http.Request) ([]byte, error) {
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

	patchOps, err := decryptSecret(admissionReviewReq.Request)
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

func decryptSecret(req *v1.AdmissionRequest) ([]patchOperation, error) {
	raw := req.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := universalDeserializer.Decode(raw, nil, &pod); err != nil {
		return nil, fmt.Errorf("could not deserialize pod object: %v", err)
	}

	// Create volumes for each Secret
	var patches []patchOperation
	patches = append(patches, appendVolumes(
		pod.Spec.Volumes,
		generateSecretVolumes(&pod),
		"/spec/volumes")...)

	// Mount volumes for each Secret
	for i, container := range pod.Spec.Containers {
		patches = append(patches, appendVolumeMounts(
			container.VolumeMounts,
			containerSecretMounts(&pod),
			fmt.Sprintf("/spec/containers/%d/volumeMounts", i))...)
	}

	// Inject the initContainer
	initContainer := corev1.Container{
		Name:            "smk-decrypt-init",
		Image:           "christianhxc/smartkey-decrypt-service:0.1",
		ImagePullPolicy: "Always",
		Env:             containerEnvVars(&pod),
		VolumeMounts:    containerSecretMounts(&pod),
	}

	patches = append(patches, appendContainers(
		pod.Spec.InitContainers,
		[]corev1.Container{initContainer},
		fmt.Sprintf("/spec/initContainers"))...)

	return patches, nil
}

func appendVolumes(target, volumes []corev1.Volume, base string) []patchOperation {
	var result []patchOperation
	first := len(target) == 0
	var value interface{}
	for _, v := range volumes {
		value = v
		path := base
		if first {
			first = false
			value = []corev1.Volume{v}
		} else {
			path = path + "/-"
		}

		result = append(result, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return result
}

func appendVolumeMounts(target, mounts []corev1.VolumeMount, base string) []patchOperation {
	var result []patchOperation
	first := len(target) == 0
	var value interface{}
	for _, v := range mounts {
		value = v
		path := base
		if first {
			first = false
			value = []corev1.VolumeMount{v}
		} else {
			path = path + "/-"
		}

		result = append(result, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return result
}

func appendContainers(target, containers []corev1.Container, base string) []patchOperation {
	var result []patchOperation
	first := len(target) == 0
	var value interface{}
	for _, container := range containers {
		value = container
		path := base
		if first {
			first = false
			value = []corev1.Container{container}
		} else {
			path = path + "/-"
		}

		result = append(result, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}

	return result
}

func generateSecretVolumes(pod *corev1.Pod) []corev1.Volume {
	containerVolumes := []corev1.Volume{
		corev1.Volume{
			Name: "smk-secrets",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: "Memory",
				},
			},
		},
	}

	secrets := getSecretsList(pod)
	for _, secret := range secrets {
		containerVolumes = append(containerVolumes, corev1.Volume{
			Name: secret.Name,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Path,
				},
			},
		})
	}

	return containerVolumes
}

func containerSecretMounts(pod *corev1.Pod) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		corev1.VolumeMount{
			Name:      "smk-secrets",
			MountPath: "/smk/secrets",
			ReadOnly:  false,
		},
	}

	// TODO: Mount only in the initContainer
	secrets := getSecretsList(pod)
	for _, secret := range secrets {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      secret.Name,
			MountPath: "/smk/encrypt/secrets/" + secret.Path,
			ReadOnly:  false,
		})
	}

	return volumeMounts
}

func containerEnvVars(pod *corev1.Pod) []corev1.EnvVar {
	var envs []corev1.EnvVar

	envs = append(envs, corev1.EnvVar{
		Name:  "SVC_ENDPOINT",
		Value: os.Getenv("SVC_ENDPOINT"),
	})

	secrets := getSecretsList(pod)
	secretsJSON, _ := json.Marshal(&secrets)
	secretsBase64 := b64.StdEncoding.EncodeToString(secretsJSON)
	envs = append(envs, corev1.EnvVar{
		Name:  "SECRETS",
		Value: secretsBase64,
	})

	return envs
}

func getSecretsList(pod *corev1.Pod) []Secret {
	secrets := []Secret{}
	for annotation, annotationValue := range pod.Annotations {
		smkPrefix := fmt.Sprintf("%s-", "smartkey.io/agent-secret")
		if strings.Contains(annotation, smkPrefix) {
			secretName := strings.ReplaceAll(annotation, smkPrefix, "")
			secret := Secret{Name: secretName, Path: annotationValue}
			secrets = append(secrets, secret)
		}
	}

	return secrets
}
