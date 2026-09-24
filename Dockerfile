# 注意：不要加 `# syntax=docker/dockerfile:1` —— 那会让 BuildKit 去 Docker Hub 拉
# frontend 镜像（CI 构建环境到 registry-1.docker.io 不通）。下面用到的 RUN --mount
# 语法，Docker 28 内置 frontend 已直接支持，无需额外 frontend。
#
# 基础镜像一律写 Harbor（CI 的 DinD 只认 harbor.graydove.cn），由集群的 harbor
# 代理缓存回源上游。

# Stage 1: Build frontend
FROM --platform=$BUILDPLATFORM harbor.graydove.cn/library/node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
# npm 走国内源；缓存 mount 落在 runner 的 DinD 数据盘，跨构建复用
RUN --mount=type=cache,target=/root/.npm \
    npm config set registry https://registry.npmmirror.com && \
    npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build backend（交叉编译；前端产物经 go:embed 打进二进制）
FROM --platform=$BUILDPLATFORM harbor.graydove.cn/library/golang:1.27-alpine AS backend
WORKDIR /app/backend
ENV CGO_ENABLED=0 GOPROXY=https://goproxy.cn,direct
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY backend/ ./
COPY --from=frontend /app/frontend/dist ./static/frontend_dist/
ARG TARGETOS TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /todo-manager cmd/server/main.go

# Stage 3: Final image
# 原先用 gcr.io/distroless/static-debian12:nonroot，但 Harbor 里没有 distroless，
# 改用已有的 alpine 并显式装 ca-certificates（OIDC discovery 与 LLM API 都要走
# HTTPS）与 tzdata。uid 与原 distroless nonroot 一致，PVC /data 写权限不变。
FROM harbor.graydove.cn/library/alpine:3.22
RUN --mount=type=cache,target=/var/cache/apk \
    apk add --no-cache ca-certificates tzdata
COPY --from=backend /todo-manager /todo-manager
COPY config.example.yaml /config.yaml
USER 65534:65534
EXPOSE 8080
ENTRYPOINT ["/todo-manager"]
CMD ["-config", "/config.yaml"]
