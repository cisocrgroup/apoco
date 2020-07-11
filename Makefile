VERSION ?= "v0.0.1"
LDFLAGS := "-X example.com/apoco/cmd/version.version=${VERSION}"
build:
	go build -ldflags ${LDFLAGS}
install:
	go install -ldflags ${LDFLAGS} .
