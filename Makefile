.POSIX:
.SUFFIXES:
.SILENT: vet
	
BINARY=skewer
COMMIT=$(shell git rev-parse HEAD)
VERSION=0.1
LDFLAGS=-ldflags "-X github.com/stephane-martin/skewer/cmd.Version=${VERSION} -X github.com/stephane-martin/skewer/cmd.GitCommit=${COMMIT}"

SOURCES = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
SUBDIRS = $(shell find . -type d -regex './[a-z].*' -not -path './vendor*' -not -path '*.shapesdoc' | xargs)

$(BINARY): ${SOURCES} model/types_gen.go utils/logging/types_gen.go
	test -n "${GOPATH}"  # test $$GOPATH
	go build ${LDFLAGS} -o ${BINARY}

model/types_gen.go: model/types.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/model

utils/logging/types_gen.go: utils/logging/types.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/utils/logging

generate: model/types_gen.go utils/logging/types_gen.go

clean:
	rm -f ${BINARY} 

push: clean
	git commit && git push origin master

vet:
	test -n "${GOPATH}"  # test $$GOPATH
	go vet ./ ${SUBDIRS} || true

