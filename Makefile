.POSIX:
.SUFFIXES:

BINARY=skewer
COMMIT=$(shell git rev-parse HEAD)
VERSION=0.1
LDFLAGS=-ldflags "-X github.com/stephane-martin/skewer/cmd.Version=${VERSION} -X github.com/stephane-martin/skewer/cmd.GitCommit=${COMMIT}"

SOURCES := $(shell find . -name '*.go')

$(BINARY): $(SOURCES) model/types_gen.go utils/logging/types_gen.go
	go build ${LDFLAGS} -o ${BINARY}

model/types_gen.go: model/types.go
	go generate github.com/stephane-martin/skewer/model

utils/logging/types_gen.go: utils/logging/types.go
	go generate github.com/stephane-martin/skewer/utils/logging

.PHONY: generate
generate: model/types_gen.go utils/logging/types_gen.go


