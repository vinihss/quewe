SERVICE := quewe
MODULE  := github.com/vinihss/quewe/services/quewe

.PHONY: build run test test-race lint arch-check tidy build-cli

build:
	go build -o bin/$(SERVICE) ./cmd/$(SERVICE)

build-cli:
	go build -o bin/quewe-cli ./cmd/quewe-cli

run:
	go run ./cmd/$(SERVICE)

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	go vet ./...
	@gofmt -l . | (! grep .) || { echo "gofmt needed:"; gofmt -l .; exit 1; }

# Guide rule (section 13): executable fitness check for dependency rules.
# 2>/dev/null keeps minimal mode (without domain/application) silent.
arch-check:
	@if go list -deps ./internal/domain/... 2>/dev/null | grep "$(MODULE)/internal/" | grep -qv "$(MODULE)/internal/domain/"; then \
		echo "domain must not import other internal packages"; exit 1; \
	fi
	@if go list -deps ./internal/application/... 2>/dev/null | grep -q "$(MODULE)/internal/adapters"; then \
		echo "application must not import adapters"; exit 1; \
	fi
	@if go list -deps ./internal/application/... 2>/dev/null | grep -q "$(MODULE)/internal/infrastructure"; then \
		echo "application must not import infrastructure"; exit 1; \
	fi
	@if go list -deps ./internal/ports/... 2>/dev/null | grep -qE "$(MODULE)/internal/(application|adapters|infrastructure)"; then \
		echo "ports must depend only on domain and stdlib"; exit 1; \
	fi
	@echo "architecture OK"

tidy:
	go mod tidy
