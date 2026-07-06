# Todo Manager

[English](README.md) | [简体中文](README.zh-CN.md)

一个可自托管的待办跟踪工具，适合个人和小团队 —— Go 后端 + React 前端，打包成单个二进制文件。跟踪 bug、feature 和 task，支持自动编号、标签、依赖关系图、评论，以及 AI 辅助摘要。

生产构建会把 React SPA 编译进 Go 二进制，整个应用作为一个可执行文件在 `:8080` 提供 HTTP 服务。

---

## 主要特性

- **类型化待办** —— 每个条目是 `bug`、`feature` 或 `task`，按用户自动编号为 `BUG-N` / `FEATURE-N` / `TASK-N`。
- **依赖与重复** —— 用 `depends_on` 把工作建模为 DAG，或标记为某个规范目标的 `duplicate_of`。完成或重开一个条目会沿依赖图级联。
- **依赖关系图** —— 交互式可视化（React Flow + ELK 布局）展示待办之间的关联。
- **评论** —— 给任意待办附加备注和进度更新。
- **置顶与高亮** —— 突出当前最重要的工作。
- **AI 摘要** —— 让 LLM 汇总某个待办（含评论与相关条目），并支持追问，结果通过 SSE 流式返回。
- **灵活认证** —— 内置表单登录，或接入 OIDC 提供商（Google、GitHub、Keycloak……）实现单点登录。
- **API 访问密钥** —— 为自动化和 CLI 签发带作用域的令牌（`todos:create`、`summaries:stream`……），与浏览器会话相互独立。
- **多数据库** —— SQLite 用于零配置本地运行，MySQL 或 PostgreSQL 用于多副本部署。
- **单二进制、单容器** —— 镜像无运行时依赖（distroless、无 CGO）。

## 技术栈

| 层级 | 选型 |
|------|------|
| 后端 | Go 1.25 · Echo v4 · GORM · SQLite / MySQL / PostgreSQL · coreos/go-oidc · openai-go |
| 前端 | Vite · React 19 · TypeScript · Ant Design v6 · TanStack Query · React Flow · i18next |
| 认证 | 表单登录（会话 Cookie）或 OIDC · CSRF 防护 · 带作用域的 API 访问密钥 |
| 构建 | Makefile · 多阶段 Dockerfile（distroless） · Helm chart |

## 快速开始（开发）

需要在本机 PATH 中安装好 Go、Node.js 和 npm。

```bash
# 1. 从模板生成本地配置（config.yaml 已被 gitignore）
cp config.example.yaml config.yaml

# 2. 启动后端（go run 热加载）—— 默认监听 :8080
make backend-dev

# 3. 在另一个终端启动 Vite 开发服务器 —— :5173
make frontend-dev
```

打开 <http://localhost:5173>。前端会把 API 请求代理到后端。示例配置中的默认基础认证用户为 `admin / admin123` 和 `user1 / user123`。

> 开发模式下使用两个进程：前端享受 Vite 的 HMR，Go 服务独立运行。生产环境中构建出的 SPA 由 Go 二进制直接托管（无需 Vite、无需 Node）。

## 生产构建

```bash
# 构建前端、嵌入 SPA，并生成服务端二进制 + CLI
make build

# 运行
make run          # ./bin/todo-manager -config config.yaml
```

产物在 `bin/` 目录下：

- `bin/todo-manager` —— 服务端（已内嵌 SPA）
- `bin/todo-cli` —— 命令行客户端

## Docker

```bash
# 本地构建镜像
make docker-build          # 打标签为 graydovee/todo-manager:<version> 和 :latest

# 使用内置默认配置运行
docker run --rm -p 8080:8080 graydovee/todo-manager:latest
```

镜像自带 `config.example.yaml` 作为默认的 `/config.yaml`。如需使用自定义配置，挂载即可：

```bash
docker run --rm -p 8080:8080 -v "$PWD/config.yaml:/config.yaml" graydovee/todo-manager:latest
```

使用 SQLite 时，把卷挂载到 `/data`（或 `db.dsn` 指向的位置）即可持久化。

多架构（amd64 + arm64）镜像通过 `make release` 发布。

## Kubernetes（Helm）

Helm chart 位于 [`charts/todo-manager/`](charts/todo-manager)，支持三种数据库模式：

| 模式 | 适用场景 |
|------|----------|
| `sqlite` | 单副本、简单部署。可选 PVC 做持久化。 |
| `bundled` | 随应用一起部署一个 PostgreSQL 子 chart。高可用的推荐默认值。 |
| `external` | 指向已有的 Postgres 或 MySQL 实例。 |

```bash
# 默认：SQLite、基础认证、单副本
helm install todo-manager ./charts/todo-manager

# 使用内置 PostgreSQL 并启用 ingress
helm install todo-manager ./charts/todo-manager \
  --set database.mode=bundled \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=todos.example.com
```

完整配置项（镜像、service、ingress、TLS、探针、资源、数据库凭据）见 [`charts/todo-manager/values.yaml`](charts/todo-manager/values.yaml)。

## 配置

所有运行时配置都在一个 YAML 文件中（默认 `config.yaml`，可用 `-config` 覆盖）。以 `TODO_MANAGER_` 为前缀的环境变量会覆盖对应的 YAML 键 —— 便于在容器中注入密钥而不落盘。

| YAML 路径 | 环境变量 |
|-----------|---------|
| `server.port` | `TODO_MANAGER_SERVER_PORT` |
| `db.driver` | `TODO_MANAGER_DB_DRIVER` |
| `db.dsn` | `TODO_MANAGER_DB_DSN` |
| `auth.mode` | `TODO_MANAGER_AUTH_MODE` |
| `session.secret` | `TODO_MANAGER_SESSION_SECRET` |
| `auth.oidc.client_secret` | `TODO_MANAGER_OIDC_CLIENT_SECRET` |
| `llm.model` / `llm.base_url` / `llm.api_key` / `llm.timeout` | `TODO_MANAGER_LLM_*` |
| `log.format` / `log.level` | `TODO_MANAGER_LOG_*` |

最小配置示例：

```yaml
server:
  port: 8080

db:
  driver: sqlite                       # sqlite | mysql | postgres
  dsn: "todo-manager.db"

auth:
  mode: basic                          # basic | oidc
  basic:
    users:
      - username: admin
        password: admin123
        display_name: Admin

session:
  secret: "change-me-in-production"    # 足够长的随机字符串
  max_age: 86400

llm:                                   # 可选 —— 驱动 AI 摘要
  model: "gpt-4o"
  base_url: "https://api.openai.com"
  api_key: "sk-your-api-key-here"
  timeout: 30

log:
  format: text                         # text | json
  level: info                          # debug | info | warn | error
```

完整 schema（含 OIDC 配置块）见 [`config.example.yaml`](config.example.yaml)。**部署前请务必修改所有默认密钥** —— 会话密钥、基础认证密码、LLM API 密钥。

## 项目结构

```
.
├── backend/                Go 服务（Echo + GORM）
│   ├── cmd/server/         入口 —— 加载配置、执行迁移、提供 HTTP 服务
│   └── internal/
│       ├── app/            路由装配、SPA 回退
│       ├── auth/           Basic + OIDC 认证提供商
│       ├── authz           权限与访问密钥作用域
│       ├── config/         YAML 配置 + TODO_MANAGER_* 环境变量覆盖
│       ├── database/       GORM + 按方言的迁移
│       ├── handler/        HTTP handler + DTO
│       ├── middleware/     认证、CSRF、CORS、权限校验
│       ├── model/          GORM 模型
│       ├── repository/     数据访问层
│       ├── service/        领域逻辑（编号、DAG、级联、摘要）
│       └── session/        基于数据库的会话存储
├── frontend/               Vite + React + TypeScript + Ant Design
├── todo-cli/               命令行客户端（对接 REST API）
├── skills/                 驱动 `todo-cli` 的 Claude Code skill
├── charts/todo-manager/    Helm chart
├── config.example.yaml     配置模板
├── Dockerfile              多阶段、distroless
└── Makefile                build / dev / test / docker / release 目标
```

## 命令行客户端（`todo-cli`）

`todo-cli` 是一个对接 REST API 的 Go 客户端 —— 既能独立使用，也是内置 Claude Code skill 的底座。执行 `make build`（或 `make cli-build`）后，登录并在终端里管理待办：

```bash
./bin/todo-cli login                          # 首次运行：创建名为 "default" 的用户
./bin/todo-cli login -u work --api-key ...    # 新增 / 更新另一个用户
./bin/todo-cli todos list --status open -u work
./bin/todo-cli todos create --category bug --title "定期轮换认证密钥"
./bin/todo-cli todos complete TASK-7
```

CLI 在 `~/.todo-manager/config.yaml` 中保存**多个用户配置**（名字可包含任意字符，因此使用列表而非 map）：

```yaml
auth:
  default_user: default
  users:
    - name: default
      base_url: https://todo.qaer.io
      api_key: tdk_...
    - name: work
      base_url: https://work.example.com
      api_key: tdk_...
```

- **`-u`/`--user`** 指定本次命令使用的用户（缺省时取 `auth.default_user`；若为空则必须传 `-u`）。
- **`config user`** 管理用户：`list`、`set-default <name>`（传 `""` 清空默认）、`remove <name>`、`rename <old> <new>`。
- **`--output`/`-o`** 选择输出格式：`yaml` 或 `json`（均为 pretty 打印）；`pretty` 作为 `json` 的兼容别名保留。`config view` 默认输出 YAML —— 机器解析请加 `-o json`。
- 旧版单用户配置（扁平的 `api_key`/`base_url`）会在首次运行时**自动迁移**为 `default` 用户。

运行 `todo-cli --help` 查看完整命令树。

## REST API

所有接口前缀为 `/api/v1`。资源路由同时接受浏览器会话和 API 访问密钥（`AuthEither` 中间件），每次调用都会按细粒度权限作用域校验。

| 模块 | 接口 |
|------|------|
| 认证 | `GET /auth/mode`、`GET /auth/csrf`、`POST /auth/login`、`GET /auth/login`（OIDC 跳转）、`GET /auth/callback`、`POST /auth/logout`、`GET /auth/me` |
| 访问密钥 | `GET/POST /access-keys`、`GET /access-keys/permissions`、`POST /access-keys/:id/rotate`、`DELETE /access-keys/:id` |
| 待办 | `GET /todos`、`GET /todos/graph`、`GET /todos/tags`、`GET /todos/by-date-range`、`GET/POST/PATCH/DELETE /todos[/:id]`、`POST /todos/:id/start\|complete\|reopen`、`PATCH /todos/:id/status\|pin\|highlight` |
| 评论 | `GET/POST /todos/:id/comments`、`DELETE /todos/:id/comments/:cid` |
| 摘要 | `POST /summaries`、`GET /summaries`、`GET /summaries/:id`、`GET /summaries/:id/stream`（SSE）、`DELETE /summaries/:id` |
| 追问 | `POST /summaries/:id/followup`、`GET /summaries/:id/followups` |
| 健康检查 | `GET /health` |

`GET /todos` 接受筛选/排序查询参数（状态、类别、标签、依赖、分页）。`GET /todos/graph` 返回用于依赖可视化的节点和边。

## 测试

```bash
make test        # 后端（go test）+ 前端（vitest）+ cli（go test）
make cli-test    # 仅 todo-cli
```

后端测试包含属性测试（基于 `pgregory.net/rapid`）以及摘要 handler 的集成测试。

## 领域规则（速查）

- **类别**创建后不可变：`bug`、`feature`、`task`。
- **编号**按用户自增：`BUG-N`、`FEATURE-N`、`TASK-N`。
- **标签**会去除首尾空格、转小写，并在单个待办内去重。
- **依赖**构成 DAG（`depends_on`）；创建环会被拒绝。
- **重复项**指向唯一规范目标（`duplicate_of`）。
- **完成**会沿 `depends_on` 级联并自动完成重复项；**重开**则反向操作。
- **删除**为硬删除：子项变为孤儿，关系被清理。

## 安全提示

- `config.yaml` 被刻意加入 gitignore —— 真实密钥都放在这里，请保持如此。提交进仓库的 [`config.example.yaml`](config.example.yaml) 只包含占位值。
- 在把应用暴露到本机以外之前，请轮换会话密钥、基础认证密码以及所有 API 密钥。
- API 访问密钥有作用域 —— 授予最小权限集合；自动化场景优先使用密钥，而非复用浏览器会话。
