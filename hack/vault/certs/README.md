To rotate CA:
```
openssl req -x509 -sha256 -new -nodes -key vault-ca.key -days 3650 -out vault-ca.crt
```

To rotate certs:

```
openssl x509 -req -in concourse.csr -days 3650 -CA vault-ca.crt -CAkey vault-ca.key -CAcreateserial -out concourse.crt -extfile extfile-concourse.cnf
openssl x509 -req -in vault.csr -days 3650 -CA vault-ca.crt -CAkey vault-ca.key -CAcreateserial -out vault.crt -extfile extfile-vault.cnf
```
