package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// Helloworld holds the schema definition for the Helloworld entity.
type Helloworld struct {
	ent.Schema
}

// Fields of the Helloworld.
func (Helloworld) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").Unique(),
		field.String("name").Default("unknown"),
	}
	return nil
}

// Edges of the Helloworld.
func (Helloworld) Edges() []ent.Edge {
	return nil
}
