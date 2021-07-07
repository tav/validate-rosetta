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

package validate

import (
	"fmt"
	"os"

	"github.com/tav/validate-rosetta/api"
	"github.com/tav/validate-rosetta/log"
)

// Config defines the configuration for validate-rosetta.
type Config struct {
	// Directory for storing validate-rosetta data.
	Directory string `json:"directory"`
	Log       struct {
		Blocks bool `json:"blocks"`
	} `json:"log"`
	// Network specifies the specific network to test against.
	Network api.NetworkIdentifier `json:"network"`
	// OfflineURL specifies the base URL for an "offline" Rosetta API server.
	OfflineURL string `json:"offline_url"`
	// OnlineURL specifies the base URL for an "online" Rosetta API server.
	OnlineURL string `json:"online_url"`
	// StatusPort specifies the port for the Status HTTP Server. If unspecified,
	// the Status HTTP Server will not be run.
	StatusPort uint16 `json:"status_port"`
}

// Init validates the Config and initializes related resources.
func (c *Config) Init() error {
	if c.Directory == "" {
		return fmt.Errorf(`validate: missing "directory" field`)
	}
	stat, err := os.Stat(c.Directory)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("validate: unable to open directory %q: %w", c.Directory, err)
		}
		if err := os.MkdirAll(c.Directory, 0o777); err != nil {
			return fmt.Errorf("validate: unable to create directory %q: %w", c.Directory, err)
		}
		log.Infof("Created directory: %s", c.Directory)
	} else if !stat.IsDir() {
		return fmt.Errorf("validate: %q is not a directory", c.Directory)
	}
	if c.OnlineURL == "" {
		return fmt.Errorf(`validate: missing "online_url" field`)
	}
	return nil
}
