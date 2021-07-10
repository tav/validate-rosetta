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

// Command genapi generates api/api.go from the Rosetta spec.
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/scanner"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tav/validate-rosetta/log"
	"gopkg.in/yaml.v3"
)

var (
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
	Description string
	Ident       string
	MinZero     bool // for "float64" / "int32" / "int64" types
	Model       *Model
	Name        string
	Ref         string
	Optional    bool
	Skip        bool
	Slice       bool
	Validate    bool
	Type        string
}

type Model struct {
	Description     string
	EndpointRequest bool
	Enum            []string // for "string" types
	Fields          []*Field // for "struct" types
	MinZero         bool     // for "int64" types
	Referenced      []*Model
	Name            string
	Network         bool
	Type            string
	Validate        bool
}

func (m *Model) ValidateStatus() bool {
	return len(m.Enum) > 0 || m.MinZero
}

func appendJSONKey(k string) string {
	if len(k) <= 13 {
		return fmt.Sprintf("`%s`...", `"`+k+`":`)
	}
	params := []byte(`'"', '`)
	for i := 0; i < len(k); i++ {
		if i != 0 {
			params = append(params, ", '"...)
		}
		char := k[i]
		params = append(params, char, '\'')
	}
	return string(append(params, `, '"', ':'`...))
}

func appendJSONKeySlice(k string) string {
	if len(k) <= 12 {
		return fmt.Sprintf("`%s`...", `"`+k+`":[`)
	}
	params := []byte(`'"', '`)
	for i := 0; i < len(k); i++ {
		if i != 0 {
			params = append(params, ", '"...)
		}
		char := k[i]
		params = append(params, char, '\'')
	}
	return string(append(params, `, '"', ':', '['`...))
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
	writeEnums(buf, models)
	writeEndpoints(buf, endpoints)
	writeModels(buf, models)
	if exitBefore == "format:noprint" {
		os.Exit(0)
	}
	if exitBefore == "format" {
		fmt.Println(buf.String())
		os.Exit(0)
	}
	src := buf.Bytes()
	dst, err := format.Source(src)
	if err != nil {
		logFormatError(src, err)
		log.Fatalf("Failed to format generated Go code: %s", err)
	}
	if exitBefore == "writeFile" {
		fmt.Println(string(dst))
		os.Exit(0)
	}
	return dst
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
		lead := elem[0]
		if lead >= 'a' && lead <= 'z' {
			ident = append(ident, lead-32)
		} else {
			ident = append(ident, lead)
		}
		ident = append(ident, elem[1:]...)
	}
	id := string(ident)
	// NOTE(tav): We special-case certain identifiers so as to match Go's rules
	// on initialisms.
	switch id {
	case "Ecdsa":
		return "ECDSA"
	case "EcdsaRecovery":
		return "ECDSARecovery"
	case "PeerId":
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

func getPrivateIdent(name string) string {
	ident := make([]byte, 0, len(name))[:1]
	ident[0] = name[0] + 32
	ident = append(ident, name[1:]...)
	return string(ident)
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

func logFormatError(src []byte, err error) {
	list, ok := err.(scanner.ErrorList)
	if !ok {
		return
	}
	lines := bytes.Split(src, []byte("\n"))
	for _, e := range list {
		prev := " "
		start := e.Pos.Line - 5
		if start < 0 {
			start = 0
		}
		for _, line := range lines[start : e.Pos.Line-1] {
			prev += " " + strings.ReplaceAll(string(line), "\t", " ") + "\n"
		}
		line := strings.ReplaceAll(string(lines[e.Pos.Line-1]), "\t", " ")
		log.Errorf(
			"Go format error: %s\n\n %s%s\n%s^\n",
			e.Msg, prev, line, strings.Repeat(" ", e.Pos.Column-1),
		)
	}
}

func processEndpoints(specDir string, spec map[string]interface{}) ([]*Endpoint, map[string]bool) {
	var endpoints []*Endpoint
	reqs := map[string]bool{}
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
		req := getRPCModel(info, "requestBody")
		reqs[req] = true
		endpoints = append(endpoints, &Endpoint{
			Description: info["description"].(string),
			Name:        string(name),
			Request:     req,
			Response:    getRPCModel(info, "responses", "200"),
			Summary:     info["summary"].(string),
			URL:         path,
		})
	}
	return endpoints, reqs
}

func processModels(specDir string, spec map[string]interface{}, reqs map[string]bool) []*Model {
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
			if reqs[name] {
				model.EndpointRequest = true
			}
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
					Optional: !required[name],
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
						field.Slice = true
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
						field.Slice = true
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
					field.Slice = true
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
			sort.Strings(model.Enum)
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
			if refModel.Name == "NetworkIdentifier" && model.EndpointRequest {
				field.Skip = true
				model.Network = true
				continue
			}
			refModel.Referenced = append(refModel.Referenced, model)
			field.Model = refModel
			field.Ref = ref
			switch refModel.Type {
			case "struct":
				if field.Slice {
					field.Type = "[]" + refModel.Name
				} else {
					field.Type = refModel.Name
				}
			case "int64", "string":
				if field.Slice {
					log.Fatalf("Unexpected array ref model type: %q", refModel.Type)
				}
				field.Type = refModel.Name
			default:
				log.Fatalf("Unexpected ref model type: %q", refModel.Type)
			}
		}
	}
	// for _, model := range models {
	// 	for _, ref := range model.Referenced {
	// 		if model.Validate {

	// 		}
	// 	}
	// }
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})
	return models
}

func writeClient(b *bytes.Buffer) {
	b.WriteString(`// Client handles requests to Rosetta API servers.
//
// A Client can only be used to do one API call at a time. That is, do not
// re-use a Client while a previous call is still being handled.
type Client struct {
	baseURL string
}

`)
}

func writeComment(b *bytes.Buffer, text string, tabs int) {
	if text[0] == '\n' {
		log.Fatalf("Got comment with a leading newline: %q", text)
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

func writeEncodeJSONField(b *bytes.Buffer, field *Field, cond string, enc string) {
	key := appendJSONKey(field.Name)
	if field.Optional {
		fmt.Fprintf(b, `	if %s {
		b = append(b, %s)
	`, fmt.Sprintf(cond, field.Ident), key)
	} else {
		fmt.Fprintf(b, "\tb = append(b, %s)\n", key)
	}
	fmt.Fprintf(b, "\tb = "+enc+"\n", field.Ident)
	if field.Optional {
		fmt.Fprintf(b, "\tb = append(b, \",\"...)\n\t}\n")
	} else {
		fmt.Fprintf(b, "\tb = append(b, \",\"...)\n")
	}
}

func writeEncodeJSONFieldRef(b *bytes.Buffer, field *Field, enc string) {
	enc = fmt.Sprintf(enc, field.Ident)
	key := appendJSONKey(field.Name)
	if field.Optional {
		fmt.Fprintf(b, `	if v.%sSet {
		b = append(b, %s)
		b = %s
		b = append(b, ","...)
	}
`, field.Ident, key, enc)
	} else {
		fmt.Fprintf(b, `	b = append(b, %s)
	b = %s
	b = append(b, ","...)
`, key, enc)
	}
}

func writeEncodeJSONFieldSlice(b *bytes.Buffer, field *Field, enc string) {
	key := appendJSONKeySlice(field.Name)
	if field.Optional {
		fmt.Fprintf(b, `	if len(v.%s) > 0 {
`, field.Ident)
	}
	fmt.Fprintf(b, `	b = append(b, %s)
	for i, elem := range v.%s {
		if i != 0 {
			b = append(b, ","...)
		}
		b = %s
	}
	b = append(b, "],"...)
`, key, field.Ident, enc)
	if field.Optional {
		b.WriteString("\t}\n")
	}
}

func writeEncodeJSONFunc(b *bytes.Buffer, model *Model) {
	fmt.Fprintf(b, "// EncodeJSON encodes %s into JSON.\n", model.Name)
	prelude := `func (v %s) EncodeJSON(b []byte) []byte {
	b = append(b, "{"...)
`
	if model.EndpointRequest && model.Network {
		if len(model.Fields) == 0 {
			panic("unexpected")
		}
		prelude = `func (v %s) EncodeJSON(b []byte, network []byte) []byte {
	b = append(b, network...)
`
	}
	fmt.Fprintf(b, prelude, model.Name)
	for _, field := range model.Fields {
		switch field.Type {
		case "string":
			writeEncodeJSONField(b, field, `v.%sSet`, "json.AppendString(b, v.%s)")
		case "int64":
			writeEncodeJSONField(b, field, `v.%sSet`, "json.AppendInt(b, v.%s)")
		case "MapObject":
			writeEncodeJSONField(b, field, `len(v.%s) > 0`, "append(b, v.%s...)")
		case "[]byte":
			writeEncodeJSONField(b, field, `len(v.%s) > 0`, "json.AppendHexBytes(b, v.%s)")
		case "int32":
			writeEncodeJSONField(b, field, `v.%sSet`, "json.AppendInt(b, int64(v.%s))")
		case "bool":
			writeEncodeJSONField(b, field, `v.%sSet`, "json.AppendBool(b, v.%s)")
		case "float64":
			writeEncodeJSONField(b, field, `v.%sSet`, "json.AppendFloat(b, v.%s)")
		default:
			// TODO
			if field.Model == nil {
				continue
			}
			switch field.Model.Type {
			case "struct":
				if field.Slice {
					writeEncodeJSONFieldSlice(b, field, "elem.EncodeJSON(b)")
				} else {
					writeEncodeJSONFieldRef(b, field, "v.%s.EncodeJSON(b)")
				}
				continue
			case "string":
				writeEncodeJSONFieldRef(b, field, "json.AppendString(b, string(v.%s))")
				continue
			case "int64":
				writeEncodeJSONFieldRef(b, field, "json.AppendInt(b, int64(v.%s))")
				continue
			}
			panic("unexpected")
		}
	}
	fmt.Fprintf(b, `	last := len(b) - 1
	if b[last] == ',' {
		b[last] = '}'
		return b
	}
	return append(b, "}"...)
}
`)
}

func writeEndpoints(b *bytes.Buffer, endpoints []*Endpoint) {
	writeClient(b)
}

func writeEnums(b *bytes.Buffer, models []*Model) {
	for _, model := range models {
		if len(model.Enum) == 0 {
			continue
		}
		fmt.Fprintf(b, `// %s values.
const (
`, model.Name)
		for _, variant := range model.Enum {
			fmt.Fprintf(
				b, "\t%s %s = %q\n", getIdent(variant), model.Name, variant,
			)
		}
		fmt.Fprintf(b, `)

`)
	}
}

func writeEqualFunc(b *bytes.Buffer, model *Model, equals map[string]string) {
	fmt.Fprintf(b, `// Equal returns whether two %s values are equal.
func (v %s) Equal(o %s) bool {
		return `, model.Name, model.Name, model.Name)
	written := false
	for _, field := range model.Fields {
		if field.Skip {
			continue
		}
		if written {
			b.WriteString(" &&\n\t\t")
		}
		switch field.Type {
		case "string", "int32", "int64", "bool", "float64":
			fmt.Fprintf(b, "v.%s == o.%s", field.Ident, field.Ident)
		case "MapObject", "[]byte":
			fmt.Fprintf(b, "string(v.%s) == string(o.%s)", field.Ident, field.Ident)
		case "[]string":
			fmt.Fprintf(
				b, "len(v.%s) == len(o.%s) &&\n\t\tstringSliceEqual(v.%s, o.%s)",
				field.Ident, field.Ident, field.Ident, field.Ident,
			)
		default:
			if field.Slice {
				prefix := getPrivateIdent(field.Model.Name)
				equals[field.Model.Name] = prefix
				fmt.Fprintf(
					b, "len(v.%s) == len(o.%s) &&\n\t\t%sSliceEqual(v.%s, o.%s)",
					field.Ident, field.Ident, prefix, field.Ident, field.Ident,
				)
			} else {
				if field.Model != nil && field.Model.Type == "struct" {
					fmt.Fprintf(b, "v.%s.Equal(o.%s)", field.Ident, field.Ident)
				} else {
					fmt.Fprintf(b, "v.%s == o.%s", field.Ident, field.Ident)
				}
			}
		}
		if field.Optional && !field.Slice {
			fmt.Fprintf(b, " &&\n\t\tv.%sSet == o.%sSet", field.Ident, field.Ident)
		}
		written = true
	}
	b.WriteString("\n}\n\n")
}

func writeFile(root string, src []byte) {
	outPath := filepath.Join(root, "api", "api.go")
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
	equals := map[string]string{}
	for _, model := range models {
		writeModelComment(b, model)
		switch model.Type {
		case "struct":
			writeStructModel(b, model)
			writeEncodeJSONFunc(b, model)
			writeEqualFunc(b, model, equals)
			writeResetFunc(b, model)
		case "string":
			writeStringModel(b, model)
		case "int64":
			writeInt64Model(b, model)
		default:
			log.Fatalf("Unknown top-level model type: %q", model.Type)
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

// Package api provides a client for Rosetta API servers.
package api

import (
	"fmt"

	"github.com/tav/validate-rosetta/json"
)

`)
}

func writeResetFunc(b *bytes.Buffer, model *Model) {
	fmt.Fprintf(b, `// Reset resets %s so that it can be reused.
func (v *%s) Reset() {
`, model.Name, model.Name)
	for _, field := range model.Fields {
		if field.Skip {
			continue
		}
		switch field.Type {
		case "string":
			fmt.Fprintf(b, "\tv.%s = \"\"\n", field.Ident)
		case "int32", "int64", "float64":
			fmt.Fprintf(b, "\tv.%s = 0\n", field.Ident)
		case "bool":
			fmt.Fprintf(b, "\tv.%s = false\n", field.Ident)
		default:
			if field.Slice {
				fmt.Fprintf(b, "\tv.%s = v.%s[:0]\n", field.Ident, field.Ident)
			} else if field.Model != nil {
				switch field.Model.Type {
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
		if field.Optional && !field.Slice {
			fmt.Fprintf(b, "\tv.%sSet = false\n", field.Ident)
		}
	}
	b.WriteString("}\n\n")
}

func writeSliceEqualFuncs(b *bytes.Buffer, equals map[string]string) {
	eqTypes := make([]string, len(equals))
	idx := 0
	for typ := range equals {
		eqTypes[idx] = typ
		idx++
	}
	sort.Strings(eqTypes)
	for _, typ := range eqTypes {
		prefix := equals[typ]
		fmt.Fprintf(b, `func %sSliceEqual(a, b []%s) bool {
	for i, elem := range a {
		if !elem.Equal(b[i]) {
			return false
		}
	}
	return true
}

`, prefix, typ)
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
		if field.Skip {
			continue
		}
		if field.Description != "" {
			writeComment(b, field.Description, 1)
		}
		fmt.Fprintf(b, "\t%s\t%s\n", field.Ident, field.Type)
		if field.Optional && !field.Slice {
			fmt.Fprintf(b, "\t%sSet\tbool\n", field.Ident)
		}
	}
	b.WriteString("}\n\n")
}

func main() {
	root := getGitRoot()
	specDir, spec := getSpec(root)
	endpoints, reqs := processEndpoints(specDir, spec)
	models := processModels(specDir, spec, reqs)
	src := genFile(endpoints, models)
	writeFile(root, src)
}

func init() {
	// NOTE(tav): Uncomment one of the following stages to emit output for
	// debugging during development.

	// exitBefore = "genFile"
	// exitBefore = "format:noprint"
	// exitBefore = "format"
	// exitBefore = "writeFile"
}
