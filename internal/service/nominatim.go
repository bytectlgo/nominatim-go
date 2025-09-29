package service

import (
	"context"
	v1 "nominatim-go/api/nominatim/v1"
	"nominatim-go/internal/biz"
	"nominatim-go/internal/data"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
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

func NewNominatimService(logger log.Logger, search *biz.SearchUsecase, data *data.Data) *NominatimService {
	return &NominatimService{log: log.NewHelper(logger), search: search, data: data}
}

func (s *NominatimService) Search(ctx context.Context, req *v1.SearchRequest) (*v1.SearchResponse, error) {
	s.log.WithContext(ctx).Infof("Search q=%s", req.GetQ())
	items, err := s.search.Search(ctx, biz.SearchParams{
		Q:                req.GetQ(),
		CountryCodes:     req.GetCountrycodes(),
		Limit:            int(req.GetLimit()),
		Offset:           int(req.GetOffset()),
		AddressDetails:   req.GetAddressdetails(),
		AcceptLanguage:   req.GetAcceptLanguage(),
		FeatureType:      req.GetFeaturetype(),
		Dedupe:           req.GetDedupe(),
		Bounded:          req.GetBounded(),
		PolygonGeoJSON:   req.GetPolygonGeojson(),
		PolygonThreshold: req.GetPolygonThreshold(),
		ExtraTags:        req.GetExtratags(),
		NameDetails:      req.GetNamedetails(),
		ExcludePlaceIDs:  req.GetExcludePlaceIds(),
		Layers:           splitCSV(req.GetLayer()),
		ViewBoxLeft:      req.GetViewbox().GetLeft(),
		ViewBoxTop:       req.GetViewbox().GetTop(),
		ViewBoxRight:     req.GetViewbox().GetRight(),
		ViewBoxBottom:    req.GetViewbox().GetBottom(),
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
	return &v1.StatusResponse{Version: "dev", DbStatus: dbStatus, Uptime: uptime}, nil
}

func (s *NominatimService) Details(ctx context.Context, req *v1.DetailsRequest) (*v1.DetailsResponse, error) {
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
	return &v1.DeletableResponse{PlaceIds: []int64{}}, nil
}

func (s *NominatimService) Polygons(ctx context.Context, _ *emptypb.Empty) (*v1.PolygonsResponse, error) {
	return &v1.PolygonsResponse{PlaceIds: []int64{}}, nil
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
	// 选择 display name：优先 name:lang，其次 name
	display := it.Name
	langs := parseAcceptLanguages(acceptLanguage)
	if len(langs) > 0 && len(it.NameDetails) > 0 {
		for _, lang := range langs {
			if v, ok := it.NameDetails["name:"+lang]; ok && v != "" {
				display = v
				break
			}
		}
		if display == it.Name {
			if v, ok := it.NameDetails["name"]; ok && v != "" {
				display = v
			}
		}
	}
	return &v1.Place{
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
