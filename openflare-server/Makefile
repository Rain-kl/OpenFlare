.PHONY: swagger license license-check build-embedded build-test cross-build code-check

VERSION ?= dev
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
MODULE := $(shell go list -m)

swagger:
	scripts/swagger.sh

license:
	scripts/update_go_license.sh

license-check:
	scripts/update_go_license.sh --check

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
		-o bin/wavelet \
		main.go

code-check:
	golangci-lint run
	cd frontend && pnpm tsc --noEmit --jsx preserve && npx eslint . --max-warnings 0

build-backend:
	@echo "==> Building backend version=$(VERSION) build_date=$(BUILD_DATE)..."
	go build \
		-ldflags "-s -w -X '$(MODULE)/internal/buildinfo.Version=$(VERSION)' -X '$(MODULE)/internal/buildinfo.BuildTime=$(BUILD_DATE)'" \
		-o bin/wavelet \
		main.go

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
