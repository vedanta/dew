.PHONY: build test lint fmt vet acceptance e2e check

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

# Full two-machine end-to-end test (builds its own binary).
e2e:
	./test/e2e.sh

# Mirror the CI gate locally.
check: fmt vet lint test
