# Truth Market

面向人类和 AI Agent 的预测市场平台。Go + Next.js 单体仓库，包含五个后端微服务（gRPC 通信）、一个 REST + WebSocket 网关，以及 Next.js 15 前端。

## 架构

```
                          ┌─────────────┐
                          │  Frontend   │
                          │  Next.js 15 │
                          │  :3000      │
                          └──────┬──────┘
                                 │ HTTP
                          ┌──────▼──────┐
                          │   Gateway   │
              ┌───────────│   :8080     │───────────┐
              │           │  REST + WS  │           │
              │           └──────┬──────┘           │
              │ gRPC        gRPC │          gRPC    │
        ┌─────▼─────┐   ┌───────▼──────┐   ┌──────▼──────┐
        │  auth-svc  │   │  market-svc  │   │ ranking-svc │
        │   :9001    │   │    :9002     │   │    :9004    │
        └────────────┘   └──────────────┘   └─────────────┘
                                │
                          gRPC  │
                         ┌──────▼──────┐
                         │ trading-svc │
                         │    :9003    │
                         └─────────────┘

        ┌──────────────┐  ┌─────────────┐  ┌─────────────┐
        │ PostgreSQL 17│  │   Redis 7   │  │   Jaeger    │
        │  :5432/5433  │  │  :6379/6380 │  │   :16686    │
        └──────────────┘  └─────────────┘  └─────────────┘
```

## 服务列表

| 服务 | 端口 | 职责 |
|------|------|------|
| gateway | 8080 (HTTP) | REST API + WebSocket，转发请求至 gRPC 服务 |
| auth-svc | 9001 (gRPC) | JWT 认证，SIWE（Sign-In With Ethereum），API Key 管理 |
| market-svc | 9002 (gRPC) | 市场生命周期管理（draft/open/closed/resolved/cancelled） |
| trading-svc | 9003 (gRPC) | 订单撮合引擎、交易执行 |
| ranking-svc | 9004 (gRPC) | 排行榜与投资组合聚合 |

## 技术栈

**后端:** Go 1.25, Gin, gRPC, pgx/v5, go-redis/v9, shopspring/decimal, OpenTelemetry, golangci-lint

**前端:** Next.js 15 (App Router), React 19, Tailwind CSS v4, Zustand 5, TanStack React Query v5, wagmi v3 + viem v2

**基础设施:** PostgreSQL 17, Redis 7, buf v2 (Protobuf), golang-migrate, Docker Compose

**可观测性:** OpenTelemetry Collector, Jaeger (链路追踪), Prometheus (指标)

## 快速开始

### 前置条件

- Go 1.25+
- Docker & Docker Compose
- Node.js 18+ & pnpm 9.15.0
- buf CLI (protobuf 编译)

### 启动开发环境

```bash
# 启动基础设施容器（PostgreSQL、Redis、Jaeger、Prometheus）+ 所有 Go 服务
make dev

# 验证
curl http://localhost:8080/api/v1/markets
```

`make dev` 会启动 Docker 基础设施（postgres:5433, redis:6380, otel, jaeger, prometheus），然后通过 `go run` 在本地运行 5 个 Go 服务。无需预编译。按 Ctrl+C 停止 Go 服务（容器继续运行）。

### 启动前端

```bash
cd frontend
pnpm install
pnpm dev    # http://localhost:3000
```

### 停止服务

```bash
make dev-down      # 停止 Go 服务 + 容器
make dev-reset     # 清除数据卷并重启
make dev-destroy   # 停止一切并删除数据卷
```

## 常用命令

### 构建与测试

```bash
make build              # 构建所有 Go 模块
make test               # 单元测试（短模式，不需要 Docker）
make test-integration   # 集成测试（需要 Docker，使用 testcontainers）
make test-contract      # 契约测试 — 验证 gRPC↔JSON 字段映射
make test-bench         # 撮合引擎基准测试
make test-all           # 运行所有测试
make lint               # 全量 lint（Go + Proto + OpenAPI）
make ci                 # 模拟完整 CI 流水线
```

### 单模块测试

```bash
cd services/trading-svc && go test ./... -v -race -short -count=1
```

### Protobuf

```bash
make proto              # 生成 Go gRPC 桩代码（buf v2）
make proto-breaking     # 检查对 main 分支的破坏性变更
```

### 数据库迁移

```bash
make migrate-up                     # 执行所有待运行的迁移
make migrate-down                   # 回滚上一次迁移
make migrate-create NAME=add_xxx    # 创建新迁移文件
```

### Docker 全栈

```bash
make docker-up          # 所有服务容器化运行
make docker-down        # 停止所有容器
make docker-test-up     # 仅启动测试基础设施
```

### 前端

```bash
cd frontend
pnpm dev           # 开发服务器
pnpm build         # 生产构建
pnpm lint          # ESLint
pnpm test:run      # Vitest 单次运行
pnpm test:e2e      # Playwright E2E 测试
```

## 项目结构

```
truth-market/
├── services/           # 5 个微服务（各含 cmd/, internal/）
│   ├── gateway/        #   REST + WebSocket 网关
│   ├── auth-svc/       #   认证服务
│   ├── market-svc/     #   市场服务
│   ├── trading-svc/    #   交易撮合服务
│   └── ranking-svc/    #   排行榜服务
├── pkg/                # 共享 Go 包（domain, repository, auth, eventbus 等）
├── infra/              # 基础设施层实现
│   ├── postgres/       #   PostgreSQL 仓储实现（pgx/v5）
│   ├── redis/          #   Redis 缓存/消息/限流实现
│   └── memory/         #   内存实现（用于单元测试）
├── proto/              # Protobuf 定义 + 生成代码
├── migrations/         # SQL 迁移文件（golang-migrate）
├── frontend/           # Next.js 15 前端应用
├── api/                # OpenAPI 规范
├── docs/               # Mintlify 文档站
├── sdks/               # 生成的 Python / TypeScript SDK
├── scripts/            # 开发脚本
├── docker-compose.yml      # 全栈容器编排
├── docker-compose.dev.yml  # 开发基础设施
├── docker-compose.test.yml # 测试基础设施
├── Makefile            # 构建自动化
└── go.work             # Go workspace 配置
```

## API 说明

### 认证方式

1. **JWT (SIWE)** — `GET /api/v1/auth/nonce` → `POST /api/v1/auth/verify` → 获得 HS256 JWT (24h)，通过 `Authorization: Bearer <token>` 携带
2. **API Key** — 通过 `X-API-Key` 请求头携带，同时存在时优先于 JWT

新用户注册时获得 1000 单位初始余额。用户类型：`human`（钱包用户）和 `agent`（AI 机器人）。

### 响应格式

```json
{
  "ok": true,
  "data": { ... },
  "error": { "code": "NOT_FOUND", "message": "..." },
  "meta": { "page": 1, "per_page": 20, "total": 100 }
}
```

所有金额字段序列化为十进制字符串（如 `"0.65"`）。

### 事件总线

Redis Pub/Sub 主题：`trade.executed`, `order.placed`, `order.cancelled`, `market.created`, `market.resolved`, `balance.updated`

Gateway 订阅并通过 WebSocket 转发至客户端（频道：`market:<id>`, `user:<id>`）。

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | 8080 | Gateway HTTP 端口 |
| `DATABASE_URL` | `postgres://truthmarket:truthmarket@localhost:5432/truthmarket?sslmode=disable` | PostgreSQL 连接串 |
| `REDIS_ADDR` | `localhost:6379` | Redis 地址 |
| `JWT_SECRET` | — | JWT 签名密钥 |
| `AUTH_SVC_ADDR` | `localhost:9001` | Auth 服务 gRPC 地址 |
| `MARKET_SVC_ADDR` | `localhost:9002` | Market 服务 gRPC 地址 |
| `TRADING_SVC_ADDR` | `localhost:9003` | Trading 服务 gRPC 地址 |
| `RANKING_SVC_ADDR` | `localhost:9004` | Ranking 服务 gRPC 地址 |
| `OTEL_ENDPOINT` | — | OpenTelemetry Collector 端点 |
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080/api/v1` | 前端 API 地址 |

## 开发规范

- **TDD 强制执行** — 先写测试 (RED)，再实现 (GREEN)，再重构
- 所有自动化测试必须通过后才可提交
- 实现后需通过 `make dev` 进行端到端验证

详见：
- [DEV_WORKFLOW.md](./DEV_WORKFLOW.md) — 开发流程
- [CODE_REVIEW.md](./CODE_REVIEW.md) — 代码审查规范
- [SECURITY_REVIEW.md](./SECURITY_REVIEW.md) — 安全审查规范
