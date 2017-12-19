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

$(BINARY): ${SOURCES} model/types_gen.go utils/logging/types_gen.go conf/derived.gen.go utils/logging/derived.gen.go metrics/derived.gen.go consul/derived.gen.go sys/derived.gen.go model/derived.gen.go model/types_ffjson.go sys/scomp/derived.gen.go sys/namespaces/derived.gen.go utils/queue/tcp/ring.go utils/queue/udp/ring.go utils/queue/kafka/ring.go utils/queue/message/ring.go 
	test -n "${GOPATH}"  # test $$GOPATH
	go build ${LDFLAGS} -o ${BINARY}

utils/queue/kafka/ring.go: utils/queue/ring.go
	test -n "${GOPATH}"  # test $$GOPATH
	genny -in=utils/queue/ring.go -out=utils/queue/kafka/ring.go -pkg=kafka gen Data=model.RawKafkaMessage

utils/queue/message/ring.go: utils/queue/ring.go
	test -n "${GOPATH}"  # test $$GOPATH
	genny -in=utils/queue/ring.go -out=utils/queue/message/ring.go -pkg=message gen Data=model.FullMessage

utils/queue/tcp/ring.go: utils/queue/ring.go
	test -n "${GOPATH}"  # test $$GOPATH
	genny -in=utils/queue/ring.go -out=utils/queue/tcp/ring.go -pkg=tcp gen Data=model.RawTcpMessage

utils/queue/udp/ring.go: utils/queue/ring.go
	test -n "${GOPATH}"  # test $$GOPATH
	genny -in=utils/queue/ring.go -out=utils/queue/udp/ring.go -pkg=udp gen Data=model.RawUdpMessage

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

generate: model/types_gen.go utils/logging/types_gen.go conf/derived.gen.go utils/logging/derived.gen.go metrics/derived.gen.go consul/derived.gen.go sys/derived.gen.go model/derived.gen.go model/types_ffjson.go sys/scomp/derived.gen.go sys/namespaces/derived.gen.go utils/queue/tcp/ring.go utils/queue/udp/ring.go utils/queue/message/ring.go

clean:
	rm -f ${BINARY} 

push: clean
	git commit && git push origin master

vet:
	test -n "${GOPATH}"  # test $$GOPATH
	go vet ./... || true

