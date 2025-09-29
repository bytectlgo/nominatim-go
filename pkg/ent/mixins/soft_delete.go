package mixins

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

type SoftDeleteMixin struct {
	mixin.Schema
}

func (SoftDeleteMixin) Fields() []ent.Field {
	return []ent.Field{
		// 删除时间
		field.Int64("deleted_at").
			Comment("删除时间").
			Optional().Default(0),
		// 删除标识
		field.Int8("is_deleted").
			Comment("删除标识").
			Optional().
			Default(0),
	}
}
