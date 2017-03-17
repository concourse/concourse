#!/bin/sh

concourse_version=$(cat bosh-rc/version)
garden_runc_version=$(cat garden-runc/version)

touch versions-vars-file/vars.yml

echo "concourse-version: ${concourse_version}" > versions-vars-file/vars.yml
echo "garden-runc-version: ${garden_runc_version}" > versions-vars-file/vars.yml