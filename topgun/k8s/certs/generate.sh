#!/bin/bash

if [ -e values.yml ]; then
    rm -f values.yml
fi
bosh int --vars-store=values.yml generate.yml
bosh int --vars-store=values.yml --path /service_ssl/ca values.yml > ca.crt
bosh int --vars-store=values.yml --path /service_ssl/certificate values.yml > ssl.crt
bosh int --vars-store=values.yml --path /service_ssl/private_key values.yml > ssl.key
