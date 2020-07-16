VERSION ?= v0.0.1
LDFLAGS := "-X git.sr.ht/~flobar/apoco/cmd/version.version=${VERSION}"
build:
	go build -ldflags ${LDFLAGS}
install:
	go install -ldflags ${LDFLAGS} .
