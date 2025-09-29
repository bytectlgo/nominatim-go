# Nominatim-Go 实现计划（可回溯 TODO）

- [x] 基础：项目功能梳理与技术规范落地（见 `PROJECT_PLAN.md`）
- [x] Proto：nominatim.v1（含 HTTP 注解与校验）
- [x] Server：注册 NominatimService（gRPC/HTTP），生成代码
- [ ] 配置：对齐 Python .env/环境变量（数据库、功能开关、限流）
- [ ] 数据库：连接 Postgres+PostGIS（读路径；当前为 MySQL/SQLite 脚手架）
- [ ] Status：返回版本、数据库健康、uptime
- [ ] Search（MVP）：按名称关键字检索，limit/offset，基础排序
- [ ] Reverse（MVP）：按经纬度最近匹配，返回基础信息
- [ ] Lookup（MVP）：osm_ids 批量查询
- [ ] addressdetails：返回地址行（address_rows）
- [ ] 本地化：accept_language 与 Locales 支持显示名
- [ ] Search（增强）：类别/过滤、分页、排序优化
- [ ] Reverse（增强）：zoom 控制粒度、类型偏好
- [ ] 维护端点：/details（调试）、/deletable、/polygons（默认关闭/鉴权）
- [ ] 观测：指标、日志与追踪；错误分层与限流
- [ ] 性能：缓存与索引优化、压测达标
- [ ] 文档：OpenAPI 导出与使用说明

标记规则：每完成一个功能即在此文件勾选对应项，并在实现处附带简短注释，便于回溯。
