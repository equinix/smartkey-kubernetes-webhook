#!/usr/bin/env bash

: ${1?'missing key directory'}

key_dir="$1"

chmod 0700 "$key_dir"
cp deployment/cert.conf $key_dir/cert.conf
cd "$key_dir"

openssl req -nodes -new -x509 -keyout ca.key -out ca.crt -subj "/CN=SmartKey Mutate Controller Webhook CA"
openssl genrsa -out webhook-server-tls.key 2048 
openssl req -new -key webhook-server-tls.key -config cert.conf -extensions 'req_ext' \
    | openssl x509 -req -CA ca.crt -CAkey ca.key -CAcreateserial -out webhook-server-tls.crt -extfile cert.conf -extensions 'req_ext'