package mixins

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

type Uint64IdMixin struct {
	mixin.Schema
}

func (Uint64IdMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Uint64("id").Comment("唯一标识"),
	}
}
