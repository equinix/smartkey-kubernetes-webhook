# Encrypting Secrets with SmartKey in Kubernetes

Confirm that the mutate webhook is available:

```
$ kubectl get mutatingwebhookconfigurations
$ kubectl describe mutatingwebhookconfiguration smartkey-webhook
```

Create a secret in a traditional way:

```
$ kubectl delete secret db-user-pass
$ kubectl create secret generic db-user-pass --from-file=./username.txt --from-file=./password.txt --dry-run=client -o yaml | kubectl apply -f -
$ kubectl get secret db-user-pass -o json
```

Create a pod with the proper annotations for testing purposes:

```
$ kubectl apply -f pod.yaml
$ kubectl logs nginx -c smk-decrypt-init
$ kubectl exec nginx -- ls -la /smk/secrets
$ kubectl exec nginx -- ls -la /smk/secrets/db-user-pass
$ kubectl exec nginx -- cat /smk/secrets/db-user-pass/password.txt
$ kubectl exec nginx -- cat /smk/secrets/db-user-pass/username.txt
```
