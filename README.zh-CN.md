# ctx

[![Go Version](https://img.shields.io/github/go-mod/go-version/ctx-hq/ctx)](https://go.dev/)
[![CI](https://github.com/ctx-hq/ctx/actions/workflows/ci.yml/badge.svg)](https://github.com/ctx-hq/ctx/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ctx-hq/ctx)](https://github.com/ctx-hq/ctx/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

中文 | [English](README.md)

AI 编程智能体的通用包管理器。

```
ctx install @openelf/code-review   # 安装技能
ctx link claude                     # 链接到你的智能体 — 搞定
```

## 为什么用 ctx？

AI 编程智能体（Claude Code、Cursor、Windsurf 等）缺少统一的扩展发现和安装方式。每个智能体有自己的格式、配置目录和链接机制。

**ctx 解决了这个问题。** 一条命令安装，一条命令链接，支持 18 个智能体。

```
                 ┌─────────────────────────────┐
                 │        ctx registry          │
                 │   技能 · MCP · CLI 工具      │
                 └──────────┬──────────────────┘
                            │
                      ctx install
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
   ┌──────────┐      ┌──────────┐      ┌──────────┐
   │  Claude   │      │  Cursor  │      │ Windsurf │  ...
   │  Code     │      │          │      │          │
   └──────────┘      └──────────┘      └──────────┘
```

## 安装

```bash
# macOS / Linux
curl -fsSL https://getctx.org/install.sh | sh

# Homebrew
brew install ctx-hq/tap/ctx

# Go
go install github.com/ctx-hq/ctx/cmd/ctx@latest

# Windows
irm https://getctx.org/install.ps1 | iex
```

<details>
<summary>更多安装方式（Scoop、deb、rpm）</summary>

```bash
# Debian / Ubuntu
dpkg -i ctx_*.deb

# Windows (Scoop)
scoop bucket add ctx https://github.com/ctx-hq/homebrew-tap
scoop install ctx
```

</details>

## 快速开始

```bash
# 搜索包
ctx search "code review"

# 安装技能 — 自动检测智能体并链接
ctx install @openelf/code-review

# 安装 MCP 服务器（指定版本）
ctx install @mcp/github@2.1.0

# 安装 CLI 工具（自动委托给 brew/cargo/binary）
ctx install @community/ripgrep

# 查看已安装
ctx list

# 检查环境
ctx doctor
```

## 包类型

| 类型 | 说明 | 示例 |
|------|------|------|
| **skill** | 智能体的提示词和工作流 | 代码审查、重构、测试生成 |
| **mcp** | 模型上下文协议服务器 | GitHub、数据库、文件系统 |
| **cli** | 命令行工具 | ripgrep、jq、fzf |

## 支持的智能体

ctx 可以检测并链接 **18 个智能体**：

`claude` · `cursor` · `windsurf` · `opencode` · `codex` · `copilot` · `cline` · `continue` · `zed` · `roo` · `goose` · `amp` · `trae` · `kilo` · `pear` · `junie` · `aider` · `generic`

```bash
ctx link                  # 列出系统中检测到的智能体
ctx link claude           # 将所有包链接到 Claude Code
ctx link cursor           # 将所有包链接到 Cursor
```

## 命令

```bash
# 发现
ctx search <query>                  # 搜索注册表
ctx search "git" --type mcp        # 按类型筛选
ctx info <package>                  # 查看包详情

# 安装 / 卸载
ctx install <package[@version]>     # 安装
ctx install github:user/repo       # 从 GitHub 直接安装
ctx remove <package>                # 卸载
ctx list                            # 列出已安装

# 更新
ctx update                          # 更新所有
ctx update @scope/name              # 更新指定包
ctx outdated                        # 检查可用更新
ctx prune                           # 清理旧版本

# 智能体链接
ctx link                            # 列出检测到的智能体
ctx link <agent>                    # 链接包到智能体

# 发布
ctx login                           # 通过 GitHub 认证
ctx init                            # 交互式生成 ctx.yaml
ctx validate                        # 验证清单文件
ctx publish                         # 发布到注册表（公开）
ctx push                            # 推送为私有包（零摩擦）

# 组织管理
ctx org create <name>               # 创建组织
ctx org list                        # 列出所属组织
ctx org info <name>                 # 查看组织详情
ctx org packages <name>             # 列出组织的包
ctx org add <org> <user> [--role]   # 添加成员
ctx org remove <org> <user>         # 移除成员
ctx org delete <name>               # 删除组织（需 0 个包）

# 跨设备同步
ctx sync export                     # 导出安装状态到本地文件
ctx sync push                       # 上传同步配置到注册表
ctx sync pull                       # 从配置恢复所有包
ctx sync status                     # 查看同步状态和时间

# 分发标签
ctx dist-tag ls <package>           # 列出分发标签
ctx dist-tag add <pkg> <tag> <ver>  # 设置标签
ctx dist-tag rm <pkg> <tag>         # 移除标签

# 配置
ctx config list                     # 查看所有设置
ctx config set <key> <value>        # 修改设置
ctx config get <key>                # 读取设置

# 系统
ctx version                         # 版本信息
ctx doctor                          # 环境诊断
ctx upgrade                         # 自我更新
ctx serve                           # 作为 MCP 服务器运行
```

## MCP 服务器模式

ctx 可作为 MCP 服务器运行，让 AI 智能体直接管理包：

```json
{
  "mcpServers": {
    "ctx": {
      "command": "ctx",
      "args": ["serve"]
    }
  }
}
```

暴露的工具：`ctx_search`、`ctx_install`、`ctx_info`、`ctx_list`

## 包清单

包通过 `ctx.yaml` 文件定义。运行 `ctx init` 生成模板。

<details>
<summary>Skill 示例</summary>

```yaml
name: "@scope/my-skill"
version: "1.0.0"
type: skill
description: "面向 AI 智能体的代码审查技能"

skill:
  entry: SKILL.md
  tags: [review, code-quality]
  compatibility: "claude-code, cursor"
```

</details>

<details>
<summary>MCP 服务器示例</summary>

```yaml
name: "@scope/github-mcp"
version: "2.1.0"
type: mcp
description: "GitHub MCP 服务器"

mcp:
  transport: stdio
  command: npx
  args: ["-y", "@modelcontextprotocol/server-github"]
  env:
    - name: GITHUB_TOKEN
      required: true
      description: "GitHub 个人访问令牌"
```

</details>

<details>
<summary>CLI 工具示例</summary>

```yaml
name: "@community/ripgrep"
version: "14.1.0"
type: cli
description: "快速正则搜索工具"

cli:
  binary: rg
  verify: "rg --version"

install:
  brew: ripgrep
  cargo: ripgrep
  platforms:
    darwin-arm64:
      binary: "https://github.com/.../ripgrep-14.1.0-aarch64-apple-darwin.tar.gz"
    linux-amd64:
      binary: "https://github.com/.../ripgrep-14.1.0-x86_64-unknown-linux-musl.tar.gz"
```

</details>

## 配置

### 文件

| 路径 | 用途 |
|------|------|
| `~/.ctx/config.yaml` | 注册表地址、用户名、隐私设置 |
| `~/.ctx/packages/` | 已安装的包 |
| `~/.ctx/links.json` | 智能体链接记录 |
| 系统钥匙串 | 认证令牌（macOS Keychain / Linux secret-tool） |

### 设置项

```bash
ctx config set update_check false      # 禁用自动更新检查
ctx config set network_mode offline    # 禁用所有网络访问
ctx config set registry https://...    # 使用自定义注册表
```

### 环境变量

| 变量 | 说明 |
|------|------|
| `CTX_HOME` | 覆盖配置目录 |
| `CTX_DATA_HOME` | 覆盖数据目录 |
| `CTX_CACHE_HOME` | 覆盖缓存目录 |
| `CTX_REGISTRY` | 覆盖注册表地址 |

### 命令行标志

| 标志 | 说明 |
|------|------|
| `--offline` | 本次命令禁用网络访问 |
| `--json` | JSON 输出 |
| `--quiet` / `-q` | 精简输出 |
| `--yes` / `-y` | 跳过确认提示 |

## 隐私

- **无遥测。** ctx 不收集任何分析或使用数据。
- **令牌存储在系统钥匙串。** 认证令牌使用 macOS Keychain 或 Linux `secret-tool` 存储，不写入配置文件。
- **更新检查可关闭。** 通过 `ctx config set update_check false` 或 `--offline` 禁用。
- **文件权限 0600/0700。** 敏感文件仅所有者可读。
- **数据不离开本机**，除非你主动搜索、安装、发布或登录。

## 架构

```
cmd/ctx/              CLI 命令（Cobra）
internal/
  ├── config/         配置 + 文件权限
  ├── auth/           GitHub OAuth + 系统钥匙串
  ├── registry/       getctx.org API 客户端
  ├── resolver/       版本 + 来源解析
  ├── installer/      下载、解压、链接
  ├── adapter/        原生包管理器桥接（brew、npm、pip、cargo）
  ├── agent/          智能体检测 + 配置链接（18 个智能体）
  ├── mcpserver/      MCP 服务器模式（ctx serve）
  └── output/         人类 + JSON 输出格式化
```

## 参与贡献

```bash
git clone https://github.com/ctx-hq/ctx.git
cd ctx
make build      # 构建
make test       # 测试
make lint       # 代码检查
make check      # vet + lint + test 全套
```

### 发版流程

通过 [release-please](https://github.com/googleapis/release-please) 自动管理。往 `main` 推送 Conventional Commits 格式的提交，自动创建发版 PR。

流水线（GoReleaser + GitHub Actions）自动完成交叉编译（Linux/macOS/Windows x AMD64/ARM64）、Shell 补全、cosign 签名、SBOM、Homebrew/Scoop/deb/rpm 包和供应链证明。

## 许可证

[MIT](LICENSE)
