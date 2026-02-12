BINARY_NAME := policy-manager
# COMPOSE: compose command. Set to override; otherwise auto-detect podman-compose or docker-compose.
COMPOSE ?= $(shell command -v podman-compose >/dev/null 2>&1 && echo podman-compose || \
	(command -v docker-compose >/dev/null 2>&1 && echo docker-compose || \
	(echo "docker compose")))

build:
	go build -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

run:
	go run ./cmd/$(BINARY_NAME)

clean:
	rm -rf bin/

fmt:
	gofmt -s -w .

vet:
	go vet ./...

test:
	go run github.com/onsi/ginkgo/v2/ginkgo -r --randomize-all --fail-on-pending --skip-package=e2e

tidy:
	go mod tidy

generate-types:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=api/v1alpha1/types.gen.cfg \
		-o api/v1alpha1/types.gen.go \
		api/v1alpha1/openapi.yaml

generate-spec:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=api/v1alpha1/spec.gen.cfg \
		-o api/v1alpha1/spec.gen.go \
		api/v1alpha1/openapi.yaml

generate-server:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=internal/api/server/server.gen.cfg \
		-o internal/api/server/server.gen.go \
		api/v1alpha1/openapi.yaml

generate-client:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=pkg/client/client.gen.cfg \
		-o pkg/client/client.gen.go \
		api/v1alpha1/openapi.yaml

generate-crud-api: generate-types generate-spec generate-server generate-client

# Engine API code generation targets
generate-engine-types:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=api/v1alpha1/engine/types.gen.cfg \
		-o api/v1alpha1/engine/types.gen.go \
		api/v1alpha1/engine/openapi.yaml

generate-engine-spec:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=api/v1alpha1/engine/spec.gen.cfg \
		-o api/v1alpha1/engine/spec.gen.go \
		api/v1alpha1/engine/openapi.yaml

generate-engine-server:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=internal/api/engine/server.gen.cfg \
		-o internal/api/engine/server.gen.go \
		api/v1alpha1/engine/openapi.yaml

generate-engine-client:
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		--config=pkg/engineclient/client.gen.cfg \
		-o pkg/engineclient/client.gen.go \
		api/v1alpha1/engine/openapi.yaml

generate-engine-api: generate-engine-types generate-engine-spec generate-engine-server generate-engine-client

generate-api: generate-crud-api generate-engine-api

check-aep-engine:
	spectral lint --fail-severity=warn ./api/v1alpha1/engine/openapi.yaml

check-aep-api:
	spectral lint --fail-severity=warn ./api/v1alpha1/openapi.yaml
# Check AEP compliance
check-aep: check-aep-api check-aep-engine

# E2E test targets
test-e2e:
	go run github.com/onsi/ginkgo/v2/ginkgo -r --randomize-all --fail-on-pending -tags=e2e ./test/e2e

e2e-up:
	$(COMPOSE) up -d --build

e2e-down:
	$(COMPOSE) down -v

test-e2e-full: e2e-up test-e2e e2e-down

.PHONY: build run clean fmt vet test tidy generate-types generate-spec generate-server generate-client generate-api generate-crud-api generate-engine-types generate-engine-spec generate-engine-server generate-engine-client generate-engine-api check-generate-api check-aep test-e2e e2e-up e2e-down test-e2e-full
