package server

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	v1 "nominatim-go/api/nominatim/v1"
	"strings"

	"github.com/go-kratos/kratos/v2/transport/http"
)

// geoJSON structures
type geoJSONFeature struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	BBox       []float64      `json:"bbox,omitempty"`
	Geometry   map[string]any `json:"geometry"`
}

type geoJSONFC struct {
	Type     string           `json:"type"`
	Licence  string           `json:"licence,omitempty"`
	Features []geoJSONFeature `json:"features"`
}

func asGeoJSONPlaces(val any) ([]*v1.Place, bool) {
	switch t := val.(type) {
	case *v1.SearchResponse:
		return t.GetResults(), true
	case *v1.LookupResponse:
		return t.GetResults(), true
	case *v1.ReverseResponse:
		if t.GetResult() == nil {
			return []*v1.Place{}, true
		}
		return []*v1.Place{t.GetResult()}, true
	default:
		return nil, false
	}
}

func encodeGeoJSON(w http.ResponseWriter, r *http.Request, v any) error {
	places, ok := asGeoJSONPlaces(v)
	if !ok {
		return http.DefaultResponseEncoder(w, r, v)
	}
	fc := geoJSONFC{Type: "FeatureCollection"}
	for _, p := range places {
		if p == nil {
			continue
		}
		props := map[string]any{
			"place_id":     p.GetPlaceId(),
			"osm_type":     p.GetOsmType(),
			"osm_id":       p.GetOsmId(),
			"category":     p.GetCategory(),
			"type":         p.GetType(),
			"display_name": p.GetDisplayName(),
			"importance":   p.GetImportance(),
		}
		var bbox []float64
		if b := p.GetBoundingbox(); b != nil {
			bbox = []float64{b.GetWest(), b.GetSouth(), b.GetEast(), b.GetNorth()}
		}
		geom := map[string]any{"type": "Point", "coordinates": []float64{p.GetCentroid().GetLon(), p.GetCentroid().GetLat()}}
		if gj := p.GetPolygonGeojson(); gj != "" {
			var parsed any
			if err := json.Unmarshal([]byte(gj), &parsed); err == nil {
				if m, ok := parsed.(map[string]any); ok {
					geom = m
				}
			}
		}
		fc.Features = append(fc.Features, geoJSONFeature{Type: "Feature", Properties: props, BBox: bbox, Geometry: geom})
	}
	if cb := r.URL.Query().Get("json_callback"); cb != "" {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		b, _ := json.Marshal(fc)
		if _, err := w.Write([]byte(cb + "(")); err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
		_, err := w.Write([]byte(")"))
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	return enc.Encode(fc)
}

// geocodejson structures
type geocodeJSON struct {
	Type      string           `json:"type"`
	Geocoding map[string]any   `json:"geocoding"`
	Features  []map[string]any `json:"features"`
}

func encodeGeocodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	places, ok := asGeoJSONPlaces(v)
	if !ok {
		return http.DefaultResponseEncoder(w, r, v)
	}
	out := geocodeJSON{
		Type: "FeatureCollection",
		Geocoding: map[string]any{
			"version": "0.1.0",
		},
	}
	for _, p := range places {
		if p == nil {
			continue
		}
		props := map[string]any{
			"geocoding": map[string]any{
				"type":  p.GetType(),
				"label": p.GetDisplayName(),
				"name":  p.GetDisplayName(),
			},
		}
		geom := map[string]any{"type": "Point", "coordinates": []float64{p.GetCentroid().GetLon(), p.GetCentroid().GetLat()}}
		if gj := p.GetPolygonGeojson(); gj != "" {
			var parsed any
			if err := json.Unmarshal([]byte(gj), &parsed); err == nil {
				if m, ok := parsed.(map[string]any); ok {
					geom = m
				}
			}
		}
		out.Features = append(out.Features, map[string]any{
			"type":       "Feature",
			"properties": props,
			"geometry":   geom,
		})
	}
	if cb := r.URL.Query().Get("json_callback"); cb != "" {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		b, _ := json.Marshal(out)
		if _, err := w.Write([]byte(cb + "(")); err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
		_, err := w.Write([]byte(")"))
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(out)
}

// Minimal XML output (compact)
type xmlSearchResults struct {
	XMLName         xml.Name   `xml:"searchresults"`
	MoreURL         string     `xml:"more_url,attr,omitempty"`
	ExcludePlaceIds string     `xml:"exclude_place_ids,attr,omitempty"`
	Place           []xmlPlace `xml:"place"`
}
type xmlReverse struct {
	XMLName xml.Name `xml:"reversegeocode"`
	Result  xmlPlace `xml:"result"`
}
type xmlPlace struct {
	XMLName     xml.Name `xml:"result"`
	PlaceID     int64    `xml:"place_id,attr"`
	OsmType     string   `xml:"osm_type,attr"`
	OsmID       string   `xml:"osm_id,attr"`
	DisplayName string   `xml:"display_name,attr"`
	Class       string   `xml:"class,attr"`
	Type        string   `xml:"type,attr"`
	Importance  float64  `xml:"importance,attr"`
	Lat         float64  `xml:"lat,attr"`
	Lon         float64  `xml:"lon,attr"`
	BoundingBox string   `xml:"boundingbox,attr,omitempty"`
}

func toXMLPlace(p *v1.Place) xmlPlace {
	bbox := ""
	if b := p.GetBoundingbox(); b != nil {
		bbox = fmt.Sprintf("%g,%g,%g,%g", b.GetSouth(), b.GetNorth(), b.GetWest(), b.GetEast())
	}
	lat, lon := 0.0, 0.0
	if c := p.GetCentroid(); c != nil {
		lat, lon = c.GetLat(), c.GetLon()
	}
	return xmlPlace{
		PlaceID:     p.GetPlaceId(),
		OsmType:     p.GetOsmType(),
		OsmID:       p.GetOsmId(),
		DisplayName: p.GetDisplayName(),
		Class:       p.GetCategory(),
		Type:        p.GetType(),
		Importance:  p.GetImportance(),
		Lat:         lat,
		Lon:         lon,
		BoundingBox: bbox,
	}
}

// buildMoreURL 构造 /search 的 more_url，带上白名单参数并设置 exclude_place_ids 与 format=xml
func buildMoreURL(r *http.Request, joinedPlaceIDs string) string {
	if r == nil {
		return ""
	}
	keep := []string{
		"q", "amenity", "street", "city", "county", "state", "country", "postalcode",
		"countrycodes",
		"viewbox", "bounded",
		"featureType", "layer",
		"addressdetails", "extratags", "namedetails", "entrances",
		"polygon_geojson", "polygon_kml", "polygon_svg", "polygon_text", "polygon_threshold",
		"limit", "offset", "dedupe",
		"accept-language",
	}
	q := url.Values{}
	src := r.URL.Query()
	for _, k := range keep {
		if vals, ok := src[k]; ok {
			for _, v := range vals {
				if v != "" {
					q.Add(k, v)
				}
			}
		}
	}
	q.Set("exclude_place_ids", joinedPlaceIDs)
	q.Set("format", "xml")
	u := url.URL{Path: "/search"}
	u.RawQuery = q.Encode()
	return u.String()
}

func encodeXML(w http.ResponseWriter, r *http.Request, v any) error {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	switch t := v.(type) {
	case *v1.SearchResponse:
		xr := xmlSearchResults{}
		ids := make([]string, 0, len(t.GetResults()))
		for _, p := range t.GetResults() {
			if p != nil {
				xr.Place = append(xr.Place, toXMLPlace(p))
				ids = append(ids, fmt.Sprintf("%d", p.GetPlaceId()))
			}
		}
		if len(ids) > 0 {
			joined := strings.Join(ids, ",")
			xr.MoreURL = buildMoreURL(r, joined)
			xr.ExcludePlaceIds = joined
		}
		return enc.Encode(xr)
	case *v1.LookupResponse:
		xr := xmlSearchResults{}
		for _, p := range t.GetResults() {
			if p != nil {
				xr.Place = append(xr.Place, toXMLPlace(p))
			}
		}
		return enc.Encode(xr)
	case *v1.ReverseResponse:
		if t.GetResult() == nil {
			return enc.Encode(xmlReverse{})
		}
		return enc.Encode(xmlReverse{Result: toXMLPlace(t.GetResult())})
	default:
		return http.DefaultResponseEncoder(w, r, v)
	}
}
