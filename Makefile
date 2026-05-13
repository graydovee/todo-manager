.PHONY: frontend-dev backend-dev frontend-build test build run docker-build clean

frontend-dev:
	cd frontend && npm run dev -- --port 5173

backend-dev:
	cd backend && go run cmd/server/main.go -config ../config.yaml

frontend-build:
	cd frontend && npm ci && npm run build
	rm -rf backend/static/frontend_dist
	cp -r frontend/dist backend/static/frontend_dist

test:
	cd backend && go test ./...
	cd frontend && npm run test -- --passWithNoTests

build: frontend-build
	cd backend && CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/todolist cmd/server/main.go

docker-build:
	docker build -t graydovee/todolist .

run: build
	cd backend && ./bin/todolist -config ../config.yaml

clean:
	rm -rf frontend/dist backend/static/frontend_dist backend/bin
	find . -name "*.db" -not -path "./.git/*" -delete
