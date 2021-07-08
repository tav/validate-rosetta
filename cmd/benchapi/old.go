// Copyright 2021 Coinbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/coinbase/rosetta-cli/configuration"
	"github.com/coinbase/rosetta-sdk-go/fetcher"
	"github.com/coinbase/rosetta-sdk-go/parser"
	"github.com/coinbase/rosetta-sdk-go/storage/database"
	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/neilotoole/errgroup"
	"github.com/tav/validate-rosetta/log"
)

type OldRunner struct {
	db      database.Database
	fetcher *fetcher.Fetcher
	network *types.NetworkIdentifier
	parser  *parser.Parser
}

func (r *OldRunner) Run(ctx context.Context, cfg RunConfig) error {
	if err := r.init(ctx, cfg.dir); err != nil {
		log.Errorf("Failed to initialize: %s", err)
		return err
	}
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := r.logProgress(ctx, cfg.start); err != nil {
			log.Errorf("Failed to log progress: %s", err)
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := r.processBlocks(ctx); err != nil {
			log.Errorf("Failed to process blocks: %s", err)
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := r.reconcileActive(ctx); err != nil {
			log.Errorf("Failed to reconcile active accounts: %s", err)
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := r.reconcileInactive(ctx); err != nil {
			log.Errorf("Failed to reconcile inactive accounts: %s", err)
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := r.syncBlocks(ctx, cfg.target); err != nil {
			log.Errorf("Failed to sync blocks: %s", err)
			return err
		}
		return nil
	})
	err := g.Wait()
	if err := r.cleanup(); err != nil {
		log.Errorf("Failed to cleanup: %s", err)
		return err
	}
	return err
}

func (r *OldRunner) cleanup() error {
	if r.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return r.db.Close(ctx)
	}
	return nil
}

func (r *OldRunner) init(ctx context.Context, dir string) error {
	if err := r.initDB(ctx, dir); err != nil {
		return err
	}
	if err := r.initFetcher(ctx); err != nil {
		return err
	}
	r.initParser()
	return nil
}

func (r *OldRunner) initDB(ctx context.Context, dir string) error {
	db, err := database.NewBadgerDatabase(
		ctx, dir, database.WithoutCompression(),
		database.WithCustomSettings(database.PerformanceBadgerOptions(dir)),
	)
	if err != nil {
		return fmt.Errorf("failed to open database: %s", err)
	}
	r.db = db
	return nil
}

func (r *OldRunner) initFetcher(ctx context.Context) error {
	opts := []fetcher.Option{
		fetcher.WithMaxConnections(configuration.DefaultMaxOnlineConnections),
		fetcher.WithMaxRetries(configuration.DefaultMaxRetries),
		fetcher.WithRetryElapsedTime(time.Duration(0) * time.Second),
		fetcher.WithTimeout(time.Duration(configuration.DefaultTimeout) * time.Second),
	}
	fetcher := fetcher.New("http://localhost:8080", opts...)
	network, _, ferr := fetcher.InitializeAsserter(ctx, &types.NetworkIdentifier{
		Blockchain: "ontology",
		Network:    "testnet",
	})
	if ferr != nil {
		return fmt.Errorf("unable to initialize the asserter: %#v", ferr)
	}
	r.fetcher = fetcher
	r.network = network
	return nil
}

func (r *OldRunner) initParser() {
	r.parser = &parser.Parser{
		Asserter:          r.fetcher.Asserter,
		BalanceExemptions: nil,
		ExemptFunc: func(op *types.Operation) bool {
			return false
		},
	}
}

func (r *OldRunner) logProgress(ctx context.Context, start time.Time) error {
	t := time.NewTicker(time.Second)
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
		}

	}
}

func (r *OldRunner) processBlocks(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
	return nil
}

func (r *OldRunner) reconcileActive(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
	return nil
}

func (r *OldRunner) reconcileInactive(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
	return nil
}

func (r *OldRunner) syncBlocks(ctx context.Context, target int) error {
	for height := 0; height < target; height++ {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
	// Signal to others that we're done
	return nil
}

// func (r *OldRunner) foo(ctx context.Context) {
// 	block := &types.Block{}
// 	changes, err := r.parser.BalanceChanges(ctx, block, false)
// 	if err != nil {
// 		log.Fatalf("Unable to calculated balance changes: %s", err)
// 	}
// 	for _, change := range changes {
// 		_ = change
// 	}
// }

// func benchOld(ctx context.Context, target int64) int64 {
// 	done := int64(0)
// 	for height := int64(0); height < target; height++ {
// 		block, ferr := fetcher.BlockRetry(ctx, network, &types.PartialBlockIdentifier{
// 			Index: types.Int64(height),
// 		})
// 		if ferr != nil {
// 			log.Errorf("Unable to get block at height %d: %#v", height, ferr)
// 			return done
// 		}
// 		accts := map[string]*accountInfo{}
// 		for _, txn := range block.Transactions {
// 			for _, op := range txn.Operations {
// 				key := types.Hash(op.Account) + "|" + types.Hash(op.Amount.Currency)
// 				accts[key] = &accountInfo{
// 					acct:     op.Account,
// 					currency: op.Amount.Currency,
// 				}
// 			}
// 		}
// 		for _, info := range accts {
// 			ident, amounts, _, ferr := fetcher.AccountBalanceRetry(
// 				ctx,
// 				network,
// 				info.acct,
// 				&types.PartialBlockIdentifier{
// 					Index: types.Int64(height),
// 				},
// 				[]*types.Currency{info.currency},
// 			)
// 			if ident.Hash != block.BlockIdentifier.Hash || ident.Index != block.BlockIdentifier.Index {
// 				log.Fatalf(
// 					"Mismatching block identifier for account %#v balance at height %d",
// 					info.acct, height,
// 				)
// 			}
// 			if ferr != nil {
// 				log.Errorf("Unable to get account balance at height %d: %#v", height, ferr)
// 				return done
// 			}
// 			_ = amounts
// 		}
// 		done++
// 		if done%1000 == 0 {
// 			diff := time.Since(start)
// 			log.Infof(
// 				"[PROGRESS] blocks: %d, total time: %s, avg time: %s",
// 				done, diff, diff/time.Duration(done),
// 			)
// 		}
// 	}
// 	return target
// }
