package data

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"nominatim-go/ent"
	"nominatim-go/internal/conf"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/store/go_cache/v4"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/google/wire"
	gocache "github.com/patrickmn/go-cache"
	"github.com/qustavo/sqlhooks/v2"

	_ "github.com/go-sql-driver/mysql"
	// sqlite "github.com/mattn/go-sqlite3"
	"modernc.org/sqlite"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData,
	NewSqlDriver,
	NewGreeterRepo,
	NewSearchRepo,
)

// Data .
type Data struct {
	entClient *ent.Client
	cache     cache.CacheInterface[any]
	conf      *conf.Data
	sqlDrv    *entsql.Driver
}

// DB 返回数据库客户端
func (d *Data) DB() *ent.Client {
	return d.entClient
}

// SQLDB 返回共享的 *sql.DB（由 ent 驱动管理的连接池）
func (d *Data) SQLDB() *sql.DB {
	if d.sqlDrv != nil {
		return d.sqlDrv.DB()
	}
	return nil
}

// Cache 返回缓存客户端
func (d *Data) Cache() cache.CacheInterface[any] {
	return d.cache
}

// NewData .
func NewData(
	c *conf.Data,
	drv *entsql.Driver,
	logger log.Logger) (*Data, func(), error) {
	goCache := gocache.New(5*time.Minute, 10*time.Minute)
	store := go_cache.NewGoCache(goCache)
	cacheManager := cache.New[any](store)
	data := &Data{
		entClient: NewEnt(drv),
		conf:      c,
		cache:     cacheManager,
		sqlDrv:    drv,
	}
	// 设置 debug 模式
	if c.Database.Debug {
		data.entClient = data.entClient.Debug()
	}
	cleanup := func() {
		log.NewHelper(logger).Info("closing the data resources")
		data.entClient.Close()
	}
	// Run the auto migration tool.
	if err := data.entClient.Schema.Create(context.Background()); err != nil {
		return nil, nil, err
	}
	// 已自动迁移，无需额外操作
	return data, cleanup, nil
}

func NewSqlDriver(conf *conf.Data) *entsql.Driver {
	switch conf.Database.Driver {
	case "mysql":
		return newMySqlDriver(conf)
	case "sqlite3":
		return newSqliteDriver(conf)
	case "postgres", "postgresql", "pgx":
		return newPostgresDriver(conf)
	default:
		panic(fmt.Sprintf("unsupported driver: %s", conf.Database.Driver))
	}
}

func newSqliteDriver(conf *conf.Data) *entsql.Driver {
	sql.Register("sqlite3WithHooks", sqlhooks.Wrap(&sqlite.Driver{}, &Hooks{}))
	db, err := sql.Open("sqlite3WithHooks", conf.Database.Source)
	if err != nil {
		panic(err)
	}
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(time.Minute * 10)
	drv := entsql.OpenDB("sqlite3", db)
	return drv
}

func newMySqlDriver(conf *conf.Data) *entsql.Driver {
	sql.Register("mysqlWithHooks", sqlhooks.Wrap(&mysql.MySQLDriver{}, &Hooks{}))
	cfg, err := mysql.ParseDSN(conf.Database.Source)
	if err != nil {
		panic(err)
	}
	DBName := cfg.DBName
	// 去除数据库名字
	cfg.DBName = ""
	tsource := cfg.FormatDSN()
	tdb, err := sql.Open("mysqlWithHooks", tsource)
	if err != nil {
		panic(err)
	}
	defer tdb.Close()
	// 自动创建数据库
	_, err = tdb.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s CHARACTER SET utf8mb4", DBName))
	if err != nil {
		panic(err)
	}

	db, err := sql.Open("mysqlWithHooks", conf.Database.Source)
	if err != nil {
		panic(err)
	}
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(time.Minute * 10)
	drv := entsql.OpenDB("mysql", db)
	return drv
}

func newPostgresDriver(conf *conf.Data) *entsql.Driver {
	sql.Register("pgxWithHooks", sqlhooks.Wrap(&stdlib.Driver{}, &Hooks{}))
	db, err := sql.Open("pgxWithHooks", conf.Database.Source)
	if err != nil {
		panic(err)
	}
	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(time.Minute * 10)
	drv := entsql.OpenDB("pgx", db)
	return drv
}

func NewEnt(drv *entsql.Driver) *ent.Client {
	client := ent.NewClient(ent.Driver(drv))
	return client
}
