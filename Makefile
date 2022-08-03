bin_dir=$(shell pwd)/bin
service=directory
proto_plugins=bin/protoc-gen-openapi3
generated_files=directory-openapi.json
version=v1.0.0

bin/protoc-gen-openapi3: go.mod $(wildcard *.go) cmd/protoc-gen-openapi3/main.go
	GOBIN=$(bin_dir) go install ./cmd/protoc-gen-openapi3

.PHONY: generate
generate: $(service)-openapi.json

$(service)-openapi.json: rpc/$(service)/service.proto $(proto_plugins)
	PATH=$(bin_dir):$(PATH) protoc \
	--proto_path=. \
	--openapi3_out=. --openapi3_opt=application=$(service),version=$(version) \
	rpc/$(service)/service.proto
