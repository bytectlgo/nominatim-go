package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"nominatim-go/internal/biz"
)

// NewSearchRepo 暂时返回实现。
func NewSearchRepo(d *Data) biz.SearchRepo {
	return &searchRepo{data: d}
}

type searchRepo struct {
	data *Data
}

func (r *searchRepo) sqlDB() *sql.DB {
	return r.data.SQLDB()
}

func (r *searchRepo) SearchPlaces(ctx context.Context, p biz.SearchParams) ([]*biz.SearchPlace, error) {
	if !(r.data.conf.Database.Driver == "postgres" || r.data.conf.Database.Driver == "postgresql" || r.data.conf.Database.Driver == "pgx") {
		return []*biz.SearchPlace{}, nil
	}
	db := r.sqlDB()
	if db == nil {
		return []*biz.SearchPlace{}, nil
	}

	var ccodes []string
	for _, c := range strings.Split(p.CountryCodes, ",") {
		c = strings.TrimSpace(strings.ToLower(c))
		if c != "" {
			ccodes = append(ccodes, c)
		}
	}

	geoJSONSelect := "''"
	if p.PolygonGeoJSON {
		if p.PolygonThreshold > 0 {
			geoJSONSelect = "COALESCE(ST_AsGeoJSON(ST_Simplify(polygon, " + strconv.FormatFloat(p.PolygonThreshold, 'f', -1, 64) + "), 6)::text, '')"
		} else {
			geoJSONSelect = "COALESCE(ST_AsGeoJSON(polygon, 6)::text, '')"
		}
	}

	// DISTINCT ON 去重设置
	distinctOn := ""
	orderPrefix := ""
	if p.Dedupe {
		distinctOn = "DISTINCT ON (osm_type, osm_id)"
		orderPrefix = "osm_type, osm_id, "
	}

	// 近距离去重逻辑：当 dedupe 启用时，对相同 class/type 且栅格化质心一致者仅保留重要性高者
	gridExpr := "ST_SnapToGrid(centroid, 0.0005)"
	if !p.Dedupe {
		gridExpr = "centroid"
	}
	base := `
WITH base AS (
  SELECT
    place_id, osm_id, osm_type, class, type,
    ` + gridExpr + ` AS gcentroid,
    COALESCE(name->'name','') AS name,
    COALESCE(ST_Y(centroid), 0) AS lat,
    COALESCE(ST_X(centroid), 0) AS lon,
    COALESCE(importance, 0) AS importance,
    COALESCE(ST_YMin(bbox), 0) AS south,
    COALESCE(ST_YMax(bbox), 0) AS north,
    COALESCE(ST_XMin(bbox), 0) AS west,
    COALESCE(ST_XMax(bbox), 0) AS east,
    COALESCE(hstore_to_json(name)::text, '{}') AS name_json,
    COALESCE(hstore_to_json(extratags)::text, '{}') AS extratags_json,
    ` + geoJSONSelect + ` AS polygon_geojson
  FROM placex
  WHERE (name ? 'name') AND (name->'name' ILIKE $1)
), deduped AS (
  SELECT ` + distinctOn + ` *
  FROM base
  ` + func() string {
		if p.Dedupe {
			return "ORDER BY class, type, gcentroid, importance DESC NULLS LAST, place_id DESC"
		}
		return ""
	}() + `
)
SELECT place_id, osm_id, osm_type, class, type,
       name, lat, lon, importance, south, north, west, east, name_json, extratags_json, polygon_geojson
FROM deduped
`
	args := []any{"%" + p.Q + "%"}
	argIdx := 2
	if len(ccodes) > 0 {
		placeholders := make([]string, 0, len(ccodes))
		for _, c := range ccodes {
			placeholders = append(placeholders, "$"+strconv.Itoa(argIdx))
			args = append(args, c)
			argIdx++
		}
		base += " AND country_code IN (" + strings.Join(placeholders, ",") + ")"
	}
	// featuretype 过滤：支持 "class:type" 或单值（匹配 class 或 type）
	if ft := strings.TrimSpace(p.FeatureType); ft != "" {
		if i := strings.Index(ft, ":"); i >= 0 {
			classVal := strings.TrimSpace(ft[:i])
			typeVal := strings.TrimSpace(ft[i+1:])
			base += " AND class = $" + strconv.Itoa(argIdx)
			args = append(args, classVal)
			argIdx++
			base += " AND type = $" + strconv.Itoa(argIdx)
			args = append(args, typeVal)
			argIdx++
		} else {
			// 对齐 v1 API：country/state/city/settlement 映射 rank_address 范围
			minRank, maxRank := mapFeatureTypeToRankRange(ft)
			if minRank > 0 || maxRank < math.MaxInt32 {
				base += " AND rank_address BETWEEN $" + strconv.Itoa(argIdx) + " AND $" + strconv.Itoa(argIdx+1)
				args = append(args, minRank, maxRank)
				argIdx += 2
			} else {
				// 回退到按 class/type 单值匹配
				base += " AND (class = $" + strconv.Itoa(argIdx) + " OR type = $" + strconv.Itoa(argIdx) + ")"
				args = append(args, ft)
				argIdx++
			}
		}
	}
	// viewbox 容错：仅当 bounded=true 且 viewbox 非全零且 left<right、bottom<top 时应用
	if p.Bounded {
		left, right := p.ViewBoxLeft, p.ViewBoxRight
		top, bottom := p.ViewBoxTop, p.ViewBoxBottom
		if !(left == 0 && right == 0 && top == 0 && bottom == 0) && left < right && bottom < top {
			base += " AND bbox && ST_MakeEnvelope($" + strconv.Itoa(argIdx) + ", $" + strconv.Itoa(argIdx+1) + ", $" + strconv.Itoa(argIdx+2) + ", $" + strconv.Itoa(argIdx+3) + ", 4326)"
			args = append(args, left, bottom, right, top)
			argIdx += 4
		}
	}
	if len(p.Layers) > 0 {
		classes := mapLayersToClasses(p.Layers)
		if len(classes) > 0 {
			list := make([]string, 0, len(classes))
			for c := range classes {
				list = append(list, c)
			}
			base += " AND class = ANY($" + strconv.Itoa(argIdx) + ")"
			args = append(args, pqArray(list))
			argIdx++
		}
	}
	if len(p.ExcludePlaceIDs) > 0 {
		base += " AND place_id <> ALL($" + strconv.Itoa(argIdx) + ")"
		// PostgreSQL 数组：使用自定义 pqArrayInt64
		baseArgs := make([]string, 0, len(p.ExcludePlaceIDs))
		for _, id := range p.ExcludePlaceIDs {
			baseArgs = append(baseArgs, strconv.FormatInt(id, 10))
		}
		args = append(args, pqArray(baseArgs))
		argIdx++
	}
	// 若提供 viewbox，则按视窗中心距离进行次级排序
	if (p.ViewBoxLeft != 0 || p.ViewBoxRight != 0 || p.ViewBoxTop != 0 || p.ViewBoxBottom != 0) && p.ViewBoxLeft < p.ViewBoxRight && p.ViewBoxBottom < p.ViewBoxTop {
		centerLon := (p.ViewBoxLeft + p.ViewBoxRight) / 2
		centerLat := (p.ViewBoxBottom + p.ViewBoxTop) / 2
		base += " ORDER BY " + orderPrefix + "importance DESC NULLS LAST, (centroid <-> ST_SetSRID(ST_Point($" + strconv.Itoa(argIdx) + ",$" + strconv.Itoa(argIdx+1) + "), 4326)), place_id DESC"
		args = append(args, centerLon, centerLat)
		argIdx += 2
	} else {
		base += " ORDER BY " + orderPrefix + "importance DESC NULLS LAST, place_id DESC"
	}
	base += " LIMIT $" + strconv.Itoa(argIdx) + " OFFSET $" + strconv.Itoa(argIdx+1)
	args = append(args, p.Limit, p.Offset)

	rows, err := db.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*biz.SearchPlace
	for rows.Next() {
		var it biz.SearchPlace
		var osmType string
		var nameJSON, extratagsJSON, poly string
		if err := rows.Scan(&it.PlaceID, &it.OSMID, &osmType, &it.Category, &it.Type, &it.Name, &it.Lat, &it.Lon, &it.Importance, &it.BBoxSouth, &it.BBoxNorth, &it.BBoxWest, &it.BBoxEast, &nameJSON, &extratagsJSON, &poly); err != nil {
			return nil, err
		}
		switch strings.ToUpper(osmType) {
		case "N":
			it.OSMType = "node"
		case "W":
			it.OSMType = "way"
		case "R":
			it.OSMType = "relation"
		default:
			it.OSMType = strings.ToLower(osmType)
		}
		_ = json.Unmarshal([]byte(nameJSON), &it.NameDetails)
		_ = json.Unmarshal([]byte(extratagsJSON), &it.ExtraTags)
		it.PolygonGeoJSON = poly
		// addressdetails
		if p.AddressDetails {
			rows2, err2 := r.fetchAddressRows(ctx, db, it.PlaceID)
			if err2 == nil {
				it.AddressRows = rows2
			}
		}
		out = append(out, &it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *searchRepo) ReversePlace(ctx context.Context, p biz.ReverseParams) (*biz.SearchPlace, error) {
	if !(r.data.conf.Database.Driver == "postgres" || r.data.conf.Database.Driver == "postgresql" || r.data.conf.Database.Driver == "pgx") {
		return nil, nil
	}
	db := r.sqlDB()
	if db == nil {
		return nil, nil
	}

	geoJSONSelect := "''"
	if p.PolygonGeoJSON {
		if p.PolygonThreshold > 0 {
			geoJSONSelect = "COALESCE(ST_AsGeoJSON(ST_Simplify(polygon, " + strconv.FormatFloat(p.PolygonThreshold, 'f', -1, 64) + "), 6)::text, '')"
		} else {
			geoJSONSelect = "COALESCE(ST_AsGeoJSON(polygon, 6)::text, '')"
		}
	}

	// 将 zoom 转换为 rank 上限，近似：对齐 v1 的 helpers.zoom_to_rank
	maxRank := zoomToMaxRank(p.Zoom)
	q := `
SELECT place_id, osm_id, osm_type, class, type,
       COALESCE(name->'name','') AS name,
       COALESCE(ST_Y(centroid), 0) AS lat,
       COALESCE(ST_X(centroid), 0) AS lon,
       COALESCE(importance, 0) AS importance,
       COALESCE(ST_YMin(bbox), 0) AS south,
       COALESCE(ST_YMax(bbox), 0) AS north,
       COALESCE(ST_XMin(bbox), 0) AS west,
       COALESCE(ST_XMax(bbox), 0) AS east,
       COALESCE(hstore_to_json(name)::text, '{}') AS name_json,
       COALESCE(hstore_to_json(extratags)::text, '{}') AS extratags_json,
       ` + geoJSONSelect + ` AS polygon_geojson
FROM placex
WHERE rank_address <= $3`

	args := []any{p.Lon, p.Lat, maxRank}
	if len(p.Layers) > 0 {
		classes := mapLayersToClasses(p.Layers)
		if len(classes) > 0 {
			list := make([]string, 0, len(classes))
			for c := range classes {
				list = append(list, c)
			}
			q += " AND class = ANY($4)"
			args = append(args, pqArray(list))
		}
	}
	q += `
ORDER BY centroid <-> ST_SetSRID(ST_Point($1,$2), 4326)
LIMIT 1`

	row := db.QueryRowContext(ctx, q, args...)
	var it biz.SearchPlace
	var osmType string
	var nameJSON, extratagsJSON, poly string
	if err := row.Scan(&it.PlaceID, &it.OSMID, &osmType, &it.Category, &it.Type, &it.Name, &it.Lat, &it.Lon, &it.Importance, &it.BBoxSouth, &it.BBoxNorth, &it.BBoxWest, &it.BBoxEast, &nameJSON, &extratagsJSON, &poly); err != nil {
		return nil, err
	}
	switch strings.ToUpper(osmType) {
	case "N":
		it.OSMType = "node"
	case "W":
		it.OSMType = "way"
	case "R":
		it.OSMType = "relation"
	default:
		it.OSMType = strings.ToLower(osmType)
	}
	_ = json.Unmarshal([]byte(nameJSON), &it.NameDetails)
	_ = json.Unmarshal([]byte(extratagsJSON), &it.ExtraTags)
	it.PolygonGeoJSON = poly
	if p.AddressDetails {
		rows2, err2 := r.fetchAddressRows(ctx, db, it.PlaceID)
		if err2 == nil {
			it.AddressRows = rows2
		}
	}
	return &it, nil
}

func (r *searchRepo) LookupPlaces(ctx context.Context, p biz.LookupParams) ([]*biz.SearchPlace, error) {
	if !(r.data.conf.Database.Driver == "postgres" || r.data.conf.Database.Driver == "postgresql" || r.data.conf.Database.Driver == "pgx") {
		return []*biz.SearchPlace{}, nil
	}
	db := r.sqlDB()
	if db == nil {
		return []*biz.SearchPlace{}, nil
	}

	var idsNode, idsWay, idsRel []string
	for _, s := range p.OSMIDs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		switch strings.ToUpper(s[:1]) {
		case "N":
			idsNode = append(idsNode, s[1:])
		case "W":
			idsWay = append(idsWay, s[1:])
		case "R":
			idsRel = append(idsRel, s[1:])
		}
	}

	parts := make([]string, 0, 3)
	args := make([]any, 0)
	if len(idsNode) > 0 {
		parts = append(parts, "(osm_type = 'N' AND osm_id = ANY($"+strconv.Itoa(len(args)+1)+"))")
		args = append(args, pqArray(idsNode))
	}
	if len(idsWay) > 0 {
		parts = append(parts, "(osm_type = 'W' AND osm_id = ANY($"+strconv.Itoa(len(args)+1)+"))")
		args = append(args, pqArray(idsWay))
	}
	if len(idsRel) > 0 {
		parts = append(parts, "(osm_type = 'R' AND osm_id = ANY($"+strconv.Itoa(len(args)+1)+"))")
		args = append(args, pqArray(idsRel))
	}
	if len(parts) == 0 {
		return []*biz.SearchPlace{}, nil
	}

	geoJSONSelect := "COALESCE(ST_AsGeoJSON(polygon, 6)::text, '')"
	if p.PolygonGeoJSON && p.PolygonThreshold > 0 {
		geoJSONSelect = "COALESCE(ST_AsGeoJSON(ST_Simplify(polygon, " + strconv.FormatFloat(p.PolygonThreshold, 'f', -1, 64) + "), 6)::text, '')"
	}

	q := `
SELECT place_id, osm_id, osm_type, class, type,
       COALESCE(name->'name','') AS name,
       COALESCE(ST_Y(centroid), 0) AS lat,
       COALESCE(ST_X(centroid), 0) AS lon,
       COALESCE(importance, 0) AS importance,
       COALESCE(ST_YMin(bbox), 0) AS south,
       COALESCE(ST_YMax(bbox), 0) AS north,
       COALESCE(ST_XMin(bbox), 0) AS west,
       COALESCE(ST_XMax(bbox), 0) AS east,
       COALESCE(hstore_to_json(name)::text, '{}') AS name_json,
       COALESCE(hstore_to_json(extratags)::text, '{}') AS extratags_json,
       ` + geoJSONSelect + ` AS polygon_geojson
FROM placex
WHERE ` + strings.Join(parts, " OR ") + `
ORDER BY importance DESC NULLS LAST`

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*biz.SearchPlace
	for rows.Next() {
		var it biz.SearchPlace
		var osmType string
		var nameJSON, extratagsJSON, poly string
		if err := rows.Scan(&it.PlaceID, &it.OSMID, &osmType, &it.Category, &it.Type, &it.Name, &it.Lat, &it.Lon, &it.Importance, &it.BBoxSouth, &it.BBoxNorth, &it.BBoxWest, &it.BBoxEast, &nameJSON, &extratagsJSON, &poly); err != nil {
			return nil, err
		}
		switch strings.ToUpper(osmType) {
		case "N":
			it.OSMType = "node"
		case "W":
			it.OSMType = "way"
		case "R":
			it.OSMType = "relation"
		default:
			it.OSMType = strings.ToLower(osmType)
		}
		_ = json.Unmarshal([]byte(nameJSON), &it.NameDetails)
		_ = json.Unmarshal([]byte(extratagsJSON), &it.ExtraTags)
		it.PolygonGeoJSON = poly
		if p.AddressDetails {
			rows2, err2 := r.fetchAddressRows(ctx, db, it.PlaceID)
			if err2 == nil {
				it.AddressRows = rows2
			}
		}
		out = append(out, &it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// fetchAddressRows 读取地址行（按常见 nominatim 表 place_addressline）。
func (r *searchRepo) fetchAddressRows(ctx context.Context, db *sql.DB, placeID int64) ([]biz.AddressRowItem, error) {
	q := `
SELECT
  COALESCE(addresstype, '') AS component,
  COALESCE(address, '') AS name,
  COALESCE(admin_level, 0) AS admin_level,
  COALESCE(cached_rank_address, 0) AS rank
FROM place_addressline
WHERE place_id = $1
ORDER BY cached_rank_address ASC`
	rows, err := db.QueryContext(ctx, q, placeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []biz.AddressRowItem
	for rows.Next() {
		var item biz.AddressRowItem
		if err := rows.Scan(&item.Component, &item.Name, &item.AdminLevel, &item.Rank); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func pqArray(ss []string) any { return "{" + strings.Join(ss, ",") + "}" }

// zoomToMaxRank 将 zoom 映射到 rank 上限，基于 0..18 的离散表。
func zoomToMaxRank(zoom int) int {
	// 近似对齐：低 zoom 聚合到更上层行政等级，高 zoom 允许更细粒度
	table := []int{2, 3, 4, 6, 8, 10, 12, 14, 15, 16, 18, 20, 22, 24, 26, 27, 28, 29, 30}
	if zoom < 0 {
		zoom = 0
	}
	if zoom > 18 {
		zoom = 18
	}
	return table[zoom]
}

// mapFeatureTypeToRankRange 映射 featureType 到 rank_address 的 [min,max]，对齐 v1。
func mapFeatureTypeToRankRange(ft string) (int, int) {
	switch strings.ToLower(ft) {
	case "country":
		return 4, 4
	case "state":
		return 8, 8
	case "city":
		return 14, 16
	case "town":
		return 16, 18
	case "village":
		return 18, 20
	case "hamlet":
		return 20, 22
	case "suburb":
		return 20, 22
	case "neighbourhood", "neighborhood":
		return 22, 26
	case "settlement":
		return 8, 20
	default:
		return 0, int(^uint(0) >> 1)
	}
}

// mapLayersToClasses 将常见 layers（与 Nominatim 兼容）映射为 placex.class 集合
func mapLayersToClasses(layers []string) map[string]struct{} {
	classes := make(map[string]struct{})
	for _, layer := range layers {
		switch strings.ToLower(strings.TrimSpace(layer)) {
		case "address":
			// 地址/行政/道路
			for _, c := range []string{"place", "boundary", "highway", "administrative"} {
				classes[c] = struct{}{}
			}
		case "poi":
			// 兴趣点类目
			for _, c := range []string{"amenity", "shop", "tourism", "leisure", "craft", "office", "aeroway", "aerialway", "sport", "healthcare"} {
				classes[c] = struct{}{}
			}
		case "railway":
			classes["railway"] = struct{}{}
			classes["public_transport"] = struct{}{}
		case "natural":
			for _, c := range []string{"natural", "waterway", "landuse", "geological"} {
				classes[c] = struct{}{}
			}
		case "manmade", "man_made":
			classes["man_made"] = struct{}{}
			classes["power"] = struct{}{}
			classes["industrial"] = struct{}{}
		case "transport":
			for _, c := range []string{"aeroway", "aerialway", "highway", "railway", "public_transport"} {
				classes[c] = struct{}{}
			}
		case "boundaries", "boundary":
			classes["boundary"] = struct{}{}
		}
	}
	return classes
}
