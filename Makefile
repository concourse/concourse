NAMESPACE ?= concourse

KUBECONFIG ?= $(shell realpath ~/.kube/config)
KUBECTL    ?= kubectl --namespace $(NAMESPACE)

INIT_CONFIGMAP ?= init
INIT_BIN       ?= ./cmd/init/init
INIT_SRC       ?= ./cmd/init/init.c

CONCOURSE_WEB_FLAGS ?= \
	--add-local-user=test:test \
	--build-tracker-interval=1s \
	--enable-global-resources \
	--external-url=http://localhost:8080 \
	--kubernetes-worker-kubeconfig=$(KUBECONFIG) \
	--lidar-scanner-interval=10s \
	--main-team-local-user=test \
	--postgres-database=concourse \
	--postgres-host=localhost \
	--postgres-password=dev \
	--postgres-port=6543 \
	--postgres-user=dev \
	--tsa-host-key=./keys/web/tsa_host_key


# install - installs concourse
#
install:
	go install -v ./cmd/concourse


# run - runs a `web` node that targets a kubernetes cluster pointed by
#       `$(KUBECONFIG)`.
#
run: install init
	kubectl config set-context --current --namespace $(NAMESPACE)
	concourse web $(CONCOURSE_WEB_FLAGS)


# debug - runs a `web` node with `dlv` so that one can debug it.
#
debug: init
	dlv debug ./cmd/concourse -- web $(CONCOURSE_WEB_FLAGS)


# db - brings the database up
#
db:
	docker-compose up -d db


# test - runs a sample workload
#
test:
	fly -t local login -u test -p test
	fly -t local set-pipeline -n -p test -c /tmp/pipeline.yml
	# fly -t local set-pipeline -n -p test -c ./hack/k8s/sample-pipeline.yml
	fly -t local unpause-pipeline -p test
	fly -t local trigger-job -j test/test


# cluster - creates a kubernetes cluster using a modified image of `kind`,
# having the containerd configuration patched to have any insecure registries
# pullable.
#
cluster:
	kind create cluster \
		--image cirocosta/kind:modified-cri \
		--config hack/k8s/kind-config.yaml


# init - populates the `init` configmap with the binary used to hold our main
#        container up so that we can run processes in there.
#
init: $(INIT_BIN)
	$(KUBECTL) create namespace $(NAMESPACE) || true
	$(KUBECTL) create configmap --from-file $< \
		$(INIT_CONFIGMAP) || true


$(INIT_BIN): $(INIT_SRC)
	cd ./cmd/init && docker build -t init-img .
	docker create --name init init-img || true
	docker cp init:/init $@
	docker rm -f init


