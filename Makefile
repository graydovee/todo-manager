.PHONY: frontend-dev backend-dev frontend-build cli-build cli-test desktop-build desktop-windows desktop-run test build run docker-build release clean

frontend-dev:
	cd frontend && npm run dev -- --port 5173

backend-dev:
	cd backend && go run cmd/server/main.go -config ../config.yaml

frontend-build:
	cd frontend && npm install && npm run build
	rm -rf backend/static/frontend_dist
	cp -r frontend/dist backend/static/frontend_dist

cli-build:
	mkdir -p bin
	cd todo-cli && go build -o ../bin/todo-cli .

cli-test:
	cd todo-cli && go test ./...

# Desktop GUI client (Gio). Windows is the primary target.
desktop-build:
	mkdir -p bin
	cd desktop && go build -o ../bin/todo-desktop .

# Cross-compile a Windows binary with no console window.
desktop-windows:
	mkdir -p bin
	cd desktop && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -H windowsgui" -o ../bin/todo-desktop.exe .

desktop-run:
	cd desktop && go run .

test:
	cd backend && go test ./...
	cd frontend && npm run test -- --passWithNoTests
	cd todo-cli && go test ./...

build: frontend-build cli-build
	mkdir -p bin
	cd backend && CGO_ENABLED=0 go build -ldflags="-s -w" -o ../bin/todo-manager cmd/server/main.go

IMAGE_NAME := graydovee/todo-manager
GIT_TAG := $(shell git describe --tags --exact-match 2>/dev/null)
GIT_VERSION := $(if $(GIT_TAG),$(GIT_TAG),$(shell git describe --tags --abbrev=7 2>/dev/null || echo dev))
CONTAINER_ENGINE := $(shell command -v docker 2>/dev/null || command -v podman 2>/dev/null)

docker-build:
	$(CONTAINER_ENGINE) build --platform linux/amd64 -t $(IMAGE_NAME):$(GIT_VERSION) -t $(IMAGE_NAME):latest .

PLATFORMS := linux/amd64,linux/arm64

release:
	$(CONTAINER_ENGINE) manifest rm $(IMAGE_NAME):$(GIT_VERSION) 2>/dev/null || true
	$(CONTAINER_ENGINE) manifest rm $(IMAGE_NAME):latest 2>/dev/null || true
	$(CONTAINER_ENGINE) build --platform $(PLATFORMS) --manifest $(IMAGE_NAME):$(GIT_VERSION) .
	$(CONTAINER_ENGINE) manifest push $(IMAGE_NAME):$(GIT_VERSION) docker://docker.io/$(IMAGE_NAME):$(GIT_VERSION)
	$(CONTAINER_ENGINE) manifest push $(IMAGE_NAME):$(GIT_VERSION) docker://docker.io/$(IMAGE_NAME):latest

run:
	./bin/todo-manager -config config.yaml

clean:
	rm -rf frontend/dist backend/static/frontend_dist bin
	find . -name "*.db" -not -path "./.git/*" -delete
