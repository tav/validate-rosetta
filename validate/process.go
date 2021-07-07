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

// Package validate handles the validation of Rosetta API implementations.
package validate

import (
	"context"
	"time"

	"github.com/neilotoole/errgroup"
	"github.com/tav/validate-rosetta/log"
	"github.com/tav/validate-rosetta/store"
)

// Runner encapsulates the validation processes for Rosetta APIs.
type Runner struct {
	cfg        *Config
	db         *store.DB
	reconciler *Reconciler
	reporter   *Reporter
	syncer     *Syncer
}

// ValidateConstructionAPI validates the Rosetta Construction API of an
// implementation.
func (p *Runner) ValidateConstructionAPI(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		time.Sleep(time.Second)
	}
}

// ValidateDataAPI validates the Rosetta Data API of an implementation.
func (p *Runner) ValidateDataAPI(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := p.syncer.run(ctx); err != nil {
			log.Errorf("Failed to sync blocks: %s", err)
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := p.reconciler.run(ctx); err != nil {
			log.Errorf("Failed to reconcile accounts: %s", err)
			return err
		}
		return nil
	})
	return g.Wait()
}

// New instantiates a new Runner to do validation. If a status port is
// specified, this will also start up the Status HTTP server in the background.
func New(cfg *Config, db *store.DB) *Runner {
	reporter := &Reporter{
		db: db,
	}
	reconciler := &Reconciler{
		db:       db,
		reporter: reporter,
	}
	srv := &Server{
		reporter: reporter,
	}
	srv.run(cfg.StatusPort)
	syncer := &Syncer{
		db:       db,
		reporter: reporter,
	}
	return &Runner{
		cfg:        cfg,
		db:         db,
		reconciler: reconciler,
		reporter:   reporter,
		syncer:     syncer,
	}
}
