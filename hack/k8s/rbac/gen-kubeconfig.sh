#!/bin/bash

# gen-kubeconfig.sh - generates a kubeconfig with access to the
# resources that a service account provides in the cluster.
#
# heavily inspired by Armory's docs[1].
#
# [1]: https://docs.armory.io/spinnaker-install-admin-guides/manual-service-account/
#

set -o errexit
set -o nounset

readonly CONTEXT=$(kubectl config current-context)
readonly NAMESPACE=concourse
readonly SERVICE_ACCOUNT=concourse-target-serviceaccount
readonly KUBECONFIG_FILE=./kubeconfig-sa


main() {
	local token=$(get_token)
	generate_kubeconfig "$token"

	echo "generated: $KUBECONFIG_FILE"
}

get_token() {
	local secret=$(kubectl get serviceaccount $SERVICE_ACCOUNT \
		--context $CONTEXT \
		--namespace $NAMESPACE \
		-o jsonpath='{.secrets[0].name}')

	local token=$(kubectl get secret $secret \
		--context $CONTEXT \
		--namespace $NAMESPACE \
		-o jsonpath='.data.token')

	echo "$token" | base64 -d
}

generate_kubeconfig() {
	local token=$1

	# create dedicated kubeconfig
	# create a full copy
	#
	kubectl config view --raw \
		>${KUBECONFIG_FILE}.full.tmp

	# switch working context to correct context
	#
	kubectl --kubeconfig ${KUBECONFIG_FILE}.full.tmp \
		config use-context ${CONTEXT}

	# minify
	#
	kubectl --kubeconfig ${KUBECONFIG_FILE}.full.tmp \
		config view --flatten --minify \
		>${KUBECONFIG_FILE}.tmp

	# rename context
	#
	kubectl config --kubeconfig ${KUBECONFIG_FILE}.tmp \
		rename-context ${CONTEXT} ${NEW_CONTEXT}

	# create token user
	#
	kubectl config --kubeconfig ${KUBECONFIG_FILE}.tmp \
		set-credentials ${CONTEXT}-${NAMESPACE}-token-user \
		--token ${token}

	# set context to use token user
	#
	kubectl config --kubeconfig ${KUBECONFIG_FILE}.tmp \
		set-context ${NEW_CONTEXT} \
		--user ${CONTEXT}-${NAMESPACE}-token-user

	# set context to correct namespace
	#
	kubectl config --kubeconfig ${KUBECONFIG_FILE}.tmp \
		set-context ${NEW_CONTEXT} \
		--namespace ${NAMESPACE}

	# flatten/minify kubeconfig
	#
	kubectl config --kubeconfig ${KUBECONFIG_FILE}.tmp \
		view --flatten --minify \
		>${KUBECONFIG_FILE}

	# remove tmp
	#
	rm ${KUBECONFIG_FILE}.full.tmp
	rm ${KUBECONFIG_FILE}.tmp
}

main "$@"
