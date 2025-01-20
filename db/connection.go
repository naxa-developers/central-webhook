package db

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InitPool(ctx context.Context, log *slog.Logger, dbUri string) (*pgxpool.Pool, error) {
	// get a connection pool
	dbPool, err := pgxpool.New(ctx, dbUri)
	if err != nil {
		log.Error("error connection to DB", "err", err)
		return nil, err
	}
	if err = dbPool.Ping(ctx); err != nil {
		log.Error("error pinging DB", "err", err)
		return nil, err
	}

	return dbPool, nil
}
