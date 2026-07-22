.PHONY: swagger license license-check format build-embedded build-test cross-build code-check build-backend build-frontend build-agent build-relay build-flared build-all

VERSION ?= dev
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
MODULE := $(shell go list -m)

swagger:
	scripts/swagger.sh

license:
	scripts/update_go_license.sh

license-check:
	scripts/update_go_license.sh --check

format:
	@echo "==> Formatting backend Go source and removing unused imports..."
	@command -v goimports >/dev/null 2>&1 || { \
		echo "goimports not found, installing..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
	}
	goimports -w $$(find . -type f -name '*.go' -not -path './.git/*' -not -path './frontend/*')
	@echo "==> Formatting frontend source and removing unused imports..."
	cd frontend && pnpm format

build-embedded:
	@echo "==> Building embedded frontend version=$(VERSION) build_date=$(BUILD_DATE)..."
	cd frontend && \
		NEXT_PUBLIC_APP_VERSION="$(VERSION)" \
		NEXT_PUBLIC_APP_BUILD_DATE="$(BUILD_DATE)" \
		pnpm build:embed
	rm -rf internal/router/root/dist
	cp -R frontend/out internal/router/root/dist
	go build \
		-tags embed_frontend \
		-ldflags "-s -w -X '$(MODULE)/internal/buildinfo.Version=$(VERSION)' -X '$(MODULE)/internal/buildinfo.BuildTime=$(BUILD_DATE)'" \
		-o bin/openflare-server \
		main.go

code-check:
	golangci-lint run
	cd frontend && pnpm tsc --noEmit --jsx preserve && npx eslint . --max-warnings 0

build-backend:
	@echo "==> Building backend version=$(VERSION) build_date=$(BUILD_DATE)..."
	go build \
		-ldflags "-s -w -X '$(MODULE)/internal/buildinfo.Version=$(VERSION)' -X '$(MODULE)/internal/buildinfo.BuildTime=$(BUILD_DATE)'" \
		-o bin/openflare-server \
		main.go

build-agent:
	@echo "==> Building agent version=$(VERSION)..."
	go build \
		-ldflags "-s -w -X '$(MODULE)/internal/apps/agent/config.Version=$(VERSION)'" \
		-o bin/openflare-agent \
		cmd/agent/main.go

build-relay:
	@echo "==> Building relay version=$(VERSION)..."
	go build \
		-ldflags "-s -w -X '$(MODULE)/internal/apps/relay/config.Version=$(VERSION)'" \
		-o bin/openflare-relay \
		cmd/relay/main.go

build-flared:
	@echo "==> Building flared version=$(VERSION)..."
	go build \
		-ldflags "-s -w -X '$(MODULE)/internal/apps/flared/config.Version=$(VERSION)'" \
		-o bin/flared \
		cmd/flared/main.go

build-all: build-backend build-agent build-relay build-flared

build-frontend:
	@echo "==> Building frontend version=$(VERSION) build_date=$(BUILD_DATE)..."
	cd frontend && \
		NEXT_PUBLIC_APP_VERSION="$(VERSION)" \
		NEXT_PUBLIC_APP_BUILD_DATE="$(BUILD_DATE)" \
		pnpm build:embed

build-test:
	@echo "==> Running frontend and backend build tests in parallel..."
	@PIDS=""; \
	STATUS=0; \
	( cd frontend && pnpm build:embed 2>&1 | sed 's/^/[frontend] /' ) & PIDS="$$PIDS $$!"; \
	( go test ./... && go build -o /dev/null ./... 2>&1 | sed 's/^/[backend]  /' ) & PIDS="$$PIDS $$!"; \
	for PID in $$PIDS; do \
		wait $$PID || STATUS=1; \
	done; \
	if [ $$STATUS -eq 0 ]; then \
		echo "==> All build tests passed."; \
	else \
		echo "==> Build test FAILED." >&2; \
		exit 1; \
	fi

cross-build:
	@echo "==> Cross-compiling \
	$(if $(GOOS),$(GOOS),linux/darwin/windows) × \
	$(if $(GOARCH),$(GOARCH),amd64/arm64) \
	(version=$(or $(VERSION),dev))..."
	@mkdir -p bin
	docker build \
		--file docker/Dockerfile.cross \
		--target export \
		--build-arg VERSION=$(or $(VERSION),dev) \
		--build-arg BUILD_DATE="$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')" \
		$(if $(GOOS),--build-arg TARGET_OS=$(GOOS)) \
		$(if $(GOARCH),--build-arg TARGET_ARCH=$(GOARCH)) \
		--output type=local,dest=./bin \
		.
	@echo "==> Done. Binaries written to ./bin/"
	@ls -lh bin/
