package data

import (
	"context"
	"database/sql"
	"encoding/json"
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
		geoJSONSelect = "COALESCE(ST_AsGeoJSON(polygon, 6)::text, '')"
	}

	base := `
SELECT
  place_id, osm_id, osm_type, class, type,
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
	if p.Bounded && (p.ViewBoxLeft != 0 || p.ViewBoxRight != 0 || p.ViewBoxTop != 0 || p.ViewBoxBottom != 0) {
		base += " AND bbox && ST_MakeEnvelope($" + strconv.Itoa(argIdx) + ", $" + strconv.Itoa(argIdx+1) + ", $" + strconv.Itoa(argIdx+2) + ", $" + strconv.Itoa(argIdx+3) + ", 4326)"
		args = append(args, p.ViewBoxLeft, p.ViewBoxBottom, p.ViewBoxRight, p.ViewBoxTop)
		argIdx += 4
	}
	base += " ORDER BY importance DESC NULLS LAST LIMIT $" + strconv.Itoa(argIdx) + " OFFSET $" + strconv.Itoa(argIdx+1)
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
		geoJSONSelect = "COALESCE(ST_AsGeoJSON(polygon, 6)::text, '')"
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
ORDER BY centroid <-> ST_SetSRID(ST_Point($1,$2), 4326)
LIMIT 1`
	row := db.QueryRowContext(ctx, q, p.Lon, p.Lat)
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
       ` + "COALESCE(ST_AsGeoJSON(polygon, 6)::text, '')" + ` AS polygon_geojson
FROM placex
WHERE ` + strings.Join(parts, " OR ")

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
