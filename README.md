# Twirp OpenAPI 3 Docs

Generates OpenAPI 3 documentation from Twirp protobuf declarations.

# Usage

Makefile example

```
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
```

When used in another package installation can be handled like this:

```
bin/protoc-gen-twirp: go.mod
	GOBIN=$(bin_dir) go install github.com/twitchtv/twirp/protoc-gen-twirp
```

...if you have specified twopdocs as a "tools.go" dependency:

```
//go:build tools
// +build tools

package yourpackage

import (
	_ "github.com/navigacontentlab/twopdocs"
)
```
