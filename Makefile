.PHONY: default di clean build

default: di build;

di:
	wire ./cmd/master
	wire ./cmd/worker

clean:
	rm -rf ./out

build:
	go build -o out/master ./cmd/master
	go build -o out/worker ./cmd/worker

install:
	go install ./cmd/master
	go install ./cmd/worker

static:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o ./out/master ./cmd/master
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o ./out/worker ./cmd/worker
