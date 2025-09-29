package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
)

var (
	pbfPath   string
	pbfURL    string
	threads   int
	osm2pgsql string
)

// importCmd imports OSM PBF into an existing PostGIS DB that already has Nominatim schema prepared
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "导入 OSM PBF 到 PostGIS（使用 osm2pgsql，仿 Python 工具链入口）",
	RunE: func(cmd *cobra.Command, args []string) error {
		dsn := os.Getenv("PG_DSN")
		if dsn == "" {
			return fmt.Errorf("PG_DSN 未设置，如: 'host=127.0.0.1 user=postgres password=postgres dbname=nominatim port=5432 sslmode=disable'")
		}
		if pbfURL == "" && pbfPath == "" {
			return fmt.Errorf("必须提供 --pbf 或 --pbf-url 其中之一")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
		defer cancel()
		// 1) 可选：下载 PBF
		local := pbfPath
		if local == "" {
			local = "./data.osm.pbf"
			if err := run(ctx, "curl", "-L", "-o", local, pbfURL); err != nil {
				return err
			}
		}
		// 2) 调用 osm2pgsql 进行导入（简化参数，用户可自定义 osm2pgsql 路径和线程数）
		if osm2pgsql == "" {
			osm2pgsql = "osm2pgsql"
		}
		// 典型参数：--create --slim --hstore --multi-geometry
		oargs := []string{"--create", "--slim", "--hstore", "--multi-geometry", "--number-processes", fmt.Sprintf("%d", threads), "--database", dsn, local}
		if err := run(ctx, osm2pgsql, oargs...); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	importCmd.Flags().StringVar(&pbfPath, "pbf", "", "本地 OSM PBF 文件路径")
	importCmd.Flags().StringVar(&pbfURL, "pbf-url", "", "远程 OSM PBF 下载地址")
	importCmd.Flags().IntVar(&threads, "threads", 4, "导入线程数")
	importCmd.Flags().StringVar(&osm2pgsql, "osm2pgsql", "", "osm2pgsql 可执行文件路径，不设置则使用 PATH 中的")
	rootCmd.AddCommand(importCmd)
}

func run(ctx context.Context, bin string, args ...string) error {
	c := exec.CommandContext(ctx, bin, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// pingDB 检查数据库可用性
func pingDB(dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}
