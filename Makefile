.PHONY: build test lint fmt vet acceptance check

BIN := dew

build:
	go build -o $(BIN) .

test:
	go test -race -shuffle=on ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

lint:
	golangci-lint run

acceptance: build
	@if [ -x test/acceptance/run.sh ]; then PATH="$(PWD):$$PATH" ./test/acceptance/run.sh; \
	else echo "test/acceptance/run.sh not present yet (Phase 0.3)"; fi

# Mirror the CI gate locally.
check: fmt vet lint test
