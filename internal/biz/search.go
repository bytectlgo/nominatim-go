package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

// SearchPlace 为搜索/查找结果的传输结构。
type SearchPlace struct {
	PlaceID        int64             // 内部 place_id（Nominatim 主键）
	OSMID          string            // OSM 对象 ID
	OSMType        string            // OSM 对象类型：node/way/relation
	Category       string            // 类别（class）
	Type           string            // 类型（type）
	Name           string            // 展示名称（本地化前的基础名）
	Lat            float64           // 纬度
	Lon            float64           // 经度
	Importance     float64           // 重要性分值
	BBoxSouth      float64           // 边界框南
	BBoxNorth      float64           // 边界框北
	BBoxWest       float64           // 边界框西
	BBoxEast       float64           // 边界框东
	ExtraTags      map[string]string // 额外标签（当请求 extratags=true）
	NameDetails    map[string]string // 名称细节（当请求 namedetails=true）
	AddressRows    []AddressRowItem  // 地址行（当请求 addressdetails=true）
	PolygonGeoJSON string            // 多边形 GeoJSON（当请求 polygon_geojson=true）
}

// AddressRowItem 地址行元素。
type AddressRowItem struct {
	Component  string // 组件类型：country/state/city/road/house_number 等
	Name       string // 名称
	AdminLevel uint32 // 行政等级（无则 0）
	Rank       uint32 // 排序等级
}

// SearchRepo 抽象读路径。
type SearchRepo interface {
	SearchPlaces(ctx context.Context, p SearchParams) ([]*SearchPlace, error)
	ReversePlace(ctx context.Context, p ReverseParams) (*SearchPlace, error)
	LookupPlaces(ctx context.Context, p LookupParams) ([]*SearchPlace, error)
}

// SearchUsecase 封装业务逻辑。
type SearchUsecase struct {
	repo SearchRepo  // 数据读取仓库
	log  *log.Helper // 日志
}

func NewSearchUsecase(repo SearchRepo, logger log.Logger) *SearchUsecase {
	return &SearchUsecase{repo: repo, log: log.NewHelper(logger)}
}

// SearchParams 搜索参数集合（与 proto 对齐，部分暂未使用）。
type SearchParams struct {
	Q                string  // 查询关键字
	CountryCodes     string  // 国家代码（逗号分隔）
	Limit            int     // 返回条数上限
	Offset           int     // 偏移量
	AddressDetails   bool    // 是否返回地址明细
	AcceptLanguage   string  // 语言偏好，如 "zh,en"
	FeatureType      string  // 类型过滤（class/type）
	Dedupe           bool    // 结果去重
	Bounded          bool    // 是否限制在视窗内
	PolygonGeoJSON   bool    // 是否返回多边形 GeoJSON
	PolygonThreshold float64 // 多边形简化阈值
	ExtraTags        bool    // 返回 extratags
	NameDetails      bool    // 返回 namedetails
	// 视窗参数
	ViewBoxLeft   float64 // 视窗左（最小经度）
	ViewBoxTop    float64 // 视窗上（最大纬度）
	ViewBoxRight  float64 // 视窗右（最大经度）
	ViewBoxBottom float64 // 视窗下（最小纬度）
}

// ReverseParams 逆地理参数。
type ReverseParams struct {
	Lat              float64 // 纬度
	Lon              float64 // 经度
	Zoom             int     // 缩放级别
	AddressDetails   bool    // 是否返回地址明细
	AcceptLanguage   string  // 语言偏好
	PolygonGeoJSON   bool    // 是否返回多边形 GeoJSON
	PolygonThreshold float64 // 多边形简化阈值
	ExtraTags        bool    // 返回 extratags
	NameDetails      bool    // 返回 namedetails
}

// LookupParams 查找参数。
type LookupParams struct {
	OSMIDs           []string // OSM 对象 ID 列表（N123/W456/R789）
	AddressDetails   bool     // 是否返回地址明细
	AcceptLanguage   string   // 语言偏好
	PolygonGeoJSON   bool     // 是否返回多边形 GeoJSON
	PolygonThreshold float64  // 多边形简化阈值
	ExtraTags        bool     // 返回 extratags
	NameDetails      bool     // 返回 namedetails
}

func (uc *SearchUsecase) Search(ctx context.Context, p SearchParams) ([]*SearchPlace, error) {
	return uc.repo.SearchPlaces(ctx, p)
}

func (uc *SearchUsecase) Reverse(ctx context.Context, p ReverseParams) (*SearchPlace, error) {
	return uc.repo.ReversePlace(ctx, p)
}

func (uc *SearchUsecase) Lookup(ctx context.Context, p LookupParams) ([]*SearchPlace, error) {
	return uc.repo.LookupPlaces(ctx, p)
}
