build:
	go build -ldflags ${LDFLAGS}
install:
	go install -ldflags ${LDFLAGS} .
