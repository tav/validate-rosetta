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

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPClient represents the global HTTP Client used to make all API calls. If
// necessary, callers should replace this global variable with their own HTTP
// Client before making any API calls.
var HTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// MapObject represents a canonical encoding of a raw map value that is used to
// represent metadata and options within the Rosetta API.
type MapObject []byte

// Equal returns whether two MapObject values are equal.
func (m MapObject) Equal(o MapObject) bool {
	if len(m) != len(o) {
		return false
	}
	return string(m) == string(o)
}

// MarshalJSON implements the json.Marshaler interface.
func (m MapObject) MarshalJSON() ([]byte, error) {
	return m, nil
}

// Raw returns the raw map value encoded within a MapObject.
func (m MapObject) Raw() (map[string]interface{}, error) {
	if len(m) == 0 {
		return nil, nil
	}
	raw := map[string]interface{}{}
	if err := json.Unmarshal(m, &raw); err != nil {
		return nil, fmt.Errorf("api: failed to decode MapObject: %w", err)
	}
	return raw, nil
}

// EncodeNetworkForJSON will create a reusable encoding of the given
// NetworkIdentifier for use in EncodeJSON calls.
func EncodeNetworkForJSON(n NetworkIdentifier) []byte {
	buf := []byte(`{"network_identifier":`)
	return append(n.EncodeJSON(buf), ","...)
}

// MapObjectFrom encodes a raw map value into a MapObject.
func MapObjectFrom(v map[string]interface{}) (MapObject, error) {
	if len(v) == 0 {
		return nil, nil
	}
	enc, err := json.Marshal(v)
	// NOTE(tav): We depend on Go's lexicographic ordering of object keys for
	// this to be deterministic.
	if err != nil {
		return nil, fmt.Errorf("api: failed to encode MapObject: %w", err)
	}
	return MapObject(enc), nil
}

// NewClient instantiates a new Client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
	}
}

func appendMapObject(b []byte, m MapObject) []byte {
	if len(m) == 0 {
		return append(b, "{}"...)
	}
	return append(b, m...)
}

// StringSliceEqual returns whether the given string slice values are equal.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, elem := range a {
		if elem != b[i] {
			return false
		}
	}
	return true
}
