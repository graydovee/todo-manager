# Stage 1: Build frontend
FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.26-alpine AS backend
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=frontend /app/frontend/dist ./static/frontend_dist/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /todo-manager cmd/server/main.go

# Stage 3: Final image
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /todo-manager /todo-manager
COPY config.example.yaml /config.yaml
EXPOSE 8080
ENTRYPOINT ["/todo-manager"]
CMD ["-config", "/config.yaml"]
