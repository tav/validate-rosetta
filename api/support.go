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
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tav/validate-rosetta/json"
)

// HTTPClient represents the global HTTP Client used to make all API calls. If
// necessary, callers should replace this global variable with their own HTTP
// Client before making any API calls.
var HTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// Client handles requests to Rosetta API servers. A Client can only be used to
// do one API call at a time. That is, do not reuse a Client while a previous
// call is still being handled.
//
// When making Client API calls, the `resp` value will be automatically Reset
// before the response JSON is decoded, so it can be reused across multiple
// Client API calls.
type Client struct {
	baseURL string
	dec     *json.Decoder
	err     *ClientError
	netjson []byte
	network NetworkIdentifier
	req     []byte
}

func (c *Client) SetNetwork(n NetworkIdentifier) {
	c.network = n
	c.netjson = EncodeNetworkForJSON(n)
}

// ClientError represents the error encountered when making a Client API call.
// Only one of the CallError or RosettaError fields will be set.
//
// ClientError values must not be retained across Client API calls, as it will
// be reset and reused by the Client in future calls.
type ClientError struct {
	// CallError indicates network and decoding related errors.
	CallError error
	// RosettaError indicates the Error value sent by the Rosetta API server.
	RosettaError Error
}

// Error implements the error interface.
func (c ClientError) Error() string {
	if c.CallError != nil {
		return fmt.Sprintf("api: client/call error: %s", c.CallError)
	}
	return fmt.Sprintf(
		"api: client/rosetta error: [%d] %s",
		c.RosettaError.Code, c.RosettaError.Message,
	)
}

// Retriable indicates whether the error is potentially retriable.
func (c ClientError) Retriable() bool {
	return c.CallError != nil || c.RosettaError.Retriable
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
	if len(m) == 0 {
		return []byte("{}"), nil
	}
	return m, nil
}

// Raw returns the raw map value encoded within a MapObject.
func (m MapObject) Raw() (map[string]interface{}, error) {
	if len(m) == 0 {
		return nil, nil
	}
	raw := map[string]interface{}{}
	if err := stdjson.Unmarshal(m, &raw); err != nil {
		return nil, fmt.Errorf("api: failed to decode MapObject: %w", err)
	}
	return raw, nil
}

// RawWithJSONNumber returns the raw map value encoded within a MapObject,
// decoding all numeric values as json.Number instead of float64.
func (m MapObject) RawWithJSONNumber() (map[string]interface{}, error) {
	if len(m) == 0 {
		return nil, nil
	}
	dec := stdjson.NewDecoder(bytes.NewReader(m))
	dec.UseNumber()
	raw := map[string]interface{}{}
	if err := dec.Decode(&raw); err != nil {
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

// InNetworkList returns whether the given NetworkIdentifier exists in the given
// list of NetworkIdentifiers.
func InNetworkList(xs []NetworkIdentifier, n NetworkIdentifier) bool {
	for _, elem := range xs {
		if n.Equal(elem) {
			return true
		}
	}
	return false
}

// MapObjectFrom encodes a raw map value into a MapObject.
func MapObjectFrom(v map[string]interface{}) (MapObject, error) {
	if len(v) == 0 {
		return nil, nil
	}
	enc, err := stdjson.Marshal(v)
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
		dec:     json.NewDecoder(),
		err:     &ClientError{},
		req:     make([]byte, 0, 1024),
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
