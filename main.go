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

// Command validate-rosetta acts as validator for Rosetta API implementations.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tav/validate-rosetta/log"
	"github.com/tav/validate-rosetta/process"
	"github.com/tav/validate-rosetta/store"
	"github.com/tav/validate-rosetta/validate"
)

func initConfig(args []string) *validate.Config {
	if len(args) == 0 {
		log.Fatalf("Path to config file not specified")
	}
	file := args[0]
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("Failed to read config file %q: %s", file, err)
	}
	cfg := &validate.Config{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(cfg); err != nil {
		log.Fatalf("Failed to decode config file %q: %s", file, err)
	}
	if err := cfg.Init(); err != nil {
		log.Fatalf("Failed to process config file %q: %s", file, err)
	}
	return cfg
}

func initDB(path string, done <-chan bool) *store.DB {
	dir := filepath.Join(path, "store")
	db, err := store.New(dir)
	if err != nil {
		log.Fatalf("Failed to open the internal datastore at %q: %s", dir, err)
	}
	log.Infof("Opened internal datastore: %s", dir)
	process.SetExitHandler(func() {
		<-done
		if err := db.Close(); err != nil {
			log.Errorf("Failed to close the internal datastore: %s", err)
		}
	})
	return db
}

func runMethod(args []string, exec func(*validate.Runner, context.Context) error) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := initConfig(args)
	done := make(chan bool, 1)
	db := initDB(cfg.Directory, done)
	runner := validate.New(cfg, db)
	process.SetExitHandler(cancel)
	err := exec(runner, ctx)
	done <- true
	if err != nil {
		process.Exit(1)
	}
	process.Exit(0)
}

func main() {
	cmd := &cobra.Command{
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Short: "Validator for Rosetta API implementations",
		Use:   "validate-rosetta",
	}
	cmd.AddCommand(&cobra.Command{
		Long: `Validate a Rosetta Construction API implementation

The check:construction command runs an automated test of a
Construction API implementation by creating and broadcasting transactions
on a blockchain. In short, this tool generates new addresses, requests
funds, constructs transactions, signs transactions, broadcasts transactions,
and confirms transactions land on-chain. At each phase, a series of tests
are run to ensure that intermediate representations are correct (i.e. does
an unsigned transaction return a superset of operations provided during
construction?).

Check out the https://github.com/coinbase/rosetta-cli/tree/master/examples
directory for examples of how to configure this test for Bitcoin and
Ethereum.

Right now, this tool only supports transfer testing (for both account-based
and UTXO-based blockchains). However, we plan to add support for testing
arbitrary scenarios (i.e. staking, governance).`,
		Run: func(cmd *cobra.Command, args []string) {
			runMethod(args, (*validate.Runner).ValidateConstructionAPI)
		},
		Short: "Validate a Rosetta Construction API implementation",
		Use:   "construction <config-file>",
	})
	cmd.AddCommand(&cobra.Command{
		Long: `Validate a Rosetta Data API implementation.

Check all server responses are properly constructed, that there are no
duplicate blocks and transactions, that blocks can be processed
from genesis to the current block (re-orgs handled automatically), and that
computed balance changes are equal to balance changes reported by the node.

When re-running this command, it will start where it left off if you specify
some data directory. Otherwise, it will create a new temporary directory and start
again from the genesis block. If you want to discard some number of blocks
populate the start_index filed in the configuration file with some block index.
Starting from a given index can be useful to debug a small range of blocks for
issues but it is highly recommended you sync from start to finish to ensure
all correctness checks are performed.

By default, account balances are looked up at specific heights (instead of
only at the current block). If your node does not support this functionality,
you can disable historical balance lookups in your configuration file. This will
make reconciliation much less efficient but it will still work.

If check fails due to an INACTIVE reconciliation error (balance changed without
any corresponding operation), the cli will automatically try to find the block
missing an operation. If historical balance disabled is true, this automatic
debugging tool does not work.

To debug an INACTIVE account reconciliation error without historical balance lookup,
set the interesting accounts to the path of a JSON file containing
accounts that will be actively checked for balance changes at each block. This
will return an error at the block where a balance change occurred with no
corresponding operations.

If your blockchain has a genesis allocation of funds and you set
historical balance disabled to true, you must provide an
absolute path to a JSON file containing initial balances with the
bootstrap balance config. You can look at the examples folder for an example
of what one of these files looks like.`,
		Run: func(cmd *cobra.Command, args []string) {
			runMethod(args, (*validate.Runner).ValidateDataAPI)
		},
		Short: "Validate a Rosetta Data API implementation",
		Use:   "data <config-file>",
	})
	cmd.AddCommand(&cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("0.0.1")
		},
		Short: "Print the validate-rosetta version",
		Use:   "version",
	})
	if err := cmd.Execute(); err != nil {
		log.Fatalf("Failed to execute command: %s", err)
	}
}
