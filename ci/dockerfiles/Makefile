TAG?=latest

all: bosh-cli

push:
		docker push concourse/bosh-cli

bosh-cli: bosh-cli/Dockerfile
		docker build -t concourse/bosh-cli:${TAG} --rm bosh-cli
