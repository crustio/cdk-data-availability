package synchronizer

import (
	"context"
	"time"

	"github.com/0xPolygon/cdk-data-availability/db"
	"github.com/0xPolygon/cdk-data-availability/offchaindata"
	"github.com/0xPolygon/cdk-validium-node/log"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v4"
)

const dbTimeout = 2 * time.Second

func getStartBlock(db *db.DB) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	start, err := db.GetLastProcessedBlock(ctx)
	if err != nil {
		log.Errorf("error retrieving last processed block, starting from 0: %v", err)
	}
	if start > 0 {
		start = start - 1 // since a block may have been partially processed
	}
	return start, err
}

func setStartBlock(db *db.DB, block uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	var (
		dbTx pgx.Tx
		err  error
	)
	if dbTx, err = db.BeginStateTransaction(ctx); err != nil {
		return err
	}
	err = db.StoreLastProcessedBlock(ctx, block, dbTx)
	if err != nil {
		return err
	}
	if err = dbTx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func rewindStartBlock(db *db.DB, lca uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	rewind, err := db.ResetLastProcessedBlock(ctx, lca)
	if err != nil {
		return err
	}
	if rewind > 0 {
		log.Infof("rewound %d blocks", rewind)
	}
	return nil
}

func exists(db *db.DB, key common.Hash) bool {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	return db.Exists(ctx, key)
}

func store(db *db.DB, block uint64, data []offchaindata.OffChainData) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()
	var (
		dbTx pgx.Tx
		err  error
	)
	if dbTx, err = db.BeginStateTransaction(ctx); err != nil {
		return err
	}
	if err = db.StoreOffChainData(ctx, data, dbTx); err != nil {
		rollback(ctx, err, dbTx)
		return err
	}
	if err = db.StoreLastProcessedBlock(ctx, block, dbTx); err != nil {
		rollback(ctx, err, dbTx)
		return err
	}
	if err = dbTx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func rollback(ctx context.Context, err error, dbTx pgx.Tx) {
	if txErr := dbTx.Rollback(ctx); txErr != nil {
		log.Errorf("failed to roll back transaction after error %v : %v", err, txErr)
	}
}
