.PHONY: build test test-go test-wrappers clean

build:
	$(MAKE) -C exports build

test: test-go test-wrappers

test-go:
	go test

# Enable parallelism if -j is specified
test-wrappers: build
	$(MAKE) .test-csharp .test-python

.test-csharp:
	$(MAKE) -C wrappers/csharp test

.test-python:
	$(MAKE) -C wrappers/python test

clean:
	$(MAKE) -C exports clean
	$(MAKE) -C wrappers/csharp clean