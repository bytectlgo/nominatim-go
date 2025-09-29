# 项目计划：Nominatim-Go 重实现

## 目标

以 Go 语言重实现 Nominatim 的核心能力，提供与 Python 实现一致或兼容的 API/行为，并保持可部署性、可维护性与可扩展性。

## 项目功能范围（源自 0-Nominatim-py 文档梳理）

- 核心能力
  - 搜索（Geocoding）：根据名称、地址、类别/类型进行搜索
  - 逆地理编码（Reverse Geocoding）：根据经纬度查找地点与地址
  - 对象查询（Lookup）：通过 OSM 对象 ID 查询详情与地址
  - 状态查询（Status）：服务健康与索引状态查询
  - 详情（Details）：调试用的对象内部信息展示
  - 维护查询：
    - deletable：已在 OSM 删除、在本地延迟清理的对象列表
    - polygons：损坏多边形列表

- 数据与结果
  - 从本地 Nominatim 数据库读取（PostgreSQL + PostGIS）
  - 结果包含：名称多语言、地址组件（address_rows）、类别/类型、rank、centroid 等
  - 提供结果本地化（Locale/Locales）能力，便于输出面向人类的简要标签

- 配置
  - 支持通过项目目录 .env、环境变量、显式参数三种方式配置数据库与行为

- 性能与并发
  - Python 版本提供同步与异步（asyncio）两种 API；Go 版本以 goroutine 原生并发实现

- 可部署性
  - 提供 HTTP Web 服务（兼容 /search, /reverse, /lookup, /status, /details 等）

## Go 重实现总体设计

- 语言与运行时
  - Go 1.22+，模块化依赖管理，采用标准库与少量成熟第三方库

- 存储
  - PostgreSQL 15+，PostGIS 扩展
  - 兼容原 Nominatim 表与索引；优先直连读取既有 Nominatim 数据库

- 服务形态
  - gRPC + HTTP（RESTful）双栈：
    - HTTP 对外提供兼容的 API 路径与查询参数
    - gRPC 提供内部/批量/高并发访问

- 并发模型
  - 使用 context 控制生命周期与限流，连接池与只读事务优化

- 配置
  - 统一使用 `configs/config.yaml` 与环境变量；优先环境变量（12-factor）

## 模块划分

- API 层（HTTP/gRPC）
  - 端点：/search, /reverse, /lookup, /status, /details, /deletable, /polygons
  - 参数解析、校验、速率限制、审计日志

- 业务层（internal/biz）
  - SearchService：名称/地址/类别检索与排序
  - ReverseService：点到地址的空间查询
  - LookupService：依据 OSM ID 汇总详情
  - DetailsService/StatusService：调试与健康检查
  - Localization：结果本地化（语言优先级、展示名生成）

- 数据访问层（internal/data）
  - 读路径：复杂 SQL + PostGIS 空间查询
  - Schema 适配：对接现有 Nominatim 表结构
  - 缓存：热点名称、行政层级、类别映射

- 公共组件（pkg/）
  - 错误模型、日志、指标、限流、中间件

## API 兼容性（V1）

- /search
  - 支持名称关键字、类别（如 amenity=...）、过滤（国家/边界）、分页、address_details

- /reverse
  - 支持 lat/lon、缩放级别、附近类型偏好、返回地址组件

- /lookup
  - 支持 osm_ids 批量查询，返回基本信息与地址

- /status
  - 返回数据库连接、索引状态、版本、uptime

- /details
  - 调试用，受权限或开关控制

- /deletable, /polygons
  - 作为维护端点，默认关闭或受鉴权控制

## 数据与索引

- 必需依赖
  - PostgreSQL + PostGIS
  - 采用已导入的 Nominatim 数据库（与 Python 导入流程保持兼容）

- 查询优化
  - GIN/GIST 索引利用
  - 业务层二级缓存（内存/可选 Redis）

## 本地化与展示

- 语言优先级解析（如 ["zh", "en"]）
- 生成 address_rows 与 display labels 的通用逻辑

## 非功能性需求

- 性能：与 Python 服务相当或更优；P95 延迟可控
- 可靠性：超时/重试/熔断；优雅关闭
- 可观测性：Prometheus 指标、结构化日志、追踪（OpenTelemetry）
- 安全：限流、鉴权（管理端点）、输入校验

## 里程碑

1. 可运行的最小子集（MVP）
   - /status, /search(name-only), /reverse(基础)
   - 直连现有 Nominatim DB，返回核心字段

2. 功能补全
   - /lookup, address_details、本地化、多类别/过滤、分页

3. 维护与调试端点
   - /details, /deletable, /polygons（默认关闭或需鉴权）

4. 性能与稳定性
   - 索引/查询优化、缓存、并发压测、熔断与降级

5. 生产化
   - 部署脚本、监控、日志、SLO、容量评估

## 落地计划（结合当前仓库）

- 现状
  - 已有 `cmd/nominatim-go`、`internal/*`、`ent/*`、`api/helloworld` 脚手架

- 计划
  - 定义 proto：`proto/nominatim/v1/*.proto`（search/reverse/lookup/status/details）
  - 生成 gRPC/HTTP 网关代码，补充 `internal/service` 实现
  - 在 `internal/data` 编写 SQL/查询器，接入 PostGIS
  - 在 `internal/biz` 组织搜索与本地化逻辑
  - 在 `internal/server` 暴露端点与中间件
  - 编写端到端用例与性能基准

## 风险与应对

- 复杂 SQL 与数据模型：优先复用 Nominatim 现有 schema 与查询模式
- 多语言与本地化：抽象 Locale 组件，建立单元测试样例
- 性能瓶颈：自上而下指标观测，逐步引入缓存与索引微调
- 维护端点安全：默认关闭或仅内网/鉴权可用

## 项目技术与规范

- 语言与版本
  - Go 1.22+；严格启用 `go mod`；`-race` 用于并发关键路径测试

- 代码风格
  - gofmt/gofumpt；golangci-lint 开启常用 linters（govet, staticcheck, errcheck, gosimple, ineffassign, revive）
  - 命名清晰、函数短小、早返回，禁止无意义空 catch（禁止吞错）
  - 错误处理：`errors.Join`/`fmt.Errorf("...: %w", err)` 包装，分层语义错误

- 结构组织
  - `cmd/`：入口可执行
  - `internal/`：业务与数据实现（`server/`, `service/`, `biz/`, `data/`）
  - `api/`：HTTP 层与 gRPC 网关
  - `proto/`：服务协议；通过 buf/插件生成
  - `ent/`：实体与查询器；聚焦读路径（与 PostGIS 共存）
  - `pkg/`：通用库（日志、中间件、指标）

- 配置与环境
  - `configs/config.yaml` + 环境变量覆盖；遵循 12-factor；敏感信息通过环境变量/Secret 管理

- 可观测性
  - 日志：结构化（zap/log/slog），统一字段（trace_id, req_id, user_agent, latency_ms）
  - 指标：Prometheus（请求计数/延迟/错误率/DB 池指标）
  - 追踪：OpenTelemetry，入站/出站全链路

- 安全与治理
  - 速率限制、入参校验、管理端点鉴权；默认关闭维护端点
  - 依赖治理：`go mod tidy` + `govulncheck`

- 测试与质量
  - 单测：服务层、数据层隔离；使用 Testcontainers 启动 PostGIS
  - 基准：核心查询基准（search/reverse）
  - CI：lint、test、build、vuln、镜像构建

- 构建与发布
  - Makefile 统一：`make lint|test|build|docker`
  - Docker 多阶段构建，非 root 运行
  - 版本与变更：语义化版本
