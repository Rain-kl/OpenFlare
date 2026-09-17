.PHONY: swagger license license-check format build-embedded build-test cross-build code-check build-backend build-frontend build-agent build-relay build-flared build-all

VERSION ?= dev
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
MODULE := $(shell cd backend && go list -m)

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
	goimports -w $$(find backend -type f -name '*.go')
	@echo "==> Formatting frontend source and removing unused imports..."
	cd frontend && pnpm format

build-embedded:
	@echo "==> Building embedded frontend version=$(VERSION) build_date=$(BUILD_DATE)..."
	cd frontend && \
		NEXT_PUBLIC_APP_VERSION="$(VERSION)" \
		NEXT_PUBLIC_APP_BUILD_DATE="$(BUILD_DATE)" \
		pnpm build:embed
	rm -rf backend/plugins/drivers/driver_http/dist
	cp -R frontend/out backend/plugins/drivers/driver_http/dist
	test -f backend/plugins/drivers/driver_http/dist/index.html
	cd backend && go build \
		-tags embed_frontend \
		-ldflags "-s -w -X '$(MODULE)/pkg/buildinfo.Version=$(VERSION)' -X '$(MODULE)/pkg/buildinfo.BuildTime=$(BUILD_DATE)'" \
		-o ../bin/openflare-server \
		main.go

code-check:
	@echo "==> Architecture guards..."
	scripts/check_cordis_architecture.sh
	@if rg -n 'db\.DB\(|db\.Redis' backend/openflare/plugins/server/kernel/model --glob '*.go' -g '!*_test.go' ; then \
		echo 'error: internal/model must not access db.DB or db.Redis (non-test code)' >&2; \
		exit 1; \
	fi
	cd backend && golangci-lint run
	cd frontend && node scripts/merge-i18n-fragments.mjs && pnpm tsc --noEmit --jsx preserve && npx eslint . --max-warnings 0

build-backend:
	@echo "==> Building backend version=$(VERSION) build_date=$(BUILD_DATE)..."
	cd backend && go build \
		-ldflags "-s -w -X '$(MODULE)/pkg/buildinfo.Version=$(VERSION)' -X '$(MODULE)/pkg/buildinfo.BuildTime=$(BUILD_DATE)'" \
		-o ../bin/openflare-server \
		main.go

build-agent:
	@echo "==> Building agent version=$(VERSION)..."
	cd backend && go build \
		-ldflags "-s -w -X '$(MODULE)/openflare/plugins/agent/config.Version=$(VERSION)'" \
		-o ../bin/openflare-agent \
		cmd/agent/main.go

build-relay:
	@echo "==> Building relay version=$(VERSION)..."
	cd backend && go build \
		-ldflags "-s -w -X '$(MODULE)/openflare/plugins/relay/config.Version=$(VERSION)'" \
		-o ../bin/openflare-relay \
		cmd/relay/main.go

build-flared:
	@echo "==> Building flared version=$(VERSION)..."
	cd backend && go build \
		-ldflags "-s -w -X '$(MODULE)/openflare/plugins/flared/config.Version=$(VERSION)'" \
		-o ../bin/flared \
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
	( cd backend && go test ./... && go build -o /dev/null ./... 2>&1 | sed 's/^/[backend]  /' ) & PIDS="$$PIDS $$!"; \
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
		--file manifest/docker/Dockerfile.cross \
		--target export \
		--build-arg VERSION=$(or $(VERSION),dev) \
		--build-arg BUILD_DATE="$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')" \
		$(if $(GOOS),--build-arg TARGET_OS=$(GOOS)) \
		$(if $(GOARCH),--build-arg TARGET_ARCH=$(GOARCH)) \
		--output type=local,dest=./bin \
		.
	@echo "==> Done. Binaries written to ./bin/"
	@ls -lh bin/

dev-f:
	@echo "==> Starting frontend development server..."
	cd frontend && pnpm dev

dev-b:
	@echo "==> Starting backend development server..."
	cd backend && go run main.go all

dev:
	@echo "==> Starting frontend and backend development servers in parallel..."
	@PIDS=""; \
	STATUS=0; \
	( cd frontend && pnpm dev 2>&1 | sed 's/^/[frontend] /' ) & PIDS="$$PIDS $$!"; \
	( cd backend && go run main.go all 2>&1 | sed 's/^/[backend]  /' ) & PIDS="$$PIDS $$!"; \
	for PID in $$PIDS; do \
		wait $$PID || STATUS=1; \
	done; \
	if [ $$STATUS -eq 0 ]; then \
		echo "==> All development servers exited successfully."; \
	else \
		echo "==> Development servers exited with errors." >&2; \
		exit 1; \
	fi


# Merge the current local branch into canary, push canary, then restore it.
# Dirty worktrees are auto-stashed and restored after the operation.
canary:
	@set -e; \
	if ! git rev-parse --git-dir >/dev/null 2>&1; then \
		echo "Error: not a git repository"; exit 1; \
	fi; \
	orig=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$orig" = "HEAD" ]; then \
		echo "Error: detached HEAD; checkout a branch first"; exit 1; \
	fi; \
	if [ "$$orig" = "canary" ]; then \
		echo "Error: already on canary; checkout a source branch first"; exit 1; \
	fi; \
	stashed=0; \
	if [ -n "$$(git status --porcelain)" ]; then \
		echo "Working tree is dirty; stashing local changes..."; \
		git stash push -u -m "make canary auto-stash from $$orig"; \
		stashed=1; \
	fi; \
	cleanup() { \
		cur=$$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true); \
		if [ "$$cur" != "$$orig" ]; then \
			echo "Restoring branch $$orig..."; \
			git checkout -q "$$orig"; \
		fi; \
		if [ "$$stashed" = "1" ]; then \
			echo "Restoring stashed local changes..."; \
			git stash pop; \
		fi; \
	}; \
	trap cleanup EXIT; \
	echo "Merging $$orig -> canary..."; \
	git checkout canary; \
	git merge --no-edit "$$orig"; \
	git push -u origin canary; \
	echo "Done: $$orig merged into canary and pushed."
