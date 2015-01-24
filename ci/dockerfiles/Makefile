TAG?=latest

all: concourse-ci

push:
		docker push concourse/concourse-ci

concourse-ci: concourse-ci/Dockerfile
		docker build -t concourse/concourse-ci:${TAG} --rm concourse-ci
