#!/bin/bash

set -e -u

cd $(dirname $0)/../..

echo 'generating certs...'
rm -rf hack/vault/certs
certstrap --depot-path hack/vault/certs init --cn vault-ca --passphrase ''
certstrap --depot-path hack/vault/certs request-cert --domain vault --ip 127.0.0.1 --passphrase ''
certstrap --depot-path hack/vault/certs sign vault --CA vault-ca --passphrase ''
certstrap --depot-path hack/vault/certs request-cert --cn concourse --passphrase ''
certstrap --depot-path hack/vault/certs sign concourse --CA vault-ca --passphrase ''

echo
echo 'step 1:'
echo '  docker-compose -f docker-compose.yml -f hack/overrides/vault.yml up -d'
echo
echo 'step 2:'
echo '  export VAULT_CACERT=$PWD/hack/vault/certs/vault-ca.crt'
echo
echo 'step 3:'
echo '  ./hack/vault/init'
