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
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tav/validate-rosetta/log"
	"gopkg.in/yaml.v3"
)

var (
	exitAfter  = ""
	exitBefore = ""
)

type Endpoint struct {
	Description string
	Name        string
	Request     string
	Response    string
	Summary     string
	URL         string
}

type Field struct {
	Array       bool
	Description string
	Ident       string
	MinZero     bool // for "float64" / "int32" / "int64" types
	Model       *Model
	Name        string
	Ref         string
	Required    bool
	Validate    bool
	Type        string
}

type Model struct {
	Description string
	Enum        []string // for "string" types
	Fields      []*Field // for "struct" types
	MinZero     bool     // for "int64" types
	Referenced  []*Model
	Name        string
	Type        string
	Validate    bool
}

func (m *Model) ValidateStatus() bool {
	return len(m.Enum) > 0 || m.MinZero
}

func commentLines(text string) [][]byte {
	lines := [][]byte{}
	line := []byte{}
	split := bytes.Split(bytes.TrimSpace([]byte(text)), []byte("\n"))
	last := len(split) - 1
	for i, src := range split {
		if len(src) == 0 || src[0] == '*' {
			if len(line) > 0 {
				line = append(line, '.')
				lines = append(lines, []byte(string(line)))
			}
			lines = append(lines, []byte(src))
			line = line[:0]
			continue
		}
		if len(line) > 0 {
			line = append(line, ' ')
		}
		line = append(line, src...)
		if i == last {
			line = append(line, '.')
			lines = append(lines, []byte(string(line)))
		}
	}
	return lines
}

func commentPrefix(tabs int) []byte {
	prefix := make([]byte, tabs+3)
	for i := 0; i < tabs; i++ {
		prefix[i] = '\t'
	}
	prefix[tabs] = '/'
	prefix[tabs+1] = '/'
	prefix[tabs+2] = ' '
	return prefix
}

func genFile(endpoints []*Endpoint, models []*Model) []byte {
	if exitBefore == "genFile" {
		os.Exit(0)
	}
	buf := &bytes.Buffer{}
	writePrelude(buf)
	writeEndpoints(buf, endpoints)
	writeModels(buf, models)
	if exitBefore == "format" {
		fmt.Println(buf.String())
		os.Exit(0)
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatalf("Got error formatting Go code: %s", err)
	}
	if exitAfter == "format" {
		fmt.Println(string(src))
		os.Exit(0)
	}
	return src
}

func getGitRoot() string {
	buf := &bytes.Buffer{}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Stdout = buf
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to run git rev-parse: %s", err)
	}
	return strings.TrimSpace(buf.String())
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
	id := string(ident)
	if id == "PeerId" {
		return "PeerID"
	}
	return id
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

func getSpec(root string) (string, map[string]interface{}) {
	specDir := filepath.Join(root, "cmd", "genapi", "rosetta-specifications")
	if err := os.Chdir(specDir); err != nil {
		log.Fatalf("Unable to switch to the rosetta-specifications directory: %s", err)
	}
	apiPath := filepath.Join(specDir, "api.yaml")
	data, err := os.ReadFile("api.yaml")
	if err != nil {
		log.Fatalf("Unable to read %s: %s", apiPath, err)
	}
	spec := map[string]interface{}{}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		log.Fatalf("Unable to decode %s: %s", apiPath, err)
	}
	return specDir, spec
}

func processEndpoints(specDir string, spec map[string]interface{}) []*Endpoint {
	var endpoints []*Endpoint
	paths := spec["paths"].(map[string]interface{})
	for path, info := range paths {
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
	return endpoints
}

func processModels(specDir string, spec map[string]interface{}) []*Model {
	var models []*Model
	components := spec["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})
	mapping := map[string]*Model{}
	for name, info := range schemas {
		model := &Model{
			Name: name,
		}
		info := info.(map[string]interface{})
		if ref := info["$ref"]; ref != nil {
			ref := ref.(string)
			filename := ref[strings.LastIndexByte(ref, '/'):]
			path := filepath.Join(specDir, "models", filename)
			data, err := os.ReadFile(path)
			if err != nil {
				log.Fatalf("Unable to read %s: %s", path, err)
			}
			info = map[string]interface{}{}
			if err := yaml.Unmarshal(data, &info); err != nil {
				log.Fatalf("Unable to decode %s: %s", path, err)
			}
		}
		model.Description = info["description"].(string)
		typ := info["type"].(string)
		switch typ {
		case "object":
			model.Type = "struct"
			required := map[string]bool{}
			if info["required"] != nil {
				for _, name := range info["required"].([]interface{}) {
					required[name.(string)] = true
				}
			}
			props := info["properties"].(map[string]interface{})
			for name, info := range props {
				field := &Field{
					Ident:    getIdent(name),
					Name:     name,
					Required: required[name],
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
						field.Array = true
						field.Type = ""
						items := info["items"].(map[string]interface{})
						ref := items["$ref"]
						if ref == nil {
							typ := items["type"].(string)
							if typ != "string" {
								log.Fatalf("Unexpected array elem type: %q", typ)
							}
							field.Type = "[]string"
						} else {
							field.Ref = ref.(string)
						}
					case "object":
						field.Array = true
						field.Type = "MapObject"
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
							minimum := minimum.(int)
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
					field.Array = true
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
				minimum := minimum.(int)
				if minimum != 0 {
					log.Fatalf("Unknown minimum value: %d", minimum)
				}
				model.MinZero = true
			}
		default:
			log.Fatalf("Unknown component type: %q", typ)
		}
		mapping[model.Name] = model
		models = append(models, model)
	}
	for _, model := range models {
		for _, field := range model.Fields {
			if field.Ref == "" {
				continue
			}
			ref := getModelName(field.Ref)
			idx := strings.LastIndexByte(ref, '.')
			if idx >= 0 {
				ref = ref[:idx]
			}
			refModel, ok := mapping[ref]
			if !ok {
				log.Fatalf("Could not find model %s", ref)
			}
			refModel.Referenced = append(refModel.Referenced, model)
			field.Model = refModel
			field.Ref = ref
			switch refModel.Type {
			case "struct":
				if field.Array {
					field.Type = "[]" + refModel.Name
				} else {
					field.Type = refModel.Name
				}
			case "int64", "string":
				if field.Array {
					log.Fatalf("Unexpected array ref model type: %q", refModel.Type)
				}
				field.Type = refModel.Name
			default:
				log.Fatalf("Unexpected ref model type: %q", refModel.Type)
			}
		}
	}
	for _, model := range models {
		for _, ref := range model.Referenced {
			if model.Validate {

			}
		}
	}
	return models
}

func writeComment(b *bytes.Buffer, text string, tabs int) {
	if text[0] == '\n' {
		log.Fatalf("Got %q", text)
	}
	prefix := commentPrefix(tabs)
	limit := 77 - (tabs * 4) // assume tabs take up 4 spaces
	for _, line := range commentLines(text) {
		if len(line) == 0 || line[0] == '*' {
			b.Write(prefix)
			b.Write(line)
			b.WriteByte('\n')
			continue
		}
		writeCommentLine(b, line, prefix, limit)
	}
}

func writeCommentLine(b *bytes.Buffer, src []byte, prefix []byte, limit int) {
	last := len(src) - 1
	line := []byte{}
	word := []byte{}
	for i := 0; i < len(src); i++ {
		char := src[i]
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

func writeEqualFunc(b *bytes.Buffer, model *Model, equals map[string]bool) {
	fmt.Fprintf(b, `// Equal returns whether two %s values are equal.
	func (v %s) Equal(o %s) bool {
		return `, model.Name, model.Name, model.Name)
	for i, field := range model.Fields {
		if i != 0 {
			b.WriteString(" && \n\t\t")
		}
		if !field.Required {
			fmt.Fprintf(b, "v.%sSet == o.%sSet && ", field.Ident, field.Ident)
		}
		switch field.Type {
		case "string", "int32", "int64", "bool", "float64":
			fmt.Fprintf(b, "v.%s == o.%s", field.Ident, field.Ident)
		case "MapObject":
			fmt.Fprintf(b, "MapObjectEqual(v.%s, o.%s)", field.Ident, field.Ident)
		case "[]byte":
			fmt.Fprintf(b, "bytes.Equal(v.%s, o.%s)", field.Ident, field.Ident)
		case "[]string":
			fmt.Fprintf(b, "StringSliceEqual(v.%s, o.%s)", field.Ident, field.Ident)
		default:
			if field.Array {
				equals[field.Model.Name] = true
				fmt.Fprintf(b, "%sSliceEqual(v.%s, o.%s)", field.Model.Name, field.Ident, field.Ident)
			} else {
				if field.Model != nil && field.Model.Type == "struct" {
					fmt.Fprintf(b, "v.%s.Equal(o.%s)", field.Ident, field.Ident)
				} else {
					fmt.Fprintf(b, "v.%s == o.%s", field.Ident, field.Ident)
				}
			}
		}
	}
	b.WriteString("\n}\n\n")
}

func writeGenFile(root string, src []byte) {
	outPath := filepath.Join(root, "api", "gen.go")
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

func writeInt64Model(b *bytes.Buffer, model *Model) {
	fmt.Fprintf(b, "type %s int64\n\n", model.Name)
	if model.MinZero {
		fmt.Fprintf(b, `// Validate the %s value.
func (v %s) Validate() error {
if v < 0 {
return fmt.Errorf("api: %s value cannot be negative: %%d", v)
}
return nil
}

`, model.Name, model.Name, model.Name)
	}
}

func writeModelComment(b *bytes.Buffer, model *Model) {
	if model.Description == "" {
		b.WriteString("// ")
		b.WriteString(model.Name)
		b.WriteString(" type.\n")
	} else {
		if !strings.HasPrefix(model.Description, model.Name+" ") {
			b.WriteString("// ")
			b.WriteString(model.Name)
			b.WriteString(" type.\n")
			b.WriteString("//\n")
		}
		writeComment(b, model.Description, 0)
	}
}

func writeModels(b *bytes.Buffer, models []*Model) {
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})
	equals := map[string]bool{}
	for _, model := range models {
		writeModelComment(b, model)
		switch model.Type {
		case "struct":
			writeStructModel(b, model)
			writeEqualFunc(b, model, equals)
			writeResetFunc(b, model)
		case "string":
			writeStringModel(b, model)
		case "int64":
			writeInt64Model(b, model)
		default:
			log.Fatalf("Unknown model type: %q", model.Type)
		}
	}
	writeSliceEqualFuncs(b, equals)
}

func writePrelude(b *bytes.Buffer) {
	b.WriteString(`// DO NOT EDIT.
// Generated by running: go run cmd/genapi/genapi.go

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
	"fmt"
)

`)
}

func writeResetFunc(b *bytes.Buffer, model *Model) {
	fmt.Fprintf(b, `// Reset resets %s so that it can be reused.
	func (v *%s) Reset() {
	`, model.Name, model.Name)
	for _, field := range model.Fields {
		switch field.Type {
		case "string":
			fmt.Fprintf(b, "\tv.%s = \"\"\n", field.Ident)
		case "int32", "int64", "float64":
			fmt.Fprintf(b, "\tv.%s = 0\n", field.Ident)
		case "bool":
			fmt.Fprintf(b, "\tv.%s = false\n", field.Ident)
		default:
			if field.Array {
				fmt.Fprintf(b, `	if len(v.%s) > 0 {
			v.%s = v.%s[:0]
		}
	`, field.Ident, field.Ident, field.Ident)
			} else if field.Model != nil {
				refModel := field.Model
				switch refModel.Type {
				case "string":
					fmt.Fprintf(b, "\tv.%s = \"\"\n", field.Ident)
				case "int32", "int64", "float64":
					fmt.Fprintf(b, "\tv.%s = 0\n", field.Ident)
				case "bool":
					fmt.Fprintf(b, "\tv.%s = false\n", field.Ident)
				default:
					fmt.Fprintf(b, "\tv.%s.Reset()\n", field.Ident)
				}
			} else {
				fmt.Fprintf(b, "\tv.%s.Reset()\n", field.Ident)
			}
		}
		if !field.Required {
			fmt.Fprintf(b, "\tv.%sSet = false\n", field.Ident)
		}
	}
	b.WriteString("}\n\n")
}

func writeSliceEqualFuncs(b *bytes.Buffer, equals map[string]bool) {
	eqTypes := make([]string, len(equals))
	idx := 0
	for typ := range equals {
		eqTypes[idx] = typ
		idx++
	}
	sort.Strings(eqTypes)
	for _, typ := range eqTypes {
		fmt.Fprintf(b, `// %sSliceEqual returns whether the given %s slice values are equal.
func %sSliceEqual(a, b []%s) bool {
	if len(a) != len(b) {
		return false
	}
	for i, elem := range a {
		if !elem.Equal(b[i]) {
			return false
		}
	}
	return true
}

`, typ, typ, typ, typ)
	}
}

func writeStringModel(b *bytes.Buffer, model *Model) {
	fmt.Fprintf(b, "type %s string\n\n", model.Name)
	if len(model.Enum) > 0 {
		fmt.Fprintf(b, `// Validate the %s value.
func (v %s) Validate() error {
if !(`, model.Name, model.Name)
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
}

func writeStructModel(b *bytes.Buffer, model *Model) {
	fmt.Fprintf(b, "type %s struct {\n", model.Name)
	for _, field := range model.Fields {
		if field.Description != "" {
			writeComment(b, field.Description, 1)
		}
		fmt.Fprintf(b, "\t%s\t%s\n", field.Ident, field.Type)
		if !field.Required {
			fmt.Fprintf(b, "\t%sSet\tbool\n", field.Ident)
		}
	}
	fmt.Fprint(b, "}\n\n")
}

func main() {
	root := getGitRoot()
	specDir, spec := getSpec(root)
	endpoints := processEndpoints(specDir, spec)
	models := processModels(specDir, spec)
	src := genFile(endpoints, models)
	writeGenFile(root, src)
}

func init() {
	// exitAfter = "format"
	// exitBefore = "genFile"
	// exitBefore = "format"
}
