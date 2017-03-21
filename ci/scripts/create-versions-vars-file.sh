#!/bin/sh

concourse_version=$(cat version/version)
garden_runc_version=$(cat garden-runc/version)

touch versions-vars-file/vars.yml

echo "deployment_name: ${deployment_name}" >> versions-vars-file/vars.yml
echo "web_ip:  ${web_ip}" >> versions-vars-file/vars.yml
echo "tls_cert: |" >> versions-vars-file/vars.yml
echo "${tls_cert}" | sed 's/^/  /' >> versions-vars-file/vars.yml
echo "tls_key: | " >> versions-vars-file/vars.yml
echo "${tls_key}" | sed 's/^/  /' >> versions-vars-file/vars.yml
echo "concourse-version: ${concourse_version}" >> versions-vars-file/vars.yml
echo "garden-runc-version: ${garden_runc_version}" >> versions-vars-file/vars.yml
