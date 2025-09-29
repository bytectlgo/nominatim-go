package service

import (
	"context"
	v1 "nominatim-go/api/nominatim/v1"
	"nominatim-go/internal/biz"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// NominatimService 实现 RPC 与 HTTP 入口，调用 biz 层。
type NominatimService struct {
	v1.UnimplementedNominatimServiceServer
	log    *log.Helper
	search *biz.SearchUsecase
}

var serviceStartTime = time.Now()

func NewNominatimService(logger log.Logger, search *biz.SearchUsecase) *NominatimService {
	return &NominatimService{log: log.NewHelper(logger), search: search}
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
		results = append(results, mapPlace(it))
	}
	return &v1.SearchResponse{Results: results}, nil
}

func (s *NominatimService) Reverse(ctx context.Context, req *v1.ReverseRequest) (*v1.ReverseResponse, error) {
	s.log.WithContext(ctx).Infof("Reverse lat=%f lon=%f", req.GetLat(), req.GetLon())
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
	})
	if err != nil {
		return nil, err
	}
	if it == nil {
		return &v1.ReverseResponse{}, nil
	}
	return &v1.ReverseResponse{Result: mapPlace(it)}, nil
}

func (s *NominatimService) Lookup(ctx context.Context, req *v1.LookupRequest) (*v1.LookupResponse, error) {
	s.log.WithContext(ctx).Infof("Lookup ids=%v", req.GetOsmIds())
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
		results = append(results, mapPlace(p))
	}
	return &v1.LookupResponse{Results: results}, nil
}

func (s *NominatimService) Status(ctx context.Context, _ *v1.StatusRequest) (*v1.StatusResponse, error) {
	uptime := time.Since(serviceStartTime).Round(time.Second).String()
	return &v1.StatusResponse{Version: "dev", DbStatus: "unknown", Uptime: uptime}, nil
}

func mapPlace(it *biz.SearchPlace) *v1.Place {
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
	return &v1.Place{
		PlaceId:        it.PlaceID,
		OsmId:          it.OSMID,
		OsmType:        it.OSMType,
		Category:       it.Category,
		Type:           it.Type,
		DisplayName:    it.Name,
		Importance:     it.Importance,
		Centroid:       &v1.Point{Lat: it.Lat, Lon: it.Lon},
		Boundingbox:    &v1.BoundingBox{South: it.BBoxSouth, North: it.BBoxNorth, West: it.BBoxWest, East: it.BBoxEast},
		AddressRows:    addrRows,
		PolygonGeojson: it.PolygonGeoJSON,
		Extratags:      it.ExtraTags,
		Namedetails:    it.NameDetails,
	}
}
