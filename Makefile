.POSIX:
.SUFFIXES:
.SILENT: vet

SED=/usr/local/bin/gsed
BINARY=skewer
COMMIT=$(shell git rev-parse HEAD)
VERSION=0.1
LDFLAGS=-ldflags '-w -s -X github.com/stephane-martin/skewer/conf.Version=${VERSION} -X github.com/stephane-martin/skewer/conf.GitCommit=${COMMIT}"'

SOURCES = $(shell find . -type f -name '*.go' -not -path "./vendor/*")
SUBDIRS = $(shell find . -type d -regex './[a-z].*' -not -path './vendor*' -not -path '*.shapesdoc' | xargs)

$(BINARY): ${SOURCES} utils/logging/types.pb.go conf/derived.gen.go utils/queue/tcp/ring.go utils/queue/udp/ring.go utils/queue/kafka/ring.go utils/queue/message/ring.go model/types.pb.go model/types_ffjson.go utils/collectd/embed/statik/statik.go
	test -n "${GOPATH}"  # test $$GOPATH
	go build -o ${BINARY}

release: ${SOURCES} utils/logging/types.pb.go conf/derived.gen.go utils/queue/tcp/ring.go utils/queue/udp/ring.go utils/queue/kafka/ring.go utils/queue/message/ring.go model/types.pb.go model/types_ffjson.go utils/collectd/embed/statik/statik.go
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

utils/collectd/embed/statik/statik.go: utils/collectd/static/types.db
	test -n "${GOPATH}"  # test $$GOPATH
	statik -src ./utils/collectd/static -dest ./utils/collectd/embed

clean:
	rm -f ${BINARY} 

push: clean
	git commit && git push 

vet:
	test -n "${GOPATH}"  # test $$GOPATH
	go vet ./... || true

tools:
	test -n "${GOPATH}"  # test $$GOPATH
	go get -u github.com/spf13/cobra/cobra
	go get -u github.com/awalterschulze/goderive
	go get -u github.com/pquerna/ffjson
	go get -u github.com/hashrocket/ws
	go get -u github.com/kardianos/govendor
	go get -u github.com/cheekybits/genny
	go get -u github.com/gogo/protobuf/proto
	go get -u github.com/gogo/protobuf/gogoproto
	go get -u github.com/gogo/protobuf/protoc-gen-gofast
	go get -u github.com/gogo/protobuf/protoc-gen-gogofast
	go get -u github.com/gogo/protobuf/protoc-gen-gogofaster
	go get -u github.com/gogo/protobuf/protoc-gen-gogoslick
	go get -u github.com/rakyll/statik

