.PHONY: build build-controller build-cli build-agent-openclaw test generate controller-gen local-k8s-up local-k8s-down demo-apply e2e-matrix

CONTROLLER_DIR := agentorgs-controller
OPENCLAW_AGENT_IMAGE ?= agentorgs/agent-openclaw:local
OPENCLAW_BASE_IMAGE ?= ghcr.io/openclaw/openclaw:latest

build: build-controller build-cli

build-controller:
	cd $(CONTROLLER_DIR) && CGO_ENABLED=0 go build -o ../bin/agentorgs-controller ./cmd/controller/

build-cli:
	cd $(CONTROLLER_DIR) && CGO_ENABLED=0 go build -o ../bin/ago ./cmd/ago/

# OpenClaw runtime image: official OpenClaw base + MinIO workspace sync glue.
build-agent-openclaw:
	docker build -t $(OPENCLAW_AGENT_IMAGE) \
		-f agent/openclaw/Dockerfile \
		--build-arg BASE_IMAGE=$(OPENCLAW_BASE_IMAGE) \
		agent/openclaw/

test:
	cd $(CONTROLLER_DIR) && go test ./...

GOBIN ?= $(shell go env GOPATH)/bin
CONTROLLER_GEN ?= $(GOBIN)/controller-gen

controller-gen:
	@test -x $(CONTROLLER_GEN) || go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

generate: controller-gen
	cd $(CONTROLLER_DIR) && $(CONTROLLER_GEN) object paths=./api/...
	cd $(CONTROLLER_DIR) && $(CONTROLLER_GEN) crd paths=./api/... output:crd:artifacts:config=../config/crd

local-k8s-up:
	AGENTORGS_LLM_API_KEY="$(AGENTORGS_LLM_API_KEY)" bash hack/local-k8s-up.sh

local-k8s-down:
	bash hack/local-k8s-down.sh

demo-apply:
	kubectl apply -f config/samples/demo.yaml

# Kind e2e: shell bootstraps cluster; Go asserts workspace + Matrix reply.
# mock-llm is e2e-only under tests/e2e/.
e2e-matrix:
	bash tests/e2e/run.sh
