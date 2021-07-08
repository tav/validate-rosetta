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

	"github.com/dgraph-io/badger/v3"
	"github.com/neilotoole/errgroup"
	"github.com/tav/validate-rosetta/log"
)

type NewRunner struct {
	db *badger.DB
}

func (r *NewRunner) Run(ctx context.Context, cfg RunConfig) error {
	if err := r.init(ctx, cfg.dir); err != nil {
		log.Errorf("Failed to initialize: %s", err)
		return err
	}
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := r.processBlocks(ctx, cfg.start, cfg.target); err != nil {
			log.Errorf("Failed to process blocks: %s", err)
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := r.reconcileAccounts(ctx); err != nil {
			log.Errorf("Failed to reconcile accounts: %s", err)
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

func (r *NewRunner) cleanup() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

func (r *NewRunner) init(ctx context.Context, dir string) error {
	if err := r.initDB(dir); err != nil {
		return err
	}
	return nil
}

func (r *NewRunner) initDB(dir string) error {
	opts := badger.DefaultOptions(dir)
	db, err := badger.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to open database: %s", err)
	}
	r.db = db
	return nil
}

func (r *NewRunner) processBlocks(ctx context.Context, start time.Time, target int) error {
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

func (r *NewRunner) reconcileAccounts(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
	return nil
}
