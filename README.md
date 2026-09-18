# 工业机器人安全包络校验

`robot-cell-safety-envelope-validator` 是面向工业机器人集成商与工厂安全工程师的离线工程工作台，用于维护工作单元、三维近似安全区域、结构化运动轨迹和联锁依赖，并保存可重放的仿真校验快照。

```bash
cp .env.example .env
# 修改 .env 中的数据库密码、DB_DSN 和 JWT_SECRET
docker compose up -d --build
docker compose ps
```

三个服务都应达到 `healthy`。访问 `http://localhost:18533`。

> 本系统只提供二维加高度近似的决策支持，不连接机器人控制器，不上传或下发程序，不控制门禁、光栅或急停，也不代表认证安全软件、人工验收或现场运行授权。

## 主要功能

- 工作单元：维护机器人、控制器、最大臂展、二维布局、责任团队和冻结版本。
- 安全区域：在画布上绘制 Polygon，维护高度、速度限制、准入规则和修订状态。
- 运动程序：导入带时间戳的结构化轨迹与联锁依赖，计算 SHA-256 来源校验值并执行版本状态流。
- 包络仿真：将工具与负载半径扩张成扫掠包络，与启用区域的二维多边形和高度区间求交。
- 联锁校验：检测缺失前置、顺序反转和有向循环，并输出逐条证据。
- 人工复核：保留输入快照、算法版本、输入哈希、发现详情、评审和接受/作废记录。
- 审计中心：按操作者、request ID、实体和动作查询所有写操作的前后投影。

## 角色与本地账号

| 角色 | 用户名 | 密码 | 主要权限 |
| --- | --- | --- | --- |
| `safety_engineer` | `engineer` | `Safety#533` | 工作单元、区域和仿真 |
| `robot_programmer` | `programmer` | `Program#533` | 程序导入和状态迁移 |
| `reviewer` | `reviewer` | `Review#533` | 校验评审、接受、作废和审计 |
| `auditor` | `auditor` | `Audit#533` | 只读业务数据和审计 |
| `admin` | `admin` | `Admin#533` | 管理能力并受自审隔离约束 |

账号仅用于本地演示。生产环境必须替换种子认证、密码和 JWT 密钥。程序上传者不能接受自己的校验结果，即使其同时具有管理员角色。

## 页面

| 路径 | 消费的真实数据 | 关键操作 |
| --- | --- | --- |
| `/cells` | `RobotCell`、`SafetyZone` | 建档、编辑、冻结布局、停用 |
| `/zones` | `SafetyZone`、`RobotCell` | 画布绘制、修订、启用、停用 |
| `/programs` | `MotionProgram`、`RobotCell`、`SafetyZone` | 导入、解析、就绪、激活、替代 |
| `/validation` | `ValidationRun`、`MotionProgram`、`SafetyZone`、`ViolationWaiver` | 仿真、证据回放、违规豁免、评审、接受、作废 |
| `/audit` | 四实体审计投影 | 操作者、request ID、实体和动作筛选 |

共享组件：

- `CellStateBadge`：状态颜色同时配合图标和文字，在工作单元、程序、区域和校验页复用。
- `SafetyCanvas`：区域页可点选绘制，工作单元、程序和校验页显示真实区域/轨迹快照。
- `FindingDrawer`：程序、校验和审计页展开联锁、碰撞或变更证据。
- `useAuth` 与 `useValidationRun`：集中注入认证权限和校验运行 store。

## 状态流

### MotionProgram

```text
uploaded -> parsed -> ready -> active -> superseded
     \          \        \
      +----------+---------> rejected
```

状态由后端条件更新校验。激活新版本时，同一工作单元的旧 `active` 版本在同一事务中改为 `superseded`。非法跳跃返回 HTTP 409。

### ValidationRun

```text
queued -> simulating -> passed | failed -> reviewed -> accepted
                              \  (waiver complete)  ^          \
                               +--------------------+-----------> voided
```

带违规的 `failed` 运行在每条违规都持有未过期豁免时，可直接与豁免同事务进入 `accepted`；否则必须先 `reviewed` 且补齐豁免后才能接受。

- 仿真接口要求 `Idempotency-Key`。
- 相同输入哈希与算法版本默认复用既有结果。
- `failed` 结果可显式重试，新记录保存 `attempt` 和 `retry_of_id`，不会覆盖旧尝试。
- 评审员可为单条包络违规（`finding_kind=envelope_violation`）或联锁发现（`interlock_finding`）登记“违规豁免”，填写理由与截止时间；同一条发现只允许一个未过期豁免，重复、过期或指向非违规项的提交分别返回 409/422。
- 接受带违规的运行前，每一条违规都必须有未过期豁免；豁免追加与接受在同一事务落盘，任一校验失败则整批回滚、运行保持失败。
- 程序上传者既不能接受自己的校验结果，也不能为其登记豁免，即使其同时具有管理员角色。
- 过期豁免不删除：刷新运行详情时历史豁免仍随证据回读，仅有一条新的未过期豁免会生效。
- `accepted` 只表示离线证据被独立记录，不等于机器人可运行。

## 包络算法、假设与误差边界

轨迹点格式：

```json
{"x_mm": 0, "y_mm": 100, "z_mm": 800, "time_ms": 2500, "speed_mm_s": 320}
```

计算步骤：

1. 校验轨迹至少两个点，坐标/时间/速度为有限值，时间严格递增。
2. 将 `tool_radius_mm + payload_radius_mm` 作为各轨迹段的保守径向扩张。
3. 每段按 1% 时间步长采样，在 XY 平面计算点到 Polygon 的带符号距离。
4. 同时检查 `z ± 扩张半径` 是否和区域 `[min_height_mm, max_height_mm]` 相交。
5. 保存首次采样相交时刻、段号、区域、净距、实际速度、允许速度和判定依据。
6. `operating` 区域内且不超速的接触是信息项；`restricted`、`service`、`escape` 接触或超速是违规项。

误差与适用边界：

- 这是二维平面加高度区间的近似，不计算机器人连杆、自碰、奇异点、加速度、制动距离或控制器插补细节。
- 1% 离散采样可能将首次接触时间最多量化到单段持续时间的约 1%。
- Polygon 仅支持单个无孔、闭合且不自交的外环；坐标单位固定为毫米。
- 工具和负载被近似为统一半径，不能代替制造商模型、认证仿真或现场测量。
- 风险分只用于排序发现，不是安全等级、合格证明或运行许可。

## 联锁算法

每个事件包含唯一 `name`、正整数 `sequence` 和 `depends_on` 名称数组。后端建立有向依赖图并输出：

- `missing_prerequisite`：引用的前置事件不存在。
- `reversed_order`：前置事件序号不早于依赖它的事件。
- `dependency_cycle`：深度优先遍历发现闭环，并保存完整路径。

门禁、光栅、急停和速度切换只是离线事件名称。系统不读取或改变任何现场信号。

## 技术栈

| 层 | 技术 |
| --- | --- |
| 前端 | Angular 17、TypeScript、Angular Material、Signals store、RxJS、Lucide，Angular application builder（esbuild/Vite 开发服务器） |
| 后端 | Go 1.22、Gin、GORM、validator/v10、JWT、slog |
| 正式数据库 | PostgreSQL 16 |
| 独立 smoke/测试 | GORM SQLite |
| 部署 | Docker 多阶段构建、Nginx、Docker Compose V2 |

## 架构与目录

```text
.
├── backend/
│   ├── cmd/server/                         # 启动和优雅停机
│   └── internal/
│       ├── config/                         # PostgreSQL/SQLite、迁移、种子
│       ├── constants/                      # 共享枚举和状态机
│       ├── model/ dto/ repository/         # 四实体独立文件
│       ├── service/ handler/ router/       # 四实体独立业务与 HTTP 层
│       ├── geometry/ algorithm/            # 包络和联锁算法
│       └── middleware/                     # request ID、恢复、认证、RBAC、审计、错误
├── frontend/src/
│   ├── api/ stores/ types/                 # 四实体独立客户端、store 和类型
│   ├── components/common/                  # Badge、Canvas、Drawer
│   ├── hooks/ pages/ router/ utils/        # inject helper、页面、守卫、错误
│   └── app/                                # 应用壳和配置
├── database/init.sql
├── docker-compose.yml
├── go.work
└── runtime_smoke.json
```

依赖方向为 `handler -> service -> repository -> model`。handler 不持有数据库；构造器在 router 层完成注入。多记录程序激活和校验运行写入使用数据库事务。

## 共享枚举位置

### ZoneType

值：`operating | restricted | service | escape`。

- 后端常量：`backend/internal/constants/zone_type.go`
- 数据库/model：`backend/internal/model/safety_zone.go`
- DTO/service/handler：`backend/internal/dto/safety_zone.go`、`service/safety_zone.go`、`handler/safety_zone.go`
- 算法与测试：`backend/internal/geometry/envelope.go`、`geometry/envelope_test.go`
- 前端类型/store/component/page：`frontend/src/types/enums/zone-type.ts`、`stores/safety-zone.store.ts`、`components/common/safety-canvas.component.ts`、`pages/zones.page.ts`

### ValidationStatus

值：`queued | simulating | passed | failed | reviewed | accepted | voided`。

- 后端常量/状态机：`backend/internal/constants/validation_status.go`
- 数据库/model：`backend/internal/model/validation_run.go`
- DTO/service/handler：`backend/internal/dto/validation_run.go`、`service/validation_run.go`、`handler/validation_run.go`
- 状态机测试：`backend/internal/constants/validation_status_test.go`
- 前端类型/store/component/page：`frontend/src/types/enums/validation-status.ts`、`stores/validation-run.store.ts`、`components/common/cell-state-badge.component.ts`、`pages/validation.page.ts`

## API

统一前缀为 `/api/v1`；所有响应包含 `request_id`。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/auth/login` | 登录并签发 JWT |
| GET/POST | `/cells` | 列表、工作单元建档 |
| GET/PUT | `/cells/:id` | 详情、草稿布局乐观锁更新 |
| POST | `/cells/:id/freeze`、`deactivate` | 冻结或停用 |
| GET/POST | `/zones` | 列表、区域创建 |
| GET/PUT | `/zones/:id` | 详情、版本修订 |
| POST | `/zones/:id/activate`、`deactivate` | 区域状态动作 |
| GET/POST | `/programs` | 列表、结构化程序导入 |
| GET | `/programs/:id` | 程序详情和 checksum |
| POST | `/programs/:id/transition` | 程序状态迁移 |
| GET/POST | `/validations` | 列表、幂等仿真 |
| GET | `/validations/:id` | 冻结证据详情（含全部豁免历史） |
| POST | `/validations/:id/review`、`accept`、`void` | 人工处置 |
| POST | `/validations/:id/waivers` | 为单条包络违规或联锁发现追加限时豁免 |
| GET | `/audit` | 审计筛选 |

健康端点为 `/healthz` 与 `/readyz`。统一错误码包括 `invalid_geometry`、`invalid_trajectory`、`invalid_program_transition`、`version_conflict`、`state_conflict`、`forbidden` 和 `unauthorized`。

## 环境变量和端口

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| `COMPOSE_PROJECT_NAME` | `robot-cell-safety-envelope-validator` | Compose 隔离名称 |
| `FRONTEND_PORT` | `18533` | 前端宿主端口 |
| `BACKEND_PORT` | `19533` | 后端宿主端口 |
| `DB_PORT` | `57533` | PostgreSQL 宿主端口 |
| `POSTGRES_DB/USER/PASSWORD` | 见 `.env.example` | 数据库初始化 |
| `DB_DRIVER` | `postgres` | Compose 正式驱动 |
| `DB_DSN` | PostgreSQL DSN | 正式连接字符串 |
| `DB_AUTO_MIGRATE` | `true` | 启动迁移 |
| `JWT_SECRET` | 必须修改 | JWT HMAC 密钥 |
| `JWT_TTL_MINUTES` | `480` | 访问令牌有效期 |
| `CORS_ORIGIN` | `http://localhost:18533` | 本地允许源 |
| `RATE_LIMIT_PER_MINUTE` | `300` | 全局单进程 IP 限流 |
| `ALGORITHM_VERSION` | `envelope-2d-height-v1.0` | 冻结到结果的算法版本 |

Nginx 只让前端访问相对 `/api`，原样代理 `/api/v1`，并单独映射 `/api/healthz` 与 `/api/readyz`。

## 本地开发与验证

需要 Go 1.22、Node.js 20、npm，以及 CGO 可用的 SQLite 工具链。

```bash
go work sync
go build ./backend/...
go vet ./backend/...
go test ./backend/...
go test -race ./backend/...

npm --prefix frontend ci
npm --prefix frontend run typecheck
npm --prefix frontend run build



```

`runtime_smoke.json` 只描述 SQLite 服务启动：端口 `20533`、工作目录 `backend`、`go run ./cmd/server`。Compose 始终使用 PostgreSQL。

启动 PostgreSQL 主链后运行：

```bash
scripts/api_smoke.sh
```

## Docker 部署

```bash
docker compose config --quiet
docker compose up -d --build
docker compose ps
curl -fsS http://127.0.0.1:19533/healthz
curl -fsS http://127.0.0.1:19533/readyz
curl -fsS http://127.0.0.1:18533/api/healthz
```

## 常见问题

- **数据库不健康**：确认 `.env` 的 `POSTGRES_PASSWORD` 与 `DB_DSN` 密码完全一致。
- **前端 API 404**：必须从 `http://localhost:18533` 访问；Nginx 保留 `/api/v1` 前缀。
- **422 invalid_geometry**：Polygon 必须闭合、无孔、不自交且面积至少 1 mm²。
- **409 invalid_program_transition**：按 uploaded -> parsed -> ready -> active 顺序推进。
- **仿真返回旧结果**：相同输入哈希和算法版本会复用；仅失败结果允许 `retry_failed=true` 创建新尝试。
- **422 unwaived_violations**：接受带违规的失败运行前，必须为每条包络违规和联锁发现登记未过期豁免；豁免可随 `/accept` 请求一次性提交，失败时整批回滚。
- **409 duplicate_waiver**：该发现已存在未过期豁免；过期后才能登记新豁免，旧记录保留在历史中。
- **接受后仍显示风险**：预期行为。人工处置不会篡改碰撞、联锁或风险证据。
- **管理员接受返回 403**：如果管理员本人上传了该程序，自审隔离仍然生效。

## 停止

```bash
docker compose down -v --remove-orphans
```

只会删除本项目容器、网络和命名 PostgreSQL 卷。

## License

MIT
