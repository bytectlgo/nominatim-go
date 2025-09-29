package biz

import (
	"github.com/go-kratos/kratos/v2/errors"
)

var (
	BadRequest     = "BAD_REQUEST"
	Unauthorized   = "UNAUTHORIZED"
	InternalServer = "INTERNAL_SERVER"
	NotFound       = "NOT_FOUND"
	Conflict       = "CONFLICT"
)

const (
	OrderDesc = "desc"
	OrderAsc  = "asc"
)

var (
	ErrInternalServer = errors.New(500, "INTERNAL_SERVER", "internal server error")
)

const (
	HiddenSecret = "*****"
)

type FindByPageCond struct {
	PageNum   int
	PageSize  int
	WithCount bool
	// 模糊搜索
	Like string
}
