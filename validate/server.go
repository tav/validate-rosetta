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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tav/validate-rosetta/log"
	"github.com/tav/validate-rosetta/process"
)

// Server acts as the Status HTTP Server for the validation processes. It
// returns a JSON-encoded status report in response to HTTP calls.
type Server struct {
	reporter *Reporter
}

// ServeHTTP acts as a handler for the Status HTTP Server.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := &statusReport{}
	data, err := json.Marshal(status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *Server) run(port uint16) {
	if port == 0 {
		return
	}
	srv := &http.Server{
		Addr:         fmt.Sprintf("localhost:%d", port),
		Handler:      s,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	log.Infof("Running Status HTTP Server: http://localhost:%d", port)
	go func() {
		process.SetExitHandler(func() {
			log.Info("Shutting down Status HTTP Server gracefully")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				log.Errorf("Failed to shutdown Status HTTP Server gracefully: %s", err)
			}
		})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Status HTTP Server failed: %s", err)
		}
	}()
}

type statusReport struct {
}
