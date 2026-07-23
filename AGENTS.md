# Agent 指南

本仓库为 monorepo（Go 后端 + React 前端 + Tauri 桌面端）。通用架构与领域规则见 `CLAUDE.md`。

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
