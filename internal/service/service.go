package service

import (
	"nominatim-go/internal/biz"

	"github.com/google/wire"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewGreeterService, NewNominatimService, biz.NewSearchUsecase)
