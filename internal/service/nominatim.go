package service

import (
	"context"
	v1 "nominatim-go/api/nominatim/v1"
	"nominatim-go/internal/biz"
	"nominatim-go/internal/data"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	kratostransport "github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/protobuf/types/known/emptypb"
)

// NominatimService 实现 RPC 与 HTTP 入口，调用 biz 层。
type NominatimService struct {
	v1.UnimplementedNominatimServiceServer
	log    *log.Helper
	search *biz.SearchUsecase
	data   *data.Data
}

var serviceStartTime = time.Now()
var licenceText = func() string {
	if v := strings.TrimSpace(os.Getenv("NOMINATIM_LICENCE")); v != "" {
		return v
	}
	return "Data © OpenStreetMap contributors"
}()
var serviceVersion = func() string {
	if v := strings.TrimSpace(os.Getenv("NOMINATIM_VERSION")); v != "" {
		return v
	}
	return "dev"
}()

func NewNominatimService(logger log.Logger, search *biz.SearchUsecase, data *data.Data) *NominatimService {
	return &NominatimService{log: log.NewHelper(logger), search: search, data: data}
}

func (s *NominatimService) Search(ctx context.Context, req *v1.SearchRequest) (*v1.SearchResponse, error) {
	s.log.WithContext(ctx).Infof("Search q=%s", req.GetQ())
	// 默认分页与上限
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	offset := int(req.GetOffset())
	if offset < 0 {
		offset = 0
	}
	// 兼容：若 accept_language 为空，则回退到 HTTP Header: Accept-Language
	acceptLang := req.GetAcceptLanguage()
	if strings.TrimSpace(acceptLang) == "" {
		if tr, ok := kratostransport.FromServerContext(ctx); ok {
			// 使用通用接口获取请求头
			if headerer, hok := tr.(interface{ RequestHeader() map[string]string }); hok && headerer.RequestHeader() != nil {
				if v, ok := headerer.RequestHeader()["Accept-Language"]; ok && v != "" {
					acceptLang = v
				}
			}
		}
	}
	// 兼容：解析查询参数 viewbox=left,top,right,bottom（当结构体为空时回填）
	vleft, vtop, vright, vbottom := req.GetViewbox().GetLeft(), req.GetViewbox().GetTop(), req.GetViewbox().GetRight(), req.GetViewbox().GetBottom()
	if vleft == 0 && vtop == 0 && vright == 0 && vbottom == 0 {
		if tr, ok := kratostransport.FromServerContext(ctx); ok {
			if urlGetter, uok := tr.(interface{ RequestQuery(key string) string }); uok {
				if vb := strings.TrimSpace(urlGetter.RequestQuery("viewbox")); vb != "" {
					parts := strings.Split(vb, ",")
					if len(parts) == 4 {
						vleft = parseFloatSafe(parts[0])
						vtop = parseFloatSafe(parts[1])
						vright = parseFloatSafe(parts[2])
						vbottom = parseFloatSafe(parts[3])
					}
				}
			}
		}
	}
	// 规范化 countrycodes：去空格、小写
	cc := strings.ToLower(strings.ReplaceAll(req.GetCountrycodes(), " ", ""))
	items, err := s.search.Search(ctx, biz.SearchParams{
		Q:                req.GetQ(),
		CountryCodes:     cc,
		Limit:            limit,
		Offset:           offset,
		AddressDetails:   req.GetAddressdetails(),
		AcceptLanguage:   acceptLang,
		FeatureType:      req.GetFeaturetype(),
		Dedupe:           req.GetDedupe(),
		Bounded:          req.GetBounded(),
		PolygonGeoJSON:   req.GetPolygonGeojson(),
		PolygonThreshold: req.GetPolygonThreshold(),
		ExtraTags:        req.GetExtratags(),
		NameDetails:      req.GetNamedetails(),
		ExcludePlaceIDs:  req.GetExcludePlaceIds(),
		Layers:           splitCSV(req.GetLayer()),
		ViewBoxLeft:      vleft,
		ViewBoxTop:       vtop,
		ViewBoxRight:     vright,
		ViewBoxBottom:    vbottom,
	})
	if err != nil {
		return nil, err
	}
	results := make([]*v1.Place, 0, len(items))
	for _, it := range items {
		results = append(results, mapPlaceWithLocale(it, req.GetAcceptLanguage()))
	}
	return &v1.SearchResponse{Results: results}, nil
}

func (s *NominatimService) Reverse(ctx context.Context, req *v1.ReverseRequest) (*v1.ReverseResponse, error) {
	it, err := s.search.Reverse(ctx, biz.ReverseParams{
		Lat:              req.GetLat(),
		Lon:              req.GetLon(),
		Zoom:             int(req.GetZoom()),
		AddressDetails:   req.GetAddressdetails(),
		AcceptLanguage:   req.GetAcceptLanguage(),
		PolygonGeoJSON:   req.GetPolygonGeojson(),
		PolygonThreshold: req.GetPolygonThreshold(),
		ExtraTags:        req.GetExtratags(),
		NameDetails:      req.GetNamedetails(),
		Layers:           splitCSV(req.GetLayer()),
	})
	if err != nil {
		return nil, err
	}
	if it == nil {
		return &v1.ReverseResponse{}, nil
	}
	return &v1.ReverseResponse{Result: mapPlaceWithLocale(it, req.GetAcceptLanguage())}, nil
}

func (s *NominatimService) Lookup(ctx context.Context, req *v1.LookupRequest) (*v1.LookupResponse, error) {
	it, err := s.search.Lookup(ctx, biz.LookupParams{
		OSMIDs:           req.GetOsmIds(),
		AddressDetails:   req.GetAddressdetails(),
		AcceptLanguage:   req.GetAcceptLanguage(),
		PolygonGeoJSON:   req.GetPolygonGeojson(),
		PolygonThreshold: req.GetPolygonThreshold(),
		ExtraTags:        req.GetExtratags(),
		NameDetails:      req.GetNamedetails(),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*v1.Place, 0, len(it))
	for _, p := range it {
		results = append(results, mapPlaceWithLocale(p, req.GetAcceptLanguage()))
	}
	return &v1.LookupResponse{Results: results}, nil
}

func (s *NominatimService) Status(ctx context.Context, _ *v1.StatusRequest) (*v1.StatusResponse, error) {
	uptime := time.Since(serviceStartTime).Round(time.Second).String()
	dbStatus := "unknown"
	if s.data != nil && s.data.SQLDB() != nil {
		if err := s.data.SQLDB().PingContext(ctx); err == nil {
			dbStatus = "ok"
		} else {
			dbStatus = "unavailable"
		}
	}
	return &v1.StatusResponse{Version: serviceVersion, DbStatus: dbStatus, Uptime: uptime}, nil
}

func (s *NominatimService) Details(ctx context.Context, req *v1.DetailsRequest) (*v1.DetailsResponse, error) {
	if strings.TrimSpace(os.Getenv("NOMINATIM_ENABLE_DETAILS")) == "0" {
		return &v1.DetailsResponse{}, nil
	}
	id := strings.TrimSpace(req.GetOsmId())
	if id == "" {
		return &v1.DetailsResponse{}, nil
	}
	res, err := s.search.Lookup(ctx, biz.LookupParams{
		OSMIDs:           []string{id},
		AddressDetails:   req.GetAddressdetails(),
		AcceptLanguage:   req.GetAcceptLanguage(),
		PolygonGeoJSON:   true,
		PolygonThreshold: 0,
		ExtraTags:        true,
		NameDetails:      true,
	})
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return &v1.DetailsResponse{}, nil
	}
	return &v1.DetailsResponse{Result: mapPlaceWithLocale(res[0], req.GetAcceptLanguage())}, nil
}

func (s *NominatimService) Deletable(ctx context.Context, _ *emptypb.Empty) (*v1.DeletableResponse, error) {
	if strings.TrimSpace(os.Getenv("NOMINATIM_ENABLE_MAINTENANCE")) == "0" {
		return &v1.DeletableResponse{PlaceIds: []int64{}}, nil
	}
	// 最小实现：当使用 PostgreSQL 时，返回 placex 中 osm 更新可能导致孤立的最近修改对象（示例逻辑：importance 为 NULL 或 0 的若干条）
	ids := []int64{}
	if s.data != nil && s.data.SQLDB() != nil {
		db := s.data.SQLDB()
		rowQuery := `SELECT place_id FROM placex WHERE COALESCE(importance, 0) = 0 ORDER BY place_id DESC LIMIT 100`
		rows, err := db.QueryContext(ctx, rowQuery)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err == nil {
					ids = append(ids, id)
				}
			}
		}
	}
	return &v1.DeletableResponse{PlaceIds: ids}, nil
}

func (s *NominatimService) Polygons(ctx context.Context, _ *emptypb.Empty) (*v1.PolygonsResponse, error) {
	if strings.TrimSpace(os.Getenv("NOMINATIM_ENABLE_MAINTENANCE")) == "0" {
		return &v1.PolygonsResponse{PlaceIds: []int64{}}, nil
	}
	// 最小实现：当使用 PostgreSQL 时，返回 polygon 无效的 place_id 列表（最多 100 条）
	ids := []int64{}
	if s.data != nil && s.data.SQLDB() != nil {
		db := s.data.SQLDB()
		rowQuery := `SELECT place_id FROM placex WHERE polygon IS NOT NULL AND NOT ST_IsValid(polygon) LIMIT 100`
		rows, err := db.QueryContext(ctx, rowQuery)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err == nil {
					ids = append(ids, id)
				}
			}
		}
	}
	return &v1.PolygonsResponse{PlaceIds: ids}, nil
}

func mapPlaceWithLocale(it *biz.SearchPlace, acceptLanguage string) *v1.Place {
	// address rows
	addrRows := make([]*v1.AddressRow, 0, len(it.AddressRows))
	for _, r := range it.AddressRows {
		addrRows = append(addrRows, &v1.AddressRow{
			Name:       r.Name,
			Type:       r.Component,
			AdminLevel: r.AdminLevel,
			Rank:       r.Rank,
		})
	}
	// 选择 display name：
	// 1) name:<lang-REGION> 精确匹配；2) name:<lang> 回退；3) int_name；4) 基础 name
	display := it.Name
	langs := parseAcceptLanguages(acceptLanguage)
	if len(langs) > 0 && len(it.NameDetails) > 0 {
		chosen := ""
		// 精确匹配 lang-REGION
		for _, lang := range langs {
			if strings.Contains(lang, "-") {
				if v, ok := it.NameDetails["name:"+lang]; ok && v != "" {
					chosen = v
					break
				}
			}
		}
		// 回退到 lang 主语言
		if chosen == "" {
			for _, lang := range langs {
				base := lang
				if i := strings.Index(lang, "-"); i >= 0 {
					base = lang[:i]
				}
				if v, ok := it.NameDetails["name:"+base]; ok && v != "" {
					chosen = v
					break
				}
			}
		}
		if chosen == "" {
			if v, ok := it.NameDetails["int_name"]; ok && v != "" {
				chosen = v
			} else if v, ok := it.NameDetails["name"]; ok && v != "" {
				chosen = v
			}
		}
		if chosen != "" {
			display = chosen
		}
	}
	return &v1.Place{
		Licence:        licenceText,
		PlaceId:        it.PlaceID,
		OsmId:          it.OSMID,
		OsmType:        it.OSMType,
		Category:       it.Category,
		Type:           it.Type,
		DisplayName:    display,
		Importance:     it.Importance,
		Centroid:       &v1.Point{Lat: it.Lat, Lon: it.Lon},
		Boundingbox:    &v1.BoundingBox{South: it.BBoxSouth, North: it.BBoxNorth, West: it.BBoxWest, East: it.BBoxEast},
		AddressRows:    addrRows,
		PolygonGeojson: it.PolygonGeoJSON,
		Extratags:      it.ExtraTags,
		Namedetails:    it.NameDetails,
	}
}

func parseAcceptLanguages(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	langs := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 取语言部分（去掉 ;q=）
		if i := strings.Index(p, ";"); i >= 0 {
			p = p[:i]
		}
		langs = append(langs, p)
	}
	return langs
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseFloatSafe(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// 使用 ParseFloat 前先替换可能的空格
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	return 0
}
