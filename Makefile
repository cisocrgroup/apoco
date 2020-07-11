VERSION ?= "v0.0.1"

build:
	go build -ldflags "-X example.com/apoco/cmd/version.version=${VERSION}" .

install:
	go install .
