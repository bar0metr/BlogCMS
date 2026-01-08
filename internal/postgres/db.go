package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct {
	*sql.DB
}

type Options struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PingTimeout     time.Duration
}

func Open(dsn string, opt Options) (*DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if opt.MaxOpenConns <= 0 {
		opt.MaxOpenConns = 10
	}
	if opt.MaxIdleConns < 0 {
		opt.MaxIdleConns = 0
	}
	if opt.MaxIdleConns > opt.MaxOpenConns {
		opt.MaxIdleConns = opt.MaxOpenConns
	}
	if opt.ConnMaxLifetime <= 0 {
		opt.ConnMaxLifetime = 30 * time.Minute
	}
	if opt.ConnMaxIdleTime <= 0 {
		opt.ConnMaxIdleTime = 5 * time.Minute
	}
	if opt.PingTimeout <= 0 {
		opt.PingTimeout = 5 * time.Second
	}

	db.SetMaxOpenConns(opt.MaxOpenConns)
	db.SetMaxIdleConns(opt.MaxIdleConns)
	db.SetConnMaxLifetime(opt.ConnMaxLifetime)
	db.SetConnMaxIdleTime(opt.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), opt.PingTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &DB{DB: db}, nil
}
