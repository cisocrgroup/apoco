GO ?= go

build:
	${GO} build

test:
	${GO} test -cover ./...

vet:
	${GO} vet ./...

install:
	${GO} install

clean:
	${RM} apoco

.PHONY: build test vet install clean
