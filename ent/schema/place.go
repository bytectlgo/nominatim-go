package schema

import (
	"nominatim-go/pkg/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Place 对应 Nominatim 的地理对象（对齐 proto 字段）。
type Place struct {
	ent.Schema
}

// Mixin returns mixin definitions.
func (Place) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

// Fields of the Place.
func (Place) Fields() []ent.Field {
	return []ent.Field{
		// 内部 place_id（主键）
		field.Int64("place_id").Unique(),
		// 许可证
		field.String("licence").Optional(),
		// OSM 基础属性
		field.String("osm_id"),
		field.String("osm_type"), // node/way/relation
		field.String("category").Optional(),
		field.String("type").Optional(),
		// 重要性
		field.Float("importance").Optional(),
		// 展示名
		field.String("display_name").Optional(),
		// 质心坐标
		field.Float("lat"),
		field.Float("lon"),
		// 边界框 south, north, west, east
		field.Float("bbox_south").Optional(),
		field.Float("bbox_north").Optional(),
		field.Float("bbox_west").Optional(),
		field.Float("bbox_east").Optional(),
		// 图标
		field.String("icon").Optional(),
		// 额外标签与名称详情（JSON）
		field.JSON("extratags", map[string]string{}).Optional(),
		field.JSON("namedetails", map[string]string{}).Optional(),
		// 多边形 GeoJSON 文本
		field.String("polygon_geojson").Optional(),
	}
}

// Edges of the Place.
func (Place) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("address_rows", AddressRow.Type),
	}
}
