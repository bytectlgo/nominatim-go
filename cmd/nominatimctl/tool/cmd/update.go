package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	updMode string // replication | append
	updPBF  string // for append mode
)

// updateCmd performs diff updates (replication) or append from a new PBF
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "增量更新：replication（首选）或使用 PBF 追加导入",
	RunE: func(cmd *cobra.Command, args []string) error {
		dsn := os.Getenv("PG_DSN")
		if dsn == "" {
			return fmt.Errorf("PG_DSN 未设置")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
		defer cancel()
		switch updMode {
		case "replication":
			// 依赖系统有 nominatim CLI 可用（官方容器或宿主安装）
			if err := run(ctx, "nominatim", "replication", "--init"); err != nil {
				return fmt.Errorf("执行 replication --init 失败：%w", err)
			}
			if err := run(ctx, "nominatim", "replication", "--catch-up"); err != nil {
				return fmt.Errorf("执行 replication --catch-up 失败：%w", err)
			}
			return nil
		case "append":
			if updPBF == "" {
				return fmt.Errorf("append 模式需要 --pbf")
			}
			osm := osm2pgsql
			if osm == "" {
				osm = "osm2pgsql"
			}
			return run(ctx, osm, "--append", "--slim", "--hstore", "--multi-geometry", "--database", dsn, updPBF)
		default:
			return fmt.Errorf("未知的 --mode：%s（可选 replication|append）", updMode)
		}
	},
}

func init() {
	updateCmd.Flags().StringVar(&updMode, "mode", "replication", "更新模式：replication|append")
	updateCmd.Flags().StringVar(&updPBF, "pbf", "", "append 模式的 PBF 路径")
	rootCmd.AddCommand(updateCmd)
}
