# Agent 指南

## 协作规则

- **回答使用中文**；代码、注释、commit message、PR 描述等产物一律使用**英文**。
- **编译、测试必须通过 `make` 执行**。如果现有 Makefile 目标无法满足需求，**优先修改 Makefile**（新增或调整 target），而不是绕过 make 直接在终端拼命令。

## 仓库概览

Monorepo，四个子项目：

| 目录 | 说明 |
| --- | --- |
| `backend/` | Go 1.25 + Echo v4 + GORM；支持 SQLite / MySQL / PostgreSQL；生产构建将前端 SPA 嵌入单个二进制 |
| `frontend/` | Vite + React 19 + TypeScript + Ant Design v6 + TanStack Query 的 Web SPA |
| `desktop/` | Tauri 2 + React 桌面端（Windows 为主要目标），通过 HTTP API 与 backend 通信 |
| `todo-cli/` | Go CLI 客户端，通过 API access key 调用后端 |

其他：`charts/` 为 Helm chart；`Dockerfile` 为多阶段 distroless 镜像；本地配置由 `config.example.yaml` 复制为 `config.yaml`（已 gitignore）。

## 常用 Make 目标

- 开发：`make backend-dev`（:8080）、`make frontend-dev`（:5173，Vite 代理 API 到后端）、`make desktop-dev`
- 测试：`make test`（backend + frontend + cli 全量）、`make cli-test`
- 构建：`make build`（前端嵌入 + server + CLI → `bin/`）、`make frontend-build`、`make cli-build`
- 桌面打包：`make desktop-windows`（交叉编译，见下文）、`make desktop-build`（本机平台）
- 其他：`make run`、`make clean`、`make docker-build`、`make release`

前端 lint 暂无 Make 目标，如需请先在 Makefile 中添加（底层命令为 `cd frontend && npm run lint`）。

## 架构约定

- backend 分层：`internal/handler`（HTTP）→ `internal/service`（业务逻辑）→ `internal/repository`（持久化）；模型在 `internal/model`，认证授权在 `internal/auth`、`internal/authz`、`internal/session`。handler 不直接访问 repository。
- 生产模式下后端从 `backend/static/frontend_dist` 提供前端静态资源（由 `make frontend-build` 负责构建并拷贝）。
- 领域规则：todo 分 `bug` / `feature` / `task` 三类，按用户自动编号为 `BUG-N` / `FEATURE-N` / `TASK-N`；`depends_on` 构成 DAG，完成/重开会沿图级联。
- 更详细的领域与架构说明见 `README.md` / `README.zh-CN.md`。

---

## Desktop 打包并复制到 Windows 下载目录

更新 `desktop/` 后，按以下流程打包 Windows 版本并复制到用户下载目录。

### 1. 打包

在仓库根目录执行（从 Linux 交叉编译到 Windows MSVC 目标）：

```bash
make desktop-windows
```

该 target 会：先 `npm run build` 构建前端，再 `cargo xwin build --release --features prod --target x86_64-pc-windows-msvc`。
产出单个自包含 exe（WebView2Loader 静态链接，无需附带 DLL）。

产出路径：

```
desktop/src-tauri/target/x86_64-pc-windows-msvc/release/todo-desktop.exe
```

> 环境依赖：`cargo-xwin`、`llvm-15-tools`、`clang-15`、`lld-15`（详见 `README`）。
> release 全量编译较慢；增量编译会快很多。

### 2. 复制到下载目录（序号去重）

目标目录：`/mnt/c/Users/DuJiahui/Downloads/`（即 Windows 的 `C:\Users\DuJiahui\Downloads\`）。

**文件名规则**：基础名 `todo-desktop.exe`；若已存在同名文件，则在文件名后追加递增数字后缀，数字为已有最大序号 +1：

| 已有文件 | 新文件名 |
| --- | --- |
| 无 | `todo-desktop.exe` |
| `todo-desktop.exe` | `todo-desktop-1.exe` |
| `todo-desktop.exe`、`todo-desktop-1.exe`、`todo-desktop-2.exe`、`todo-desktop-3.exe` | `todo-desktop-4.exe` |

> 注意：序号是 `-N` 形式（`-1`、`-2`、`-3`…），`todo-desktop.exe` 视为序号 0。
> 选取下一个序号前，先 `ls` 目标目录确认当前最大序号，避免覆盖。

复制命令示例（假设下一个序号为 4）：

```bash
cp desktop/src-tauri/target/x86_64-pc-windows-msvc/release/todo-desktop.exe \
   /mnt/c/Users/DuJiahui/Downloads/todo-desktop-4.exe
```

### 3. 验证

复制后 `ls -l` 确认文件存在、大小合理（通常 ~15 MB）且时间戳为本次打包时间。

### 常见问题

- **`make desktop-windows` 失败**：确认工具链已安装（`cargo-xwin` 等）。首次需联网拉取 Windows SDK/MSVC 头文件缓存。
- **目标目录不可写**：WSL 未挂载 `/mnt/c` 或权限问题——需在 Windows 侧确认磁盘挂载。
