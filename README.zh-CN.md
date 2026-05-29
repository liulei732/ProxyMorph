# ProxyMorph

[English](README.md) | 中文

ProxyMorph 是一个自托管的代理订阅转换服务。目前第一期主要用于把 Clash 订阅转换成 Surge 6 配置，同时支持固定节点合并、自定义规则、自定义策略组，以及通过 sing-box 辅助转换部分 Surge 直连兼容性不好的协议节点。

## 功能特性

- 将 Clash 订阅链接转换成 Surge 6 订阅。
- 通过内置 Web 管理界面维护多个转换任务。
- 支持从代理 URI 或 Surge 节点行导入固定节点。
- 在统一节点模型中尽量保留 `ss`、`trojan`、`vmess`、`vless`、`anytls` 等协议信息。
- 保留 Trojan WebSocket 的 SNI、WebSocket Host、路径和 UDP 元数据。
- 支持自定义规则、自定义策略组、规则合并顺序和 FINAL 策略。
- 支持生成 MANAGED-CONFIG 头部，并在编辑时实时预览。
- 任务编辑时配置变更会实时刷新预览。
- VLESS 和 Trojan WebSocket 可分别配置 sing-box 辅助转换。
- 管理员登录、退出和修改密码。
- 使用 SQLite 存储数据，默认目录为 `/data`。
- Docker 部署，镜像内置前端资源和 `sing-box`。

## 项目结构

```text
cmd/proxymorph/      应用入口
internal/app/        HTTP 路由和应用组装
internal/auth/       管理员登录、会话和密码处理
internal/config/     环境变量配置
internal/convert/    Clash 解析、统一节点模型、Surge 渲染
internal/nodes/      固定节点导入和管理
internal/singbox/    sing-box 中继管理和配置生成
internal/storage/    SQLite 表结构、迁移和模型
internal/subscription/ 订阅生成、缓存和错误处理
internal/tasks/      转换任务服务和接口
internal/web/        嵌入式前端资源
web/                 React/Vite 管理界面
docs/                安全、排障和规划文档
```

## Docker Compose 快速开始

1. 修改 `docker-compose.yml`。
2. 修改管理员密码、会话密钥、中继密码、公开访问地址和中继公开主机名。
3. 启动服务：

```bash
docker compose up -d --build
```

打开 `http://localhost:28888`，使用配置的管理员账号登录。

默认 `docker-compose.yml` 使用 `network_mode: host`，适合 Linux 服务器。这样 `docker ps` 不会展示大量中继端口映射。如果在 Docker Desktop 上运行，需要把 host 网络改成显式端口映射，例如 `28888` 和 `31800-31999`。

## 部署注意事项

- 公网部署前请使用 HTTPS。
- 部署前必须修改 `PROXYMORPH_ADMIN_PASSWORD`、`PROXYMORPH_SESSION_SECRET` 和 `PROXYMORPH_VLESS_RELAY_PASSWORD`。
- `PROXYMORPH_PUBLIC_BASE_URL` 应设置成真实外部访问地址，例如 `https://proxy.example.com`。
- `PROXYMORPH_VLESS_RELAY_PUBLIC_HOST` 应设置成 Surge 客户端可以访问到的服务器地址。
- `/data` 必须持久化，它保存 SQLite 数据库、缓存输出和 sing-box 运行配置。
- 更新镜像后建议重新创建容器，不要只重启旧容器。

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PROXYMORPH_ADDR` | `:28888` | HTTP 监听地址。 |
| `PROXYMORPH_DATA_DIR` | `/data` | SQLite 数据库和运行数据目录。 |
| `PROXYMORPH_PUBLIC_BASE_URL` | 空 | 用于生成订阅链接的外部基础 URL。 |
| `PROXYMORPH_ADMIN_USERNAME` | `admin` | 初始管理员用户名。 |
| `PROXYMORPH_ADMIN_PASSWORD` | 空 | 初始管理员密码。首次启动为空时会使用 `admin`。 |
| `PROXYMORPH_SESSION_SECRET` | 随机开发密钥 | 管理端会话签名密钥。生产环境必须设置稳定强密钥。 |
| `PROXYMORPH_VLESS_RELAY_ENABLED` | `false` | 启用 sing-box 中继运行环境。具体节点是否使用中继仍由 UI 的全局/任务设置决定。 |
| `PROXYMORPH_VLESS_RELAY_PUBLIC_HOST` | 从公开 URL 推导 | 生成 SOCKS5 中继节点时使用的主机名。 |
| `PROXYMORPH_VLESS_RELAY_LISTEN_HOST` | `0.0.0.0` | sing-box SOCKS 入站监听地址。 |
| `PROXYMORPH_VLESS_RELAY_PORT_START` | `31800` | sing-box 中继端口起始值。 |
| `PROXYMORPH_VLESS_RELAY_PORT_END` | `31999` | sing-box 中继端口结束值。 |
| `PROXYMORPH_VLESS_RELAY_USERNAME` | `proxymorph` | 生成 SOCKS5 中继节点使用的用户名。 |
| `PROXYMORPH_VLESS_RELAY_PASSWORD` | 空 | 生成 SOCKS5 中继节点使用的密码。生产环境必须设置强密码。 |
| `PROXYMORPH_SING_BOX_PATH` | `sing-box` | sing-box 可执行文件路径。 |
| `PROXYMORPH_SING_BOX_CONFIG_PATH` | `/data/sing-box.json` | 生成的 sing-box 配置文件路径。 |

## 辅助转换说明

管理界面里有两个独立的辅助转换开关：

- VLESS 辅助转换：只影响 `vless` 节点。
- Trojan WebSocket 辅助转换：只影响 WebSocket 传输的 `trojan` 节点。

当某个任务实际启用了辅助转换时，生成的 Surge 节点会变成指向服务器的 `socks5` 中继节点，再由服务器上的 sing-box 连接原始上游节点。普通 Trojan 节点仍然保持 Surge 直出。

如果没有启用 Trojan WebSocket 辅助转换，生成结果继续是 `trojan, ... ws=true ...` 属于正常现象。已经排查过的 SNI、WebSocket Host、`udp-relay` 和辅助转换问题记录在 [排障文档](docs/troubleshooting.md)。

## 开发

后端测试：

```bash
go test ./...
```

前端开发：

```bash
cd web
npm install
npm run dev
```

本地构建生产二进制前，需要先构建并同步嵌入式前端资源：

```bash
cd web
npm run build
cd ..
rm -rf internal/web/dist
mkdir -p internal/web/dist
cp -R web/dist/. internal/web/dist/
go build ./cmd/proxymorph
```

提交前建议执行：

```bash
cd web && node --test src/*.test.ts
cd web && npm run build
go test ./...
```

## 相关文档

- [英文 README](README.md)
- [排障记录](docs/troubleshooting.md)
- [安全检查记录](docs/security-review.md)
- [规划约束](docs/planning-policy.md)

## 安全说明

ProxyMorph 面向可信管理员自托管使用。订阅链接包含类似 bearer token 的访问令牌，应当当作敏感信息处理。公开订阅接口不会返回详细内部错误，详细诊断只在登录后的管理界面展示。

准备公网发布前，请先阅读 [docs/security-review.md](docs/security-review.md)。
