export CGO_ENABLED=0

.PHONY: build test test-v test-tool lint fuzz cross check clean

# Build the binary. CGO_ENABLED=0 is mandatory - static binary promise depends on it.
build:
	go build -o vrk .

# Run all tests. Use before every commit.
test:
	go test ./... -timeout 30s

# Verbose tests. Use when debugging a specific failure.
test-v:
	go test ./... -v -timeout 30s

# One tool only: make test-tool TOOL=jwt
test-tool:
	go test ./cmd/$(TOOL)/... -v -timeout 30s

# Run the linter. Fix all warnings before committing.
lint:
	golangci-lint run ./...

# Fuzz targets - 60s each. Run before v1 release.
fuzz:
	go test -fuzz=FuzzJwt   -fuzztime=60s ./cmd/jwt/
	go test -fuzz=FuzzEpoch -fuzztime=60s ./cmd/epoch/
	go test -fuzz=FuzzTok   -fuzztime=60s ./cmd/tok/
	go test -fuzz=FuzzSse   -fuzztime=60s ./cmd/sse/

# Verify cross-compilation works. Run after every Claude Code session.
# If this fails, CGO crept in. Most common cause: mattn/go-sqlite3.
cross:
	@GOOS=linux  GOARCH=amd64 go build -o /dev/null . && echo "ok  linux/amd64"
	@GOOS=linux  GOARCH=arm64 go build -o /dev/null . && echo "ok  linux/arm64"
	@GOOS=darwin GOARCH=amd64 go build -o /dev/null . && echo "ok  darwin/amd64"
	@GOOS=darwin GOARCH=arm64 go build -o /dev/null . && echo "ok  darwin/arm64"

# Full pre-commit check. Run before every commit. Takes ~30 seconds.
check: build test lint cross
	@echo ""
	@echo "all checks passed"

# Remove build artifacts
clean:
	rm -f vrk
