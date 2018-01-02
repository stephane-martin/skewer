.POSIX:
.SUFFIXES:
.SILENT: vet

SED=/usr/local/bin/gsed
BINARY=skewer
COMMIT=$(shell git rev-parse HEAD)
VERSION=0.1
LDFLAGS=-ldflags '-X github.com/stephane-martin/skewer/conf.Version=${VERSION} -X github.com/stephane-martin/skewer/conf.GitCommit=${COMMIT}"'

SOURCES = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
SUBDIRS = $(shell find . -type d -regex './[a-z].*' -not -path './vendor*' -not -path '*.shapesdoc' | xargs)

$(BINARY): ${SOURCES} utils/logging/types.pb.go conf/derived.gen.go utils/logging/derived.gen.go metrics/derived.gen.go consul/derived.gen.go sys/derived.gen.go sys/scomp/derived.gen.go sys/namespaces/derived.gen.go utils/queue/tcp/ring.go utils/queue/udp/ring.go utils/queue/kafka/ring.go utils/queue/message/ring.go model/types.pb.go model/types_ffjson.go 
	test -n "${GOPATH}"  # test $$GOPATH
	go build ${LDFLAGS} -o ${BINARY}

model/types_ffjson.go: model/types.go
	test -n "${GOPATH}"  # test $$GOPATH
	ffjson model/types.go

model/types.pb.go: model/types.proto
	test -n "${GOPATH}"  # test $$GOPATH
	protoc -I=. -I="$$GOPATH/src" -I="$$GOPATH/src/github.com/stephane-martin/skewer/vendor/github.com/gogo/protobuf/protobuf" --gogoslick_out=. model/types.proto

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

utils/logging/types.pb.go: utils/logging/types.proto
	test -n "${GOPATH}"  # test $$GOPATH
	protoc -I=. -I="$$GOPATH/src" -I="$$GOPATH/src/github.com/stephane-martin/skewer/vendor/github.com/gogo/protobuf/protobuf" --gogoslick_out=. utils/logging/types.proto

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

sys/scomp/derived.gen.go: sys/scomp/seccomp.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/sys/scomp
	gsed -i '1i// +build linux\n' sys/scomp/derived.gen.go

sys/namespaces/derived.gen.go: sys/namespaces/base.go
	test -n "${GOPATH}"  # test $$GOPATH
	go generate github.com/stephane-martin/skewer/sys/namespaces

generate: utils/logging/types_gen.go conf/derived.gen.go utils/logging/derived.gen.go metrics/derived.gen.go consul/derived.gen.go sys/derived.gen.go sys/scomp/derived.gen.go sys/namespaces/derived.gen.go utils/queue/tcp/ring.go utils/queue/udp/ring.go utils/queue/message/ring.go

clean:
	rm -f ${BINARY} 

push: clean
	git commit && git push origin master

vet:
	test -n "${GOPATH}"  # test $$GOPATH
	go vet ./... || true

