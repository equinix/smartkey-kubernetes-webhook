# SmartKey Mutate Webhook for Kubernetes Secrets

This is an alpha [webhook that mutates](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#mutating-webhook-auditing-annotations) Kubernetes secrets to encrypt its content using an encryption key from SmartKey. When a pod wants to use a secret, the mutate webhook injects an [init container](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/) that decrypts the secrets with SmartKey and stores them in a shared volume in memory with the pod. All data secrets are available under the `/smk/secrets/` folder.

---
## Requirements

* An [SmartKey account](https://www.equinix.com/services/edge-services/smartkey/)
* Kubernetes v1.16 or higher
* Ensure that the `MutatingAdmissionWebhook` controller is enabled
* Ensure that the `admissionregistration.k8s.io/v1` API is enabled

If you can run the following command without getting any error, you're good to go:

```
kubectl get mutatingwebhookconfigurations
```

## How to Install the Webhook

Before you start, you need to get the SmartKey setting values for the credentials secret below:

* Use the endpoint URL from your SmartKey account for the `SMARTKEY_URL` setting.
* Generate a (or use an existing) API key from your account for the `SMARTKEY_API_KEY` setting.
* Generate a (or use an existing) security object, and take note of the object UUID for the `SMARTKEY_OBJECT_UUID` setting.

Clone this repository locally, and run the following commands using the setting values from above:

```
cd smartkey-kubernetes-webhook
kubectl create namespace smartkey-vault
kubectl create secret generic smk-credentials \
 --from-literal=SMARTKEY_URL='https://eu.smartkey.io' \
 --from-literal=SMARTKEY_OBJECT_UUID='{YOUR_SECURITY_OBJECT_UUID}' \
 --from-literal=SMARTKEY_API_KEY='{YOUR_API_KEY}' \
 -n smartkey-vaul --dry-run=client -o yaml | kubectl apply -f -
./deploy.sh
```

Confirm that the webhook is running:

```
kubectl get mutatingwebhookconfigurations
kubectl describe mutatingwebhookconfiguration smartkey-webhook
kubectl get all -n smartkey-vault
```

## How to Encrypt Secrets

### Label a Namespace

The mutate webhook only works if your namespace has the proper label. 

To label a namespace, run the following command:

```
kubectl label namespace default smartkey-vault=enabled
```


### Generate a Secret

Creating a secret doesn't change, you simply run a command in the cluster like the following one:

```
kubectl create secret generic db-user-pass \
 --from-file=./username.txt --from-file=./password.txt \
 --dry-run=client -o yaml | kubectl apply -f -
```

If you try to get the values from the secret, they're still encoded in base64 but its content is encrypted.

```
kubectl get secret db-user-pass -o json
echo ENCODEDVALUE | base64 --decode
```

### Include SmartKey Annotations

Once you create the secret, the only way to decrypt it is by including the annotations in the pod referencing the scret name, like this:

```
  annotations:
    smartkey.io/agent-secret-credentials: "db-user-pass"
```

Based on the above anotation, the name of the secret is `db-user-pass` and the name of the folder available in the container will be `credentials`.

What does that mean? When you create a POD like the following one:

```
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  labels:
    app: nginx
  annotations:
    smartkey.io/agent-secret-credentials: "db-user-pass"
spec:
  containers:
  - name: nginx
    image: nginx:1.14.2
    ports:
    - containerPort: 80
```

You can inspect the init container logs in case something fails or you want to confirm that everything worked.

```
kubectl logs nginx -c smk-decrypt-init
```

You can get access to the secrets at `/smk/secrets/credentials/`, you can give it a try by running the following commands:

```
kubectl exec nginx -c nginx -- ls -la /smk/secrets
kubectl exec nginx -c nginx -- ls -la /smk/secrets/credentials
kubectl exec nginx -c nginx -- cat /smk/secrets/credentials/password.txt
kubectl exec nginx -c nginx -- cat /smk/secrets/credentials/username.txt
```

## How to Uninstall the Webhook

Simply run the following command:

```
kubectl delete -f deployment/
```

## Roadmap

* Cover more use cases for secrets configuration besides the `data` type
* Provide the option to use secrets in environment variables (although it's not recommended)
* Support for automatically decrypt a secret when the data changes
