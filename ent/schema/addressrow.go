package schema

import (
	"nominatim-go/pkg/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// AddressRow 表示结果中的一条地址组成元素（对齐 proto）。
type AddressRow struct {
	ent.Schema
}

// Mixin returns mixin definitions.
func (AddressRow) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

// Fields of the AddressRow.
func (AddressRow) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Unique(),
		// 由 Edge 自动生成 place 外键（与 Place.id 对应，类型为 int）
		field.String("component"), // type
		field.String("name"),      // 名称
		field.Uint32("admin_level").Optional(),
		field.Uint32("rank").Default(0),
	}
}

// Edges of the AddressRow.
func (AddressRow) Edges() []ent.Edge {
	return []ent.Edge{
		// 由 ent 自动在 AddressRow 表上创建 place_id 外键（int），指向 Place.id
		edge.From("place", Place.Type).Ref("address_rows").Unique().Required(),
	}
}
