package mixins

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

type TimeMixin struct {
	mixin.Schema
}

func (TimeMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("created_at").Comment("创建时间,unix时间戳").
			Immutable().
			DefaultFunc(func() int64 {
				return time.Now().Unix()
			}),
		field.Int64("updated_at").Comment("更新时间,unix时间戳").
			DefaultFunc(func() int64 {
				return time.Now().Unix()
			}).
			UpdateDefault(func() int64 {
				return time.Now().Unix()
			}),
	}
}
