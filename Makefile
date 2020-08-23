include settings.sh

# Current Operator version
#VERSION ?= 0.0.1
# Default bundle image tag
#BUNDLE_IMG ?= controller-bundle:$(VERSION)
BUNDLE_IMG ?= quay.io/$(USERNAME)/$(OPERATOR_NAME)-bundle:v$(VERSION)
FROM_BUNDLE_IMG ?= quay.io/$(USERNAME)/$(OPERATOR_NAME)-bundle:v$(FROM_VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Bundle Index tag
BUNDLE_INDEX_IMG ?= quay.io/$(USERNAME)/$(OPERATOR_NAME)-index:v$(VERSION)
FROM_BUNDLE_INDEX_IMG ?= quay.io/$(USERNAME)/$(OPERATOR_NAME)-index:v$(FROM_VERSION)

# Image URL to use all building/pushing image targets
#IMG ?= controller:latest
IMG ?= quay.io/$(USERNAME)/$(OPERATOR_IMAGE):v$(VERSION)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Prepare Test Env: https://sdk.operatorframework.io/docs/golang/references/env-test-setup/
# Setup binaries required to run the tests
# See that it expects the Kubernetes and ETCD version
K8S_VERSION = v1.18.2
ETCD_VERSION = v3.4.3
testbin:
	curl -sSLo setup_envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/kubebuilder/master/scripts/setup_envtest_bins.sh 
	chmod +x setup_envtest.sh
	./setup_envtest.sh $(K8S_VERSION) $(ETCD_VERSION)

# Run tests
test: generate fmt vet manifests testbin
    TESTBIN_DIR=$(pwd)/testbin TEST_ASSET_KUBECTL=${TESTBIN_DIR}/kubectl && \
	TEST_ASSET_KUBE_APISERVER=${TESTBIN_DIR}/kube-apiserver && \
	TEST_ASSET_ETCD=${TESTBIN_DIR}/etcd && \
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	export DB_SCRIPTS_BASE_DIR=. && go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# Undeploy controller in the configured Kubernetes cluster in ~/.kube/config
undeploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# Run the container using docker
# Create ./run/ca.crt ./run/server-ca.crt and ./run/token 
docker-run:
	docker run -it --rm --entrypoint /bin/bash -v $(shell pwd)/run:/var/run/secrets/kubernetes.io/serviceaccount \
	  -e KUBERNETES_SERVICE_PORT_HTTPS=6443 \
	  -e KUBERNETES_SERVICE_PORT=6443 \
	  -e KUBERNETES_SERVICE_HOST=api.cluster-644b.644b.example.opentlc.com ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

# Push the bundle image.
bundle-push: bundle-build
	docker push $(BUNDLE_IMG)

# Do all the bundle stuff
bundle-validate: bundle-push
	operator-sdk bundle validate $(BUNDLE_IMG)

# Do all bundle stuff
bundle-all: bundle-build bundle-push bundle-validate

# Bundle Index
# Build bundle by referring to the previous version if FROM_VERSION is defined
index-build:
ifneq (,$(shell go env FROM_VERSION))
	echo "FROM_VERSION ${FROM_VERSION}"
	opm -u docker index add --bundles $(BUNDLE_IMG) --tag $(BUNDLE_INDEX_IMG)
else
	opm -u docker index add --bundles $(BUNDLE_IMG) --from-index $(FROM_BUNDLE_INDEX_IMG) --tag $(BUNDLE_INDEX_IMG)
endif
	
# Push the index
index-push: index-build
	docker push $(BUNDLE_INDEX_IMG)

# [DEBUGGING] Export the index (pulls image) to download folder
index-export:
	opm index export --index="$(BUNDLE_INDEX_IMG)" --package="$(OPERATOR_NAME)"

# [DEBUGGING] Create a test sqlite db and serves it
index-registry-serve:
	opm registry add -b $(FROM_BUNDLE_IMG) -d "test-registry.db"
	opm registry add -b $(BUNDLE_IMG) -d "test-registry.db"
	opm registry serve -d "test-registry.db" -p 50051

# Catalog
catalog-deploy:
	sed "s|BUNDLE_INDEX_IMG|$(BUNDLE_INDEX_IMG)|" ./config/catalog/catalog-source.yaml | kubectl apply -f -
catalog-undeploy:
	sed "s|BUNDLE_INDEX_IMG|$(BUNDLE_INDEX_IMG)|" ./config/catalog/catalog-source.yaml | kubectl delete -f -

# [DEMO] Deploy previous index to then upgrade!
catalog-deploy-prev:
	sed "s|BUNDLE_INDEX_IMG|$(FROM_BUNDLE_INDEX_IMG)|" ./config/catalog/catalog-source.yaml | kubectl apply -f -
catalog-undeploy-prev:
	sed "s|BUNDLE_INDEX_IMG|$(FROM_BUNDLE_INDEX_IMG)|" ./config/catalog/catalog-source.yaml | kubectl delete -f -

# Ingress on minikube
ingress-minikube-deploy:
	minikube addons enable ingress

ingress-minikube-undeploy:
	minikube addons disable ingress

# Operator Lifecycle Manager OLM
olm-deploy:
	operator-sdk olm install

olm-undeploy:
	operator-sdk olm uninstall