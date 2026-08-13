.PHONY: build build-controller build-cli test generate controller-gen

build: build-controller build-cli

build-controller:
	CGO_ENABLED=0 go build -o bin/agentorgs-controller ./cmd/controller/

build-cli:
	CGO_ENABLED=0 go build -o bin/ago ./cmd/ago/

test:
	go test ./...

GOBIN ?= $(shell go env GOPATH)/bin
CONTROLLER_GEN ?= $(GOBIN)/controller-gen

controller-gen:
	@test -x $(CONTROLLER_GEN) || go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

generate: controller-gen
	$(CONTROLLER_GEN) object paths=./api/...
	$(CONTROLLER_GEN) crd paths=./api/... output:crd:artifacts:config=../config/crd
