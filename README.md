# Nominatim-Go

## 快速开始

```bash
make all
./bin/nominatim-go -conf ./configs
```

HTTP: `:8000`，gRPC: `:9000`（见 `configs/config.yaml`）。

### 主要端点

- `/search`：名称/地址搜索（支持 `addressdetails`、`countrycodes`、`featuretype`、`layer`、`viewbox`/`bounded`、`dedupe`、`polygon_geojson` 等）
- `/reverse`：逆地理（支持 `zoom`、`addressdetails`、`layer`、多边形参数）
- `/lookup`：按 `osm_ids`（如 `N123,W456,R789`）查询
- `/details`：对象详情（可由开关关闭）
- `/status`：服务状态
- `/deletable`、`/polygons`：维护端点（可由开关关闭）
- `/metrics`：Prometheus 指标

### 示例 curl

```bash
# 搜索（GeoJSON + 多边形）
curl 'http://127.0.0.1:8000/search?q=beijing&addressdetails=1&polygon_geojson=1&format=geojson'

# 逆地理（XML）
curl 'http://127.0.0.1:8000/reverse?lat=39.9&lon=116.4&zoom=16&format=xml'

# 按 OSM IDs 查询（GeocodeJSON）
curl 'http://127.0.0.1:8000/lookup?osm_ids=W12345&namedetails=1&format=geocodejson'
```

### 输出格式

- 默认 JSON；`?format=geojson` / `?format=geocodejson` / `?format=xml`
- 多边形附加输出（需 `polygon_geojson=1`）：
  - `polygon_text=1`：在 properties 中附加 `polygon`
  - `polygon_svg=1`：附加 `svg`（Path 片段）
  - `polygon_kml=1`：附加 `kml`（KML 片段）

### 环境变量（治理/兼容）

- `NOMINATIM_LICENCE`：覆盖响应 `licence` 字段（默认 `Data © OpenStreetMap contributors`）
- `NOMINATIM_RPS`：启用全局令牌桶限流（每秒请求数）
- `NOMINATIM_ENABLE_DETAILS`：为 `0` 时关闭 `/details`
- `NOMINATIM_ENABLE_MAINTENANCE`：为 `0` 时关闭 `/deletable`、`/polygons`

### 数据库

- 需连接已有 Nominatim PostgreSQL（PostGIS）数据库；配置见 `configs/config.yaml` 中 `data.database`。

### 兼容性备注

- `featuretype` 与 `layers` 做了近似映射；`zoom→rank` 使用近似表；`viewbox` 在 `bounded=1` 时做了基本容错。

## Docker

```bash
docker build -t nominatim-go .
docker run --rm -p 8000:8000 -p 9000:9000 \
  -e NOMINATIM_RPS=20 \
  -e NOMINATIM_LICENCE="Data © OpenStreetMap contributors, ODbL 1.0" \
  -v $(pwd)/configs:/data/conf nominatim-go
```

## CLI：nominatimctl

构建：
```bash
make nominatimctl
```

设置数据库 DSN：
```bash
export PG_DSN="host=127.0.0.1 user=postgres password=postgres dbname=nominatim port=5432 sslmode=disable"
```

导入 PBF：
```bash
bin/nominatimctl import --pbf-url https://download.geofabrik.de/asia/china-latest.osm.pbf --threads 4
# 或本地 PBF
bin/nominatimctl import --pbf /path/to/osm.pbf --threads 4
```

更新与索引：
```bash
bin/nominatimctl update --mode replication
bin/nominatimctl reindex
```

就绪等待：
```bash
bin/nominatimctl waitready --url http://127.0.0.1:8000/status --timeout 10m
```

## Docker Compose（示例）

> 注意：以下为示例模板，需替换为 PostgreSQL + PostGIS 的 Nominatim 数据库（本仓库自带的 `deploy/compose` 仍是 MySQL 脚手架）。

```yaml
version: "3.9"
services:
  db:
    image: postgis/postgis:15-3.3
    environment:
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=nominatim
    ports:
      - "5432:5432"
    volumes:
      - ./pg-data:/var/lib/postgresql/data

  nominatim-go:
    image: nominatim-go:latest
    depends_on:
      - db
    ports:
      - "8000:8000"
      - "9000:9000"
    environment:
      - NOMINATIM_RPS=20
      - NOMINATIM_ENABLE_MAINTENANCE=1
      - NOMINATIM_ENABLE_DETAILS=1
    volumes:
      - ./configs:/data/conf
```

### 使用官方容器导入 OSM 数据

`deploy/compose/compose.yaml` 中提供 `nominatim-import` 服务以执行一次性导入：

```bash
# 1) 使用远程 PBF（示例：Geofabrik 中国）
PBF_URL=https://download.geofabrik.de/asia/china-latest.osm.pbf \
docker compose up -d nominatim-import

# 2) 使用本地 PBF
mkdir -p deploy/compose/data
cp <你的数据>.osm.pbf deploy/compose/data/osm.pbf
docker compose up -d nominatim-import

# 查看导入日志
docker logs -f nominatim-import

# 导入完成后，停止并移除导入容器
docker compose rm -sf nominatim-import

# 启动应用
docker compose up -d nominatim-go
```
