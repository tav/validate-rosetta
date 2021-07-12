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
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
)

var (
	resultBool  bool
	resultSlice []byte
)

func BenchmarkEncodeSmallOld(b *testing.B) {
	val := createOldAccountBalanceRequest()
	var buf *bytes.Buffer
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Mimic rosetta-sdk-go/client/setBody
		buf = &bytes.Buffer{}
		err := json.NewEncoder(buf).Encode(val)
		if err != nil {
			b.Fatalf("Failed to encode value to JSON: %s", err)
		}
		if buf.Len() == 0 {
			b.Fatal("Invalid body type")
		}
	}
	resultSlice = buf.Bytes()
}

func BenchmarkEncodeSmallNew(b *testing.B) {
	network := EncodeNetworkForJSON(NetworkIdentifier{
		Blockchain: "ontology",
		Network:    "testnet",
	})
	val := createNewAccountBalanceRequest()
	var buf []byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf = val.EncodeJSON(buf[:0], network)
	}
	resultSlice = buf
}

func BenchmarkEncodeLargeOld(b *testing.B) {
	val := createOldBlock()
	var buf *bytes.Buffer
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Mimic rosetta-sdk-go/client/setBody
		buf = &bytes.Buffer{}
		err := json.NewEncoder(buf).Encode(val)
		if err != nil {
			b.Fatalf("Failed to encode value to JSON: %s", err)
		}
		if buf.Len() == 0 {
			b.Fatal("Invalid body type")
		}
	}
	resultSlice = buf.Bytes()
}

func BenchmarkEncodeLargeNew(b *testing.B) {
	val := createNewBlock()
	var buf []byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf = val.EncodeJSON(buf[:0])
	}
	resultSlice = buf
}

func BenchmarkEqualOld(b *testing.B) {
	val := &types.AccountCurrency{
		Account: &types.AccountIdentifier{
			Address: "AcbumKMerW2abeahCen26VaDDXAHd9hatc",
		},
		Currency: &types.Currency{
			Decimals: 9,
			Metadata: map[string]interface{}{
				"contract": "0200000000000000000000000000000000000000",
			},
			Symbol: "ONG",
		},
	}
	eq := false
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		//lint:ignore SA4000 We're benchmarking val here
		eq = types.Hash(val) == types.Hash(val)
	}
	resultBool = eq
}

func BenchmarkEqualNew(b *testing.B) {
	acct := AccountIdentifier{
		Address: "AcbumKMerW2abeahCen26VaDDXAHd9hatc",
	}
	md, _ := MapObjectFrom(map[string]interface{}{
		"contract": "0200000000000000000000000000000000000000",
	})
	currency := Currency{
		Decimals: 9,
		Metadata: md,
		Symbol:   "ONG",
	}
	eq := false
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		eq = acct.Equal(acct) && currency.Equal(currency)
	}
	resultBool = eq
}

func TestEncodeJSON(t *testing.T) {
	type test struct {
		old        interface{}
		new        interface{}
		useNetwork bool
	}
	type EncodeJSON interface {
		EncodeJSON([]byte) []byte
	}
	type EncodeJSONNetwork interface {
		EncodeJSON([]byte, []byte) []byte
	}
	network := EncodeNetworkForJSON(NetworkIdentifier{
		Blockchain: "ontology",
		Network:    "testnet",
	})
	for _, test := range []test{{
		old:        createOldAccountBalanceRequest(),
		new:        createNewAccountBalanceRequest(),
		useNetwork: true,
	}, {
		old: createOldBlock(),
		new: createNewBlock(),
	}} {
		enc, err := json.Marshal(test.old)
		if err != nil {
			t.Errorf("Failed to encode old value: %s", err)
			continue
		}
		oldHash, ok := hashEncoding(t, "old", enc)
		if !ok {
			continue
		}
		if test.useNetwork {
			val := test.new.(EncodeJSONNetwork)
			enc = val.EncodeJSON(enc[:0], network)
		} else {
			val := test.new.(EncodeJSON)
			enc = val.EncodeJSON(enc[:0])
		}
		newHash, ok := hashEncoding(t, "new", enc)
		if !ok {
			continue
		}
		if oldHash != newHash {
			t.Errorf("Mismatching encoded values: %s != %s", oldHash, newHash)
		}
	}
}

func createNewAccountBalanceRequest() AccountBalanceRequest {
	md, _ := MapObjectFrom(map[string]interface{}{
		"contract": "0200000000000000000000000000000000000000",
	})
	return AccountBalanceRequest{
		AccountIdentifier: AccountIdentifier{
			Address: "AcbumKMerW2abeahCen26VaDDXAHd9hatc",
		},
		BlockIdentifier: OptionalPartialBlockIdentifier(PartialBlockIdentifier{
			Index: OptionalInt64(16075615),
		}),
		Currencies: []Currency{{
			Decimals: 9,
			Metadata: md,
			Symbol:   "ONG",
		}},
	}
}

func createNewBlock() Block {
	ong, _ := MapObjectFrom(map[string]interface{}{
		"contract": "0200000000000000000000000000000000000000",
	})
	ont, _ := MapObjectFrom(map[string]interface{}{
		"contract": "0100000000000000000000000000000000000000",
	})
	return Block{
		BlockIdentifier: BlockIdentifier{
			Hash:  "bad16a07c079f99117d9914855815b28bed3110d7fdc31645eaf9c1f1402f15b",
			Index: 16075615,
		},
		ParentBlockIdentifier: BlockIdentifier{
			Hash:  "9ba4e03b2d15440b2c0100aa56fb850a666d50b8ba88385ed4a2bd9b86340078",
			Index: 16075614,
		},
		Timestamp: 1624897505000,
		Transactions: []Transaction{{
			Operations: []Operation{{
				Account: OptionalAccountIdentifier(AccountIdentifier{
					Address: "AcbumKMerW2abeahCen26VaDDXAHd9hatc",
				}),
				Amount: OptionalAmount(Amount{
					Currency: Currency{
						Decimals: 0,
						Metadata: ont,
						Symbol:   "ONT",
					},
					Value: "-1",
				}),
				OperationIdentifier: OperationIdentifier{
					Index: 0,
				},
				Status: OptionalString("SUCCESS"),
				Type:   "transfer",
			}, {
				Account: OptionalAccountIdentifier(AccountIdentifier{
					Address: "AcbumKMerW2abeahCen26VaDDXAHd9hatc",
				}),
				Amount: OptionalAmount(Amount{
					Currency: Currency{
						Decimals: 0,
						Metadata: ont,
						Symbol:   "ONT",
					},
					Value: "1",
				}),
				OperationIdentifier: OperationIdentifier{
					Index: 1,
				},
				RelatedOperations: []OperationIdentifier{{
					Index: 0,
				}},
				Status: OptionalString("SUCCESS"),
				Type:   "transfer",
			}, {
				Account: OptionalAccountIdentifier(AccountIdentifier{
					Address: "AcbumKMerW2abeahCen26VaDDXAHd9hatc",
				}),
				Amount: OptionalAmount(Amount{
					Currency: Currency{
						Decimals: 9,
						Metadata: ong,
						Symbol:   "ONG",
					},
					Value: "-50000000",
				}),
				OperationIdentifier: OperationIdentifier{
					Index: 2,
				},
				Status: OptionalString("SUCCESS"),
				Type:   "gas_fee",
			}, {
				Account: OptionalAccountIdentifier(AccountIdentifier{
					Address: "AFmseVrdL9f9oyCzZefL9tG6UbviEH9ugK",
				}),
				Amount: OptionalAmount(Amount{
					Currency: Currency{
						Decimals: 9,
						Metadata: ong,
						Symbol:   "ONG",
					},
					Value: "50000000",
				}),
				OperationIdentifier: OperationIdentifier{
					Index: 3,
				},
				RelatedOperations: []OperationIdentifier{{
					Index: 2,
				}},
				Status: OptionalString("SUCCESS"),
				Type:   "gas_fee",
			}},
			TransactionIdentifier: TransactionIdentifier{
				Hash: "a6ea247b4a71caa3e2a11356b2e127c1f0d9e9423949b3fc325ea0b65381bbaf",
			},
		}},
	}
}

func createOldAccountBalanceRequest() *types.AccountBalanceRequest {
	return &types.AccountBalanceRequest{
		AccountIdentifier: &types.AccountIdentifier{
			Address: "AcbumKMerW2abeahCen26VaDDXAHd9hatc",
		},
		BlockIdentifier: &types.PartialBlockIdentifier{
			Index: types.Int64(16075615),
		},
		Currencies: []*types.Currency{{
			Decimals: 9,
			Metadata: map[string]interface{}{
				"contract": "0200000000000000000000000000000000000000",
			},
			Symbol: "ONG",
		}},
		NetworkIdentifier: &types.NetworkIdentifier{
			Blockchain: "ontology",
			Network:    "testnet",
		},
	}
}

func createOldBlock() *types.Block {
	ong := map[string]interface{}{
		"contract": "0200000000000000000000000000000000000000",
	}
	ont := map[string]interface{}{
		"contract": "0100000000000000000000000000000000000000",
	}
	return &types.Block{
		BlockIdentifier: &types.BlockIdentifier{
			Hash:  "bad16a07c079f99117d9914855815b28bed3110d7fdc31645eaf9c1f1402f15b",
			Index: 16075615,
		},
		ParentBlockIdentifier: &types.BlockIdentifier{
			Hash:  "9ba4e03b2d15440b2c0100aa56fb850a666d50b8ba88385ed4a2bd9b86340078",
			Index: 16075614,
		},
		Timestamp: 1624897505000,
		Transactions: []*types.Transaction{{
			Operations: []*types.Operation{{
				Account: &types.AccountIdentifier{
					Address: "AcbumKMerW2abeahCen26VaDDXAHd9hatc",
				},
				Amount: &types.Amount{
					Currency: &types.Currency{
						Decimals: 0,
						Metadata: ont,
						Symbol:   "ONT",
					},
					Value: "-1",
				},
				OperationIdentifier: &types.OperationIdentifier{
					Index: 0,
				},
				Status: types.String("SUCCESS"),
				Type:   "transfer",
			}, {
				Account: &types.AccountIdentifier{
					Address: "AcbumKMerW2abeahCen26VaDDXAHd9hatc",
				},
				Amount: &types.Amount{
					Currency: &types.Currency{
						Decimals: 0,
						Metadata: ont,
						Symbol:   "ONT",
					},
					Value: "1",
				},
				OperationIdentifier: &types.OperationIdentifier{
					Index: 1,
				},
				RelatedOperations: []*types.OperationIdentifier{{
					Index: 0,
				}},
				Status: types.String("SUCCESS"),
				Type:   "transfer",
			}, {
				Account: &types.AccountIdentifier{
					Address: "AcbumKMerW2abeahCen26VaDDXAHd9hatc",
				},
				Amount: &types.Amount{
					Currency: &types.Currency{
						Decimals: 9,
						Metadata: ong,
						Symbol:   "ONG",
					},
					Value: "-50000000",
				},
				OperationIdentifier: &types.OperationIdentifier{
					Index: 2,
				},
				Status: types.String("SUCCESS"),
				Type:   "gas_fee",
			}, {
				Account: &types.AccountIdentifier{
					Address: "AFmseVrdL9f9oyCzZefL9tG6UbviEH9ugK",
				},
				Amount: &types.Amount{
					Currency: &types.Currency{
						Decimals: 9,
						Metadata: ong,
						Symbol:   "ONG",
					},
					Value: "50000000",
				},
				OperationIdentifier: &types.OperationIdentifier{
					Index: 3,
				},
				RelatedOperations: []*types.OperationIdentifier{{
					Index: 2,
				}},
				Status: types.String("SUCCESS"),
				Type:   "gas_fee",
			}},
			TransactionIdentifier: &types.TransactionIdentifier{
				Hash: "a6ea247b4a71caa3e2a11356b2e127c1f0d9e9423949b3fc325ea0b65381bbaf",
			},
		}},
	}
}

func hashEncoding(t *testing.T, typ string, data []byte) (string, bool) {
	val := map[string]interface{}{}
	if err := json.Unmarshal(data, &val); err != nil {
		t.Errorf("Failed to decode %s value: %s\n\n%s", typ, err, string(data))
		return "", false
	}
	enc, err := json.Marshal(val)
	if err != nil {
		t.Errorf("Failed to re-encode %s value: %s", typ, err)
		return "", false
	}
	hash := sha512.Sum512_256(enc)
	return hex.EncodeToString(hash[:]), true
}
