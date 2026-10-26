# BelieveBoy 小程序方案查看系统设计

- 日期：2026-10-26
- 状态：已确认，待实施
- 首发形态：微信小程序 + H5 查看器 + Next.js 管理后台 + Go API

## 1. 背景与目标

系统用于把管理员上传的单个自包含 HTML 方案安全地分发给客户查看。

核心目标：

- 管理员上传单个 `.html` 文件，系统生成不可变内容版本。
- 同一份 HTML 可以创建公共分享或专属分享。
- 客户无需注册账号，通过分享入口和 4 位密码查看内容。
- 每次分享可以独立设置有效期、水印文字，并可随时撤销。
- 同一内容同时支持微信小程序和普通浏览器 H5 访问。
- 查看页适配手机、平板和桌面宽屏。
- HTML 中允许本地 JavaScript 交互，但默认禁止外部网络请求和高风险浏览器能力。
- 管理后台使用 Next.js，业务 API、鉴权和权限规则统一由 Go 实现。

## 2. 非目标

首发版本不做以下内容：

- 不建立客户账号、客户资料或客户关系管理。
- 不记录可关联到真实客户身份的 User ID。
- 不提供员工管理页面；数据库和 API 预留 `ADMIN`、`STAFF` 角色。
- 不接受 ZIP、多文件站点或外部资源包，只接受单个自包含 HTML。
- 不允许 HTML 主动访问任意外部 API、CDN 或表单接口。
- 不承诺阻止截图、录屏、打印或人工复制。
- 不提供搜索引擎公开访问，也不做内容 SEO。
- 不在首发版本实现微信 URL Link、小程序码批量生成或微信 `openid` 登录。
- 不把密码校验、有效期判断或内容授权放在前端。

## 3. 关键决策

| 领域 | 决策 |
| --- | --- |
| 内容格式 | 单个自包含 HTML，允许内联 CSS、JavaScript、图片和字体数据 |
| 小程序 | 微信原生小程序，TypeScript |
| H5 | React + Vite + TypeScript |
| 管理后台 | Next.js App Router + TypeScript |
| 业务后端 | Go |
| 数据库 | PostgreSQL |
| 对象存储 | S3 兼容接口；开发使用 MinIO |
| 管理后台边界 | 只做界面，所有业务规则和鉴权在 Go |
| 客户身份 | 无客户账号；分享记录和短时会话就是访问主体 |
| 分享类型 | `PUBLIC` 群分享和 `PRIVATE` 专属分享 |
| 密码 | 每个分享 4 位数字密码，服务端保存 Argon2id 摘要 |
| 默认有效期 | 必填，管理端默认 7 天 |
| 默认会话 | 30 分钟 |
| WebView 入场票据 | 一次性，60 秒有效 |
| 内容地址 | 短时签名地址，默认 10 分钟有效 |
| 上传上限 | 20 MiB |

## 4. 总体架构

系统按来源和安全边界拆分为四个站点：

- 管理端：`ADMIN_ORIGIN`，由 Next.js 提供页面。
- H5 查看器：`VIEW_ORIGIN`，由 React + Vite 构建静态资源。
- 内容域：`CONTENT_ORIGIN`，只提供隔离的 HTML 内容。
- API 域：`API_ORIGIN`，供微信小程序直接请求。

本地开发使用以下逻辑域名：

- `admin.localhost`
- `view.localhost`
- `content.localhost`
- `api.localhost`

生产域名在部署配置中设置，但必须保持四个来源相互独立。`CONTENT_ORIGIN` 不得与管理端、H5 查看器共享 Cookie，也不得承载管理后台。

组件关系：

```text
微信小程序 ───────────────> API_ORIGIN
     │
     └── WebView ─────────> VIEW_ORIGIN ──> API（同域 /api）──> Go
                                  │
                                  └── sandbox iframe ───────> CONTENT_ORIGIN

Next.js 管理后台 ─────────> ADMIN_ORIGIN/api ─────────────────> Go

Go ─────────> PostgreSQL
Go ─────────> S3 / MinIO
```

路由规则：

- `ADMIN_ORIGIN/api/*` 由 Caddy 转发到 Go，其余请求交给 Next.js。
- `VIEW_ORIGIN/api/*` 和 `VIEW_ORIGIN/enter` 由 Caddy 转发到 Go，其余请求交给 H5 静态站点。
- `CONTENT_ORIGIN/c/*` 由 Caddy 转发到 Go 的内容处理器。
- `API_ORIGIN/*` 由 Caddy 转发到 Go。

管理端和 H5 的 API 与页面保持同源，因此会话 Cookie 不需要跨站发送。客户端代码不得直接持有 Go 内部服务凭据。

## 5. 组件职责

### 5.1 管理后台

- 管理员登录、退出和会话续期。
- 上传、查看、预览和归档 HTML 内容版本。
- 创建公共分享和专属分享。
- 设置 4 位密码、过期时间、分享标签和水印文字。
- 复制分享链接、查看访问审计、撤销分享、重置密码。
- 展示上传和解析错误。
- 不实现内容授权、密码校验、文件解析等业务规则。

### 5.2 H5 查看器

- 解析 `/s/{shareToken}`。
- 展示密码输入、错误、限流、过期、撤销和会话失效页面。
- 密码校验成功后建立 30 分钟查看器会话。
- 请求短时内容地址，并在 sandbox iframe 中展示 HTML。
- 每 45 秒检查一次会话状态；失效后移除 iframe。
- 在 iframe 上方显示不可交互水印。
- 响应式适配 320px 到宽屏，内容区域最大宽度按视口动态分配。

### 5.3 微信小程序

- 从分享卡片页面参数读取 `shareToken`。
- 展示分享状态和原生密码输入页。
- 调用 Go 解锁接口并获得一次性 WebView 入场票据。
- 打开 `VIEW_ORIGIN/enter?ticket=...`。
- 展示网络错误、票据失效和无法打开 WebView 的恢复操作。
- 不保存明文密码，不读取 HTML 内容。

小程序 WebView 需要满足微信平台要求：小程序类目可用、`VIEW_ORIGIN` 完成 HTTPS、ICP 备案并加入业务域名白名单。

### 5.4 Go API

- 管理端和员工 RBAC。
- HTML 上传、校验、解析、版本化和对象存储。
- 分享令牌、密码摘要、有效期和撤销状态。
- 密码限流、查看器会话、一次性票据和内容签名。
- 审计日志、请求 ID、健康检查和指标。
- 所有业务数据只能通过 Go 读写，Next.js 不直接连接 PostgreSQL 或 MinIO。

## 6. 数据模型

所有主键使用 UUID。时间统一以 UTC 存储，管理端按 `Asia/Shanghai` 显示和输入。

### 6.1 `admin_users`

- `id`
- `email`
- `credential_hash`：Argon2id
- `role`：`ADMIN` 或 `STAFF`
- `status`：`ACTIVE` 或 `DISABLED`
- `created_at`
- `updated_at`

首发只提供管理员界面。初始管理员通过 Go 命令行工具创建，不设置默认密码。

### 6.2 `admin_sessions`

- `id`
- `admin_user_id`
- `session_hash`
- `expires_at`
- `last_seen_at`
- `revoked_at`
- `ip_hash`
- `user_agent_hash`

### 6.3 `contents`

- `id`
- `title`
- `status`：`ACTIVE` 或 `ARCHIVED`
- `created_by`
- `created_at`
- `updated_at`

### 6.4 `content_versions`

- `id`
- `content_id`
- `version_no`
- `object_key`
- `sha256`
- `byte_size`
- `content_title`
- `analysis_status`：`PASSED` 或 `REJECTED`
- `analysis_notes`
- `created_by`
- `created_at`

内容版本创建后不可修改。相同 SHA-256 可以复用对象存储中的文件，但仍然创建独立版本记录。

### 6.5 `shares`

- `id`
- `content_version_id`
- `type`：`PUBLIC` 或 `PRIVATE`
- `label`
- `token_hash`
- `token_ciphertext`
- `pin_hash`
- `watermark_text`
- `expires_at`
- `revoked_at`
- `session_ttl_seconds`
- `created_by`
- `created_at`
- `updated_at`

`token_hash` 用于查找，`token_ciphertext` 使用服务端 AES-GCM 密钥加密，使管理员后续仍可复制现有链接。明文密码不保存，只能重置。

### 6.6 `access_sessions`

- `id`
- `share_id`
- `session_hash`
- `expires_at`
- `last_seen_at`
- `revoked_at`
- `ip_hash`
- `user_agent_hash`

### 6.7 `viewer_tickets`

- `id`
- `access_session_id`
- `ticket_hash`
- `expires_at`
- `consumed_at`

Ticket 只能消费一次，默认 60 秒失效。

### 6.8 `audit_events`

- `id`
- `actor_type`：`ADMIN`、`VIEWER` 或 `SYSTEM`
- `actor_id`
- `share_id`
- `event_type`
- `request_id`
- `ip_hash`
- `user_agent_hash`
- `metadata_json`
- `created_at`

审计事件至少覆盖上传、预览、分享创建、密码成功、密码失败、限流、票据消费、内容签发、撤销和过期访问。

## 7. HTML 上传与解析

上传流程：

1. 管理端以 `multipart/form-data` 上传单个文件。
2. Go 限制请求体为 20 MiB。
3. 仅接受扩展名 `.html`，并校验实际内容可解析为 HTML。
4. 使用 `golang.org/x/net/html` 提取 `<title>`、检查视口声明和扫描外部资源。
5. 计算 SHA-256，上传到对象存储。
6. 创建不可变 `content_versions` 记录。
7. 返回解析结果和管理端预览地址。

校验规则：

- 缺失 `<title>` 时使用原文件名作为标题，并在解析结果中提示。
- 缺失 viewport 声明时允许上传，但管理端提示宽屏和移动端适配风险。
- `src`、`href`、`srcset` 和 CSS `url()` 中出现的 `http://`、`https://`、协议相对地址视为外部资源。
- 外部资源不允许发布；管理端必须修正后重新上传。
- HTML 可以包含内联 `<script>`、内联样式、`data:` 和 `blob:` 资源。
- 上传失败不能留下半成品版本或孤立对象。

默认大小和扫描规则由配置控制，但生产默认值不得高于本设计中的 20 MiB。

## 8. 分享与访问流程

### 8.1 公共分享

- 管理员为某个内容版本创建一条 `PUBLIC` 分享。
- 所有收到链接的人使用同一个 4 位密码。
- 修改密码、有效期、水印或撤销分享会同时影响所有访问者。
- 公共分享的 `label` 和水印应避免写入客户个人身份信息。

### 8.2 专属分享

- 管理员为每个客户单独创建一条 `PRIVATE` 分享。
- 每条分享拥有独立令牌、密码、有效期、水印和撤销状态。
- 系统不要求客户账号，不保存“客户”实体。
- 专属分享的标签可以由管理员填写，但不得作为客户身份认证依据。

### 8.3 H5 访问

1. 浏览器打开 `VIEW_ORIGIN/s/{shareToken}`。
2. Go 返回最小分享状态，不返回内容地址。
3. 用户输入 4 位密码。
4. Go 校验令牌、限流、密码、有效期和撤销状态。
5. 成功后设置 HttpOnly、Secure、SameSite=Lax 查看器会话 Cookie。
6. 查看器请求短时内容地址并在 iframe 中加载。

### 8.4 小程序访问

1. 用户打开分享卡片，页面参数包含 `shareToken`。
2. 小程序请求分享状态，展示原生 4 位密码输入。
3. Go 校验成功后返回一次性 `viewerTicket` 和查看器地址。
4. 小程序打开 `VIEW_ORIGIN/enter?ticket=...`。
5. Go 消费票据、设置查看器会话 Cookie，并重定向到 `/s/{shareToken}`。
6. 后续 HTML 展示和安全控制与 H5 完全一致。

### 8.5 会话与撤销

- 查看器会话默认 30 分钟，不提供无限续期。
- 会话有效期不得超过分享有效期。
- 每次内容签发都重新检查分享撤销状态和有效期。
- 查看器每 45 秒请求一次状态；发现失效时移除 iframe 并显示原因。
- 管理员撤销分享或重置密码时，同时撤销该分享的全部 `access_sessions`。

## 9. 密码与限流

- 密码必须是 4 位数字。
- 密码使用 Argon2id 保存，带独立随机盐。
- 限流在密码摘要计算前执行，避免高成本哈希被滥用。
- 首发为单实例部署，限流状态保存在 Go 内存中，并通过接口抽象隔离。
- 默认规则：同一“分享 + IP”10 分钟内最多 5 次失败，随后冷却 15 分钟。
- 默认规则：同一分享 10 分钟内累计 100 次失败时，短暂冷却 5 分钟。
- 成功解锁清除当前 IP 的失败计数。
- 返回 `429` 和 `Retry-After`，不透露密码是否正确以外的内部细节。

高熵分享令牌是防止密码被枚举的主要边界，4 位密码只是第二道门槛。

## 10. 内容隔离与安全

### 10.1 内容域

- 内容只通过 `CONTENT_ORIGIN/c/{signedToken}` 提供。
- 签名令牌默认 10 分钟有效，绑定分享、版本和会话。
- 内容域不设置任何登录 Cookie。
- 内容响应使用 `Cache-Control: private, no-store`。
- 内容响应使用 `X-Content-Type-Options: nosniff` 和 `Referrer-Policy: no-referrer`。
- 内容响应允许被 `VIEW_ORIGIN` iframe 嵌入，禁止其他父页面嵌入。

### 10.2 iframe

查看器使用 sandbox iframe。内容域与查看器域隔离，因此允许以下能力：

- `allow-scripts`
- `allow-same-origin`

不允许：

- 顶层导航
- 弹窗
- 表单提交
- 摄像头、麦克风、定位
- 下载属性绕过
- 与查看器页面共享 Cookie

`allow-same-origin` 仅用于支持内容内部 Web Storage 和常见交互式脚本。由于内容域不承载其他敏感资源，且与父页面不同源，它不会获得查看器或管理端权限。

### 10.3 CSP

内容默认策略：

```text
default-src 'none';
script-src 'unsafe-inline' blob:;
style-src 'unsafe-inline';
img-src data: blob:;
font-src data:;
media-src data: blob:;
connect-src 'none';
frame-src 'none';
worker-src blob:;
object-src 'none';
base-uri 'none';
form-action 'none';
frame-ancestors <VIEW_ORIGIN>;
```

默认不开放 `unsafe-eval`。如果某份 HTML 必须使用 `eval`，视为不满足首发兼容范围。

### 10.4 水印

- 水印由 H5 查看器覆盖在 iframe 上方，不修改原始 HTML。
- 内容包含分享标签、查看日期和短时会话标识，不包含真实客户姓名。
- 水印层设置 `pointer-events: none`，不阻挡交互。
- 水印只用于降低泄露意愿，不承诺阻止截图。

### 10.5 管理端安全

- 管理端密码使用 Argon2id。
- 会话 Cookie 使用 HttpOnly、Secure、SameSite=Lax。
- 所有写操作校验 CSRF 和 `Origin`。
- Go 在所有管理 API 上执行 RBAC，不能依赖 Next.js 路由保护。
- 管理端设置 `frame-ancestors 'none'`。

### 10.6 隐私

- 不收集客户姓名、手机号或微信身份。
- IP 和 User-Agent 只保存带服务端密钥的 HMAC 摘要。
- 审计日志保留 180 天，过期会话和票据每日清理。
- 密码、分享令牌明文、内容对象密钥不得写入日志。

## 11. 错误处理

H5 和小程序统一处理以下状态：

- `SHARE_NOT_FOUND`
- `SHARE_EXPIRED`
- `SHARE_REVOKED`
- `PIN_INVALID`
- `RATE_LIMITED`
- `SESSION_EXPIRED`
- `CONTENT_UNAVAILABLE`
- `NETWORK_ERROR`

接口错误响应包含：

- 稳定的 `code`
- 面向用户的中文 `message`
- 可选的 `retry_after_seconds`
- `request_id`

服务端日志记录完整原因、堆栈和 `request_id`，但响应不得泄露数据库信息、对象存储路径或内部堆栈。前端遇到未知错误统一显示可重试页面。

上传失败分为文件过大、类型不支持、HTML 解析失败、外部资源不合格和存储失败。管理端必须逐项展示，不产生部分成功状态。

## 12. API 轮廓

公共接口：

- `GET /api/v1/shares/{token}/status`
- `POST /api/v1/shares/{token}/unlock`
- `POST /api/v1/miniapp/shares/{token}/unlock`
- `GET /enter?ticket=...`
- `GET /api/v1/viewer/session`
- `POST /api/v1/viewer/content-url`
- `POST /api/v1/viewer/heartbeat`
- `GET /c/{signedToken}`

管理接口：

- `POST /api/v1/admin/login`
- `POST /api/v1/admin/logout`
- `GET /api/v1/admin/me`
- `GET /api/v1/admin/contents`
- `POST /api/v1/admin/contents`
- `GET /api/v1/admin/contents/{id}`
- `POST /api/v1/admin/contents/{id}/preview-url`
- `GET /api/v1/admin/shares`
- `POST /api/v1/admin/shares`
- `GET /api/v1/admin/shares/{id}`
- `POST /api/v1/admin/shares/{id}/revoke`
- `POST /api/v1/admin/shares/{id}/reset-pin`
- `GET /api/v1/admin/audit-events`

OpenAPI 文件是接口契约的唯一来源。TypeScript 类型和客户端由 OpenAPI 生成；Go 端使用同一契约进行路由和请求校验。

## 13. 仓库结构

```text
apps/
  admin/       Next.js 管理后台
  viewer/      React + Vite H5 查看器
  miniapp/     微信原生小程序
services/
  api/         Go API
packages/
  contracts/   OpenAPI 生成的 TypeScript 类型和客户端
infra/
  compose.yaml
  Caddyfile
docs/
```

技术约束：

- Node.js 使用当前 LTS。
- 包管理使用 pnpm workspace。
- 管理端使用 Next.js App Router、TypeScript、Tailwind CSS、shadcn/ui、TanStack Table、React Hook Form 和 Zod。
- H5 查看器使用 React、Vite 和 TypeScript，不引入后台组件库。
- 微信小程序使用原生 TypeScript、WXML 和 WXSS。
- Go 使用 Chi、pgx、sqlc、goose、Argon2id、`log/slog` 和 S3 客户端。
- CSS 使用响应式布局；内容 HTML 自身必须包含 viewport 和使用流式布局。

## 14. 部署

首发使用单台 Linux 服务器和 Docker Compose：

- Caddy
- Go API
- Next.js
- H5 静态站点
- PostgreSQL
- MinIO

Caddy 负责：

- HTTPS
- 四个逻辑域名的路由
- `/api` 反向代理
- 请求体大小限制
- 基础安全响应头

PostgreSQL 和 MinIO 使用持久卷，每日执行加密备份并复制到服务器之外。Go 提供 `/health/live` 和 `/health/ready`。日志使用 JSON 输出到 stdout，由容器日志轮转管理。

必需环境变量集中在 `.env.example`，至少包含：

- 数据库连接
- S3 端点、桶名和凭据
- 四个站点 Origin
- 管理员会话签名密钥
- 分享令牌加密密钥
- 内容签名密钥
- IP 与 User-Agent HMAC 密钥

任何密钥都不得提交到 Git 或写入前端构建产物。

## 15. 测试策略

Go：

- 单元测试覆盖密码摘要、限流、过期、撤销、RBAC、内容签名和令牌票据。
- 集成测试使用真实 PostgreSQL 和 MinIO 容器，覆盖上传、分享、解锁、会话和内容签发。
- 安全测试覆盖外部资源拒绝、CSP、跨域 Cookie、过期令牌和重放票据。

前端：

- Vitest 覆盖查看器和后台关键组件状态。
- Playwright 覆盖管理员登录、上传、创建分享、H5 解锁、iframe 展示、撤销和过期。
- 类型检查、ESLint 和前端构建进入 CI。

小程序：

- 对请求封装和状态映射做单元测试。
- 使用开发者工具验证分享参数、解锁、票据、WebView 和失效跳转。
- 发布前执行真机清单：iOS、Android、弱网、过期、撤销和返回页面。

## 16. 实施顺序

1. 初始化 Git、pnpm workspace、Go module、Docker Compose、环境变量模板、基础文档和 CodeGraph。
2. 建立 OpenAPI 初版、数据库迁移、Go 配置、日志和健康检查。
3. 实现管理员创建、登录、会话和 RBAC。
4. 实现 HTML 上传、解析、对象存储和预览。
5. 实现分享创建、公共/专属语义、密码、限流、撤销和审计。
6. 实现查看器会话、一次性票据、内容签名和隔离内容服务。
7. 实现 Next.js 管理后台。
8. 实现 H5 查看器和微信小程序。
9. 补齐 Playwright、安全测试、部署配置、备份脚本和运行手册。

## 17. 验收标准

- 管理员可以通过 Next.js 后台登录并上传单个自包含 HTML。
- 上传后可以预览，外部资源和超限文件会被拒绝并给出明确原因。
- 管理员可以创建公共分享和专属分享，分别设置 4 位密码、有效期和水印。
- H5 和小程序都能通过分享入口和密码打开同一份 HTML。
- 错误密码受到双层限流，无法直接枚举。
- 分享到期或被撤销后，新访问立即失败，活动查看器在 45 秒内移除内容。
- 内容在独立域名和 sandbox iframe 中展示，不能向外部网络发请求。
- 查看器在手机、平板和桌面宽度下正常布局，内容 HTML 的响应式表现可预览。
- 管理端、H5 和小程序不出现明文密码、密钥或对象存储路径。
- 所有核心流程有自动化测试，生产部署具备 HTTPS、备份和日志轮转。
