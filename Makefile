.POSIX:
.SUFFIXES:
.SILENT: vet

SED=/usr/local/bin/gsed
BINARY=skewer
COMMIT=$(shell git rev-parse HEAD)
VERSION=0.1
LDFLAGS=-ldflags '-X github.com/stephane-martin/skewer/cmd.Version=${VERSION} -X github.com/stephane-martin/skewer/cmd.GitCommit=${COMMIT}"'

SOURCES = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
SUBDIRS = $(shell find . -type d -regex './[a-z].*' -not -path './vendor*' -not -path '*.shapesdoc' | xargs)

$(BINARY): ${SOURCES} model/types_gen.go utils/logging/types_gen.go conf/derived.gen.go utils/logging/derived.gen.go metrics/derived.gen.go consul/derived.gen.go sys/derived.gen.go model/derived.gen.go model/types_ffjson.go sys/scomp/derived.gen.go sys/namespaces/derived.gen.go
	test -n "${GOPATH}"  # test $$GOPATH
	go build ${LDFLAGS} -o ${BINARY}

model/types_gen.go: model/types.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/model

utils/logging/types_gen.go: utils/logging/types.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/utils/logging

conf/derived.gen.go: conf/types.go conf/conf.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/conf

utils/logging/derived.gen.go: utils/logging/receiver.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/utils/logging

metrics/derived.gen.go: metrics/metrics.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/metrics

consul/derived.gen.go: consul/dynamicconf.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/consul

sys/derived.gen.go: sys/process.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/sys

model/derived.gen.go: model/rfc5424_format.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/model

model/types_ffjson.go: model/types.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/model

sys/scomp/derived.gen.go: sys/scomp/seccomp.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/sys/scomp
	gsed -i '1i// +build linux\n' sys/scomp/derived.gen.go

sys/namespaces/derived.gen.go: sys/namespaces/base.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/sys/namespaces

generate: model/types_gen.go utils/logging/types_gen.go conf/derived.gen.go utils/logging/derived.gen.go metrics/derived.gen.go consul/derived.gen.go sys/derived.gen.go model/derived.gen.go model/types_ffjson.go sys/scomp/derived.gen.go sys/namespaces/derived.gen.go

clean:
	rm -f ${BINARY} 

push: clean
	git commit && git push origin master

vet:
	test -n "${GOPATH}"  # test $$GOPATH
	go vet ./ ${SUBDIRS} || true

