# ctx

[![Go Version](https://img.shields.io/github/go-mod/go-version/ctx-hq/ctx)](https://go.dev/)
[![CI](https://github.com/ctx-hq/ctx/actions/workflows/ci.yml/badge.svg)](https://github.com/ctx-hq/ctx/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/ctx-hq/ctx)](https://github.com/ctx-hq/ctx/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

中文 | [English](README.md)

LLM 智能体技能、MCP 服务器和 CLI 工具的通用包管理器。

**ctx** 让你轻松发现、安装和管理用于扩展 AI 编程智能体（Claude Code、Cursor、Windsurf 等）的各类包。

## 安装

```bash
# macOS / Linux（一键安装，带 SHA256 校验）
curl -fsSL https://getctx.org/install.sh | sh

# Windows (PowerShell)
irm https://getctx.org/install.ps1 | iex

# macOS / Linux (Homebrew)
brew install ctx-hq/tap/ctx

# Go 用户
go install github.com/getctx/ctx/cmd/ctx@latest

# Debian / Ubuntu
dpkg -i ctx_*.deb

# Windows (Scoop)
scoop bucket add ctx https://github.com/ctx-hq/homebrew-tap
scoop install ctx
```

## 快速开始

```bash
# 搜索包
ctx search "code review"

# 安装技能
ctx install @hong/my-skill

# 安装 MCP 服务器
ctx install @mcp/github@2.1.0

# 安装 CLI 工具
ctx install @community/ripgrep

# 将所有包链接到智能体
ctx link claude
```

## 包类型

ctx 管理三种类型的包：

| 类型 | 说明 | 示例 |
|------|------|------|
| **skill** | 智能体技能与命令 | 代码审查提示词、重构工作流 |
| **mcp** | MCP（模型上下文协议）服务器 | GitHub、数据库、文件系统服务器 |
| **cli** | 命令行工具 | ripgrep、jq、fzf |

## 命令

### 发现与安装

```bash
ctx search <query>                  # 搜索注册表
ctx search "git" --type mcp         # 按类型筛选
ctx install <package[@version]>     # 安装包
ctx install github:user/repo        # 从 GitHub 直接安装
ctx info <package>                  # 查看包详情
ctx list                            # 列出已安装的包
ctx remove <package>                # 卸载包
```

### 更新

```bash
ctx update                          # 更新所有包
ctx update @hong/my-skill           # 更新指定包
ctx outdated                        # 检查可用更新
```

### 智能体链接

```bash
ctx link                            # 列出检测到的智能体
ctx link claude                     # 将包链接到 Claude Code
ctx link cursor                     # 将包链接到 Cursor
```

支持的智能体：`claude`、`cursor`、`windsurf`、`generic`

### 发布

```bash
ctx login                           # 通过 GitHub 认证
ctx init --type skill               # 生成 ctx.yaml 模板
ctx validate                        # 验证清单文件
ctx publish                         # 发布到注册表
```

### 组织管理

```bash
ctx org create <name>               # 创建组织
ctx org info <name>                 # 查看组织信息
ctx org add <org> <user>            # 添加成员
ctx org remove <org> <user>         # 移除成员
```

### 诊断

```bash
ctx version                         # 打印版本信息
ctx doctor                          # 诊断环境与连接状态
```

## MCP 服务器模式

ctx 可以作为 MCP 服务器运行，让 AI 智能体直接管理包：

```bash
ctx serve
```

在智能体的 MCP 配置中添加：

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

包通过 `ctx.yaml` 文件定义：

### 技能（Skill）

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

### MCP 服务器

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

### CLI 工具

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

## 配置

| 路径 | 用途 |
|------|------|
| `~/.ctx/config.yaml` | 注册表地址、认证令牌 |
| `~/.ctx/packages/` | 已安装的包 |
| `ctx.lock` | 锁定文件（项目级别） |

环境变量：

| 变量 | 说明 |
|------|------|
| `CTX_HOME` | 覆盖配置目录 |
| `CTX_DATA_HOME` | 覆盖数据目录 |
| `CTX_CACHE_HOME` | 覆盖缓存目录 |
| `CTX_REGISTRY` | 覆盖注册表地址 |

## 开发

```bash
make build          # 构建（版本号自动从 git 读取）
make test           # 运行测试
make test-race      # 运行测试（带竞态检测）
make lint           # 运行代码检查
make vet            # 运行 go vet
make check          # vet + lint + test 全套检查
make build-all      # 交叉编译所有平台
```

## 发版

通过 [release-please](https://github.com/googleapis/release-please) 自动管理版本。往 main 推送 Conventional Commits 格式的提交，会自动创建发版 PR。

手动发版：

```bash
# 预演（只检查，不打 tag）
scripts/release.sh v0.2.0 --dry-run

# 正式发版（7 项安全检查 → 打 tag → 推送）
scripts/release.sh v0.2.0
```

发版流水线（GoReleaser + GitHub Actions）自动完成：
- 交叉编译（Linux/macOS/Windows × AMD64/ARM64）
- Shell 自动补全（bash/zsh/fish）
- Cosign 签名 + SBOM 生成
- Homebrew formula、Scoop manifest、deb/rpm 包
- Build Provenance 供应链证明

## 许可证

[MIT](LICENSE)
