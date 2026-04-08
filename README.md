<div align="center">
  <img src="./assets/logo.png" alt="VDD Logo" width="120" />

# VDD (Video Download Desktop)

基于 `yt-dlp` 的跨平台桌面视频下载工具（Go + Fyne）

<p>
  <a href="./LICENSE"><img alt="License" src="https://img.shields.io/badge/License-Apache--2.0-blue.svg"></a>
  <img alt="Go Version" src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go">
  <img alt="Platform" src="https://img.shields.io/badge/Platform-macOS%20%7C%20Windows%20%7C%20Linux-5c6ac4">
  <img alt="Status" src="https://img.shields.io/badge/Edition-Open%20Source-2ea043">
</p>

</div>

## 项目简介

VDD 是一个面向个人学习与研究场景的桌面下载器，支持多平台视频解析与下载，提供可视化任务管理、订阅工作流与常用网络配置能力。

当前仓库为 **开源版**，已移除激活/授权限制，功能默认可用。

## 特性亮点

- 智能推荐可下载格式
- 下载队列与并发调度
- 任务暂停、恢复、取消
- 订阅扫描与增量下载
- Cookie 与代理支持
- 自动更新检查（GitHub Releases）
- Windows / macOS / Linux 跨平台

## 快速开始

### 1. 克隆代码

```bash
git clone https://github.com/hankmor/vdd.git
cd vdd
```

### 2. 安装依赖

```bash
go mod download
```

### 3. 本地运行

```bash
go run main.go
```

## 构建

```bash
# 构建 GUI 主程序
go build -o vdd ./

# 构建 CLI 工具（可选）
go build -o vdd-cli ./cmd/cli
```

## 运行要求

运行 VDD 需要以下外部工具之一：

- 系统 `PATH` 可直接找到 `yt-dlp` 与 `ffmpeg`
- 或自行放置二进制到约定目录（见源码路径解析逻辑）

说明：当前开源仓库不再提交第三方可执行文件。

## 配置与数据目录

VDD 运行时会在用户目录创建配置与数据库，不会默认写入仓库目录。

- 配置/数据库基目录：`os.UserConfigDir()/VDD`
- 主数据库文件：`vdd.db`
- 日志目录：`os.UserConfigDir()/VDD/logs`

## 文档导航

- 用户手册: [`docs/product/USER_GUIDE.md`](./docs/product/USER_GUIDE.md)
- 构建说明: [`docs/development/BUILD.md`](./docs/development/BUILD.md)
- 发布流程: [`docs/development/RELEASE_GUIDE.md`](./docs/development/RELEASE_GUIDE.md)
- 架构设计: [`docs/development/DESIGN.md`](./docs/development/DESIGN.md)
- 开发任务: [`docs/development/TASK.md`](./docs/development/TASK.md)
- 贡献指南: [`CONTRIBUTING.md`](./CONTRIBUTING.md)
- 安全策略: [`SECURITY.md`](./SECURITY.md)

## 项目结构

```text
.
├── main.go
├── cmd/
│   └── cli/
├── core/
│   ├── config/
│   ├── db/
│   ├── download/
│   ├── parser/
│   ├── recommender/
│   ├── subscription/
│   └── tasks/
├── ui/
├── utils/
├── assets/
└── docs/
```

## 贡献

欢迎提交 Issue / PR。提交前建议执行：

```bash
go build ./...
go test ./...
```

说明：当前仓库中存在依赖外网资源的测试，用例在离线或网络受限环境可能失败。

## 合规与免责声明

本项目仅用于个人学习与研究。请遵守：

- 所在地区法律法规
- 目标网站服务条款
- 版权与内容使用规范

## License

本项目采用 [Apache License 2.0](./LICENSE)。

## 致谢

- [yt-dlp](https://github.com/yt-dlp/yt-dlp)
- [Fyne](https://fyne.io/)
- [FFmpeg](https://ffmpeg.org/)
