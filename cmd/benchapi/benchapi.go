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

// Command benchapi benchmarks Rosetta API calls.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/tav/validate-rosetta/log"
	"github.com/tav/validate-rosetta/process"
)

type RunConfig struct {
	dir    string
	start  time.Time
	target int
}

type Runner interface {
	Run(ctx context.Context, cfg RunConfig) error
}

func run(runner Runner, target int64) {
	if err := os.MkdirAll("benchapi-data", 0o777); err != nil {
		log.Fatalf("Failed to create benchapi-data directory: %s", err)
	}
	dir, err := os.MkdirTemp("benchapi-data", "db")
	if err != nil {
		log.Fatalf("Failed to create temp directory: %s", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	process.SetExitHandler(cancel)
	err = runner.Run(ctx, RunConfig{
		dir:    dir,
		start:  time.Now(),
		target: int(target),
	})
	if err := os.RemoveAll(dir); err != nil {
		log.Errorf("Failed to remove temp directory: %s", err)
	}
	if err != nil {
		os.Exit(1)
	}
}

func main() {
	args := os.Args[1:]
	if len(args) != 2 {
		fmt.Println(`Usage: benchapi <target-height> [ "old" | "new" ]`)
		os.Exit(1)
	}
	target, err := strconv.ParseInt(args[0], 10, 32)
	if err != nil || target <= 0 {
		log.Fatalf("Invalid target height value: %q", args[0])
	}
	switch args[1] {
	case "old":
		run(&OldRunner{}, target)
	case "new":
		run(&NewRunner{}, target)
	default:
		log.Fatalf("Invalid benchmark type: %q", args[1])
	}
}
