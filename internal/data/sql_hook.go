package data

import (
	"context"
	"time"

	"github.com/fatih/color"
)

type Hooks struct{}

func (h *Hooks) Before(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	return context.WithValue(ctx, "begin", time.Now()), nil
}

func (h *Hooks) After(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	begin := ctx.Value("begin").(time.Time)
	d := time.Since(begin)
	if d > 500*time.Millisecond {
		color.Red("%v slow  sql: %s %q .took: %s\n", time.Now().Format(time.RFC3339), query, args, d)
	}
	//color.Green("%v slow  sql: %s %q .took: %s\n", time.Now().Format(time.RFC3339), query, args, d)
	return ctx, nil
}
