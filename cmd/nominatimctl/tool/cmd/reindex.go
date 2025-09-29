package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// reindexCmd triggers reindexing/refresh of helper tables
var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "重建索引/刷新辅助表（需要系统有 nominatim CLI 或 SQL 脚本）",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Hour)
		defer cancel()
		// 尝试调用 nominatim 工具
		if err := run(ctx, "nominatim", "refresh-functions"); err == nil {
			_ = run(ctx, "nominatim", "reindex")
			return nil
		}
		return fmt.Errorf("未找到 nominatim CLI，请在容器内运行或提供自定义 SQL 流程")
	},
}

func init() {
	rootCmd.AddCommand(reindexCmd)
}
