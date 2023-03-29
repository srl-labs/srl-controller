# KNE_REF is a git reference to use for KNE. It can be a branch, tag, or commit hash.
KNE_REF ?= v0.1.9
KNE_REPO := https://github.com/openconfig/kne.git
KNE_TEMP_DIR := /tmp/.srlcontroller-tests/kne
KNE_TEST_DEPLOYMENT_FILE := ${KNE_TEMP_DIR}/deploy/kne/kind-bridge-no-controllers.yaml
KIND_CLUSTER_NAME ?= srl-test
SRL_IMAGE ?= ghcr.io/nokia/srlinux:latest

.PHONY: install-kne
install-kne: ## Install KNE
	rm -rf ${KNE_TEMP_DIR} && \
	mkdir -p ${KNE_TEMP_DIR} && \
	git clone ${KNE_REPO} ${KNE_TEMP_DIR} && \
	cd ${KNE_TEMP_DIR} && git checkout ${KNE_REF} && \
	make install

.PHONY: kne-test-deployment-cfg-file
kne-test-deployment-cfg-file: ## Create a KNE test deployment config file without controllers section
	sed -n '/controllers:/q;p' < ${KNE_TEMP_DIR}/deploy/kne/kind-bridge.yaml > ${KNE_TEST_DEPLOYMENT_FILE}
	sed -i 's/name: kne/name: ${KIND_CLUSTER_NAME}/g' ${KNE_TEST_DEPLOYMENT_FILE}

.PHONY: deploy-kne
deploy-kne: ## Deploy KNE kind cluster
	kind delete cluster --name=${KIND_CLUSTER_NAME} && \
	cd ${KNE_TEMP_DIR} && kne deploy ${KNE_TEST_DEPLOYMENT_FILE}

.PHONY: temp-docker-build
temp-docker-build: ## Built controller container using the image and tag specified in the manager's manifest
	image="$(shell docker run -i --rm mikefarah/yq '.images[0].newName' < config/manager/kustomization.yaml)" tag="$(shell docker run -i --rm mikefarah/yq '.images[0].newTag' < config/manager/kustomization.yaml)" && \
	docker build -t $$image:$$tag . && \
	kind load docker-image $$image:$$tag --name ${KIND_CLUSTER_NAME}

.PHONY: install-srl-controller
install-srl-controller: ## Install srl-controller from current working dir
	kubectl apply -k config/default
	@echo "wait for controller manager to be ready"
	kubectl -n srlinux-controller wait --for=condition=Available deployment.apps/srlinux-controller-controller-manager

.PHONY: uninstall-srl-controller
uninstall-srl-controller: ## Uninstall srl-controller from current working dir
	kubectl delete -k config/default

.PHONY: kind-load-image
kind-load-image:  ## Load SR Linux container image to kind cluster
	docker pull ${SRL_IMAGE}
	kind load docker-image ${SRL_IMAGE} --name ${KIND_CLUSTER_NAME}

.PHONY: start-kne-cluster
start-kne-cluster: install-kne kne-test-deployment-cfg-file deploy-kne kind-load-image ## Deploy KNE kind cluster but do not install any controllers

.PHONY: prepare-e2e-env
prepare-e2e-env: install-kne kne-test-deployment-cfg-file deploy-kne temp-docker-build install-srl-controller kind-load-image ## Install srl-controller from current working dir

.PHONY: test-e2e
test-e2e: ## Test e2e using kind and a provided test name
	go test -timeout 5m -v github.com/srl-labs/srl-controller/tests/e2e -run ${E2E_TEST_NAME}
