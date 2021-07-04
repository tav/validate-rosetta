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

// Command genapi generates api/gen.go from the Rosetta spec.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tav/validate-rosetta/log"
)

// var match = map[string]bool{
// 	"/account/balance": true,
// }

type Endpoint struct {
	Description string
	Name        string
	Request     string
	Response    string
	Summary     string
	URL         string
}

type Field struct {
	Description string
	Ident       string
	Item        string // for "slice" types
	MinZero     bool   // for "float64" / "int32" / "int64" types
	Model       *Model
	Name        string
	Ref         string
	Required    bool
	Type        string
}

type Model struct {
	Description string
	Enum        []string // for "string" types
	Fields      []*Field // for "struct" types
	MinZero     bool     // for "int64" types
	Name        string
	Type        string
}

func (m *Model) Validate() bool {
	return len(m.Enum) > 0 || m.MinZero
}

func getIdent(name string) string {
	var ident []byte
	for _, elem := range strings.Split(name, "_") {
		if elem == "" {
			continue
		}
		ident = append(ident, elem[0]-32)
		ident = append(ident, elem[1:]...)
	}
	return string(ident)
}

func getModelName(src string) string {
	return src[strings.LastIndexByte(src, '/')+1:]
}

func getPath(src map[string]interface{}, elems ...string) string {
	last := len(elems) - 1
	for i, elem := range elems {
		if i == last {
			return src[elem].(string)
		}
		src = src[elem].(map[string]interface{})
	}
	panic("invalid getPath call")
}

func getRPCModel(src map[string]interface{}, elems ...string) string {
	elems = append(elems, "content", "application/json", "schema", "$ref")
	return getModelName(getPath(src, elems...))
}

func process(spec map[string]interface{}) ([]*Endpoint, []*Model) {
	var (
		endpoints []*Endpoint
		models    []*Model
	)
	paths := spec["paths"].(map[string]interface{})
	for path, info := range paths {
		// if !match[path] {
		// 	continue
		// }
		info := info.(map[string]interface{})["post"].(map[string]interface{})
		var name []byte
		for _, elem := range strings.Split(path, "/") {
			if elem == "" {
				continue
			}
			name = append(name, elem[0]-32)
			name = append(name, elem[1:]...)
		}
		endpoints = append(endpoints, &Endpoint{
			Description: info["description"].(string),
			Name:        string(name),
			Request:     getRPCModel(info, "requestBody"),
			Response:    getRPCModel(info, "responses", "200"),
			Summary:     info["summary"].(string),
			URL:         path,
		})
	}
	components := spec["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})
	for name, info := range schemas {
		info := info.(map[string]interface{})
		model := &Model{
			Description: info["description"].(string),
			Name:        name,
		}
		typ := info["type"].(string)
		switch typ {
		case "object":
			model.Type = "struct"
			props := info["properties"].(map[string]interface{})
			for name, info := range props {
				field := &Field{
					Ident: getIdent(name),
					Name:  name,
				}
				info := info.(map[string]interface{})
				ref := info["$ref"]
				if ref == nil {
					desc := info["description"]
					if desc != nil {
						field.Description = desc.(string)
					}
					typ := info["type"].(string)
					switch typ {
					case "string":
						field.Type = "string"
					case "array":
						field.Type = typ
					case "object":
						field.Type = "map[string]interface{}"
					case "integer":
						format := info["format"].(string)
						switch format {
						case "int64":
							field.Type = "int64"
						case "int32":
							field.Type = "int32"
						default:
							log.Fatalf("Unknown integer format: %q", format)
						}
						minimum := info["minimum"]
						if minimum != nil {
							minimum := minimum.(float64)
							if minimum != 0 {
								log.Fatalf("Unknown minimum value: %d", minimum)
							}
							field.MinZero = true
						}
					case "boolean":
						field.Type = "bool"
					case "number":
						format := info["format"].(string)
						if format != "double" {
							log.Fatalf("Unknown number format: %q", format)
						}
						minimum := info["minimum"]
						if minimum != nil {
							minimum := minimum.(float64)
							if minimum != 0 {
								log.Fatalf("Unknown minimum value: %d", minimum)
							}
							field.MinZero = true
						}
						field.Type = "float64"
					default:
						log.Fatalf("Unknown field type: %q", typ)
					}
				} else {
					field.Ref = ref.(string)
				}
				if name == "hex_bytes" {
					field.Ident = "Bytes"
					field.Type = "[]byte"
				}
				model.Fields = append(model.Fields, field)
			}
			sort.Slice(model.Fields, func(i, j int) bool {
				return model.Fields[i].Ident < model.Fields[j].Ident
			})
		case "string":
			model.Type = "string"
			enum := info["enum"]
			if enum != nil {
				for _, variant := range enum.([]interface{}) {
					model.Enum = append(model.Enum, variant.(string))
				}
			}
		case "integer":
			format := info["format"].(string)
			if format != "int64" {
				log.Fatalf("Unknown integer format: %q", format)
			}
			model.Type = "int64"
			minimum := info["minimum"]
			if minimum != nil {
				minimum := minimum.(float64)
				if minimum != 0 {
					log.Fatalf("Unknown minimum value: %d", minimum)
				}
				model.MinZero = true
			}
		default:
			log.Fatalf("Unknown component type: %q", typ)
		}
		models = append(models, model)
	}
	// os.Exit(0)
	return endpoints, models
}

func writeComment(b *bytes.Buffer, text string, tabs int) {
	prefix := make([]byte, tabs+3)
	for i := 0; i < tabs; i++ {
		prefix[i] = '\t'
	}
	prefix[tabs] = '/'
	prefix[tabs+1] = '/'
	prefix[tabs+2] = ' '
	last := len(text) - 1
	limit := 77 - (tabs * 8) // assume tabs take up 4 spaces
	line := []byte{}
	word := []byte{}
	for i := 0; i < len(text); i++ {
		char := text[i]
		if char == ' ' || i == last {
			length := len(word)
			if len(line) > 0 {
				length += len(line) + 1
			}
			if length > limit {
				b.Write(prefix)
				b.Write(line)
				b.WriteByte('\n')
				if i == last {
					if len(word) > 0 {
						b.Write(prefix)
						b.Write(word)
						b.WriteByte('\n')
					}
				} else {
					line = append(line[:0], word...)
					word = word[:0]
				}
			} else {
				if len(line) > 0 {
					line = append(line, ' ')
				}
				line = append(line, word...)
				if i == last {
					b.Write(prefix)
					b.Write(line)
					b.WriteByte('\n')
				} else {
					word = word[:0]
				}
			}
		} else {
			word = append(word, char)
		}
	}
}

func writeEndpoints(b *bytes.Buffer, endpoints []*Endpoint) {
}

func writeModels(b *bytes.Buffer, models []*Model) {
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})
	for _, model := range models {
		b.WriteString("// ")
		b.WriteString(model.Name)
		b.WriteString(" type.\n")
		if model.Description != "" {
			b.WriteString("//\n")
			writeComment(b, model.Description, 0)
		}
		switch model.Type {
		case "struct":
			fmt.Fprintf(b, "type %s struct {\n", model.Name)
			for _, field := range model.Fields {
				if field.Description != "" {
					writeComment(b, field.Description, 1)
				}
				fmt.Fprintf(b, "\t%s\tstring\n", field.Ident)
			}
			fmt.Fprint(b, "}\n\n")
		case "string":
			fmt.Fprintf(b, "type %s string\n\n", model.Name)
			if len(model.Enum) > 0 {
				fmt.Fprintf(b, `func (v %s) Validate() error {
	if !(`, model.Name)
				for i, variant := range model.Enum {
					if i != 0 {
						b.WriteString(" || ")
					}
					fmt.Fprintf(b, "v == %q", variant)
				}
				fmt.Fprintf(b, `) {
		return fmt.Errorf("api: invalid %s value: %%q", v)
	}
	return nil
}
`, model.Name)
			}
		case "int64":
			fmt.Fprintf(b, "type %s int64\n\n", model.Name)
			if model.MinZero {
				fmt.Fprintf(b, `func (v %s) Validate() error {
	if v < 0 {
		return fmt.Errorf("api: %s value cannot be negative: %%d", v)
	}
	return nil
}
`, model.Name, model.Name)
			}
		default:
			log.Fatalf("Unknown model type: %q", model.Type)
		}
	}
}

func writePrelude(b *bytes.Buffer) {
	b.WriteString(`// Copyright 2021 Coinbase, Inc.
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
	"fmt"
)

`)
}

func main() {
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Stdout = buf
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to run git rev-parse: %s", err)
	}
	root := strings.TrimSpace(buf.String())
	apiPath := filepath.Join(root, "cmd", "genapi", "api.json")
	data, err := os.ReadFile(apiPath)
	if err != nil {
		log.Fatalf("Unable to read %s: %s", apiPath, err)
	}
	spec := map[string]interface{}{}
	if err := json.Unmarshal(data, &spec); err != nil {
		log.Fatalf("Unable to decode %s: %s", apiPath, err)
	}
	endpoints, models := process(spec)
	outPath := filepath.Join(root, "api", "gen.go")
	buf.Reset()
	writePrelude(buf)
	writeEndpoints(buf, endpoints)
	writeModels(buf, models)
	src, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatalf("Got error formatting Go code: %s", err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("Failed to create %s: %s", outPath, err)
	}
	if _, err := f.Write(src); err != nil {
		log.Fatalf("Failed to write to %s: %s", outPath, err)
	}
	if err := f.Close(); err != nil {
		log.Fatalf("Failed to close %s: %s", outPath, err)
	}
}
