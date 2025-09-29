package server

import (
	hv1 "nominatim-go/api/helloworld/v1"
	v1 "nominatim-go/api/nominatim/v1"
	"nominatim-go/internal/conf"
	"nominatim-go/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// 编码相关逻辑已拆分到 encoders.go

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, greeter *service.GreeterService, nominatim *service.NominatimService, logger log.Logger) *http.Server {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			logging.Server(logger),
		),
		http.ResponseEncoder(func(w http.ResponseWriter, r *http.Request, v any) error {
			if r != nil {
				q := r.URL.Query().Get("format")
				if q == "geojson" {
					return encodeGeoJSON(w, r, v)
				} else if q == "geocodejson" {
					return encodeGeocodeJSON(w, r, v)
				} else if q == "xml" {
					return encodeXML(w, r, v)
				}
			}
			return http.DefaultResponseEncoder(w, r, v)
		}),
		http.RequestDecoder(http.DefaultRequestDecoder),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	hv1.RegisterGreeterHTTPServer(srv, greeter)
	v1.RegisterNominatimServiceHTTPServer(srv, nominatim)
	return srv
}
