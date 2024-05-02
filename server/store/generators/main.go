// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"text/template"
)

const (
	WithTransactionComment = "@withTransaction"
	ErrorType              = "error"
	StringType             = "string"
	IntType                = "int"
	Int32Type              = "int32"
	Int64Type              = "int64"
	BoolType               = "bool"
)

func isError(typeName string) bool {
	return strings.Contains(typeName, ErrorType)
}

func isString(typeName string) bool {
	return typeName == StringType
}

func isInt(typeName string) bool {
	return typeName == IntType || typeName == Int32Type || typeName == Int64Type
}

func isBool(typeName string) bool {
	return typeName == BoolType
}

func main() {
	if err := buildTimerLayer(); err != nil {
		log.Fatal(err)
	}

	if err := buildPublicMethods(); err != nil {
		log.Fatal(err)
	}
}

func buildTimerLayer() error {
	topLevelFunctionsToSkip := map[string]bool{
		"BeginTx":    true,
		"RollbackTx": true,
		"CommitTx":   true,
	}

	code, err := generateLayer("TimerLayer", "timer_layer.go.tmpl", topLevelFunctionsToSkip)
	if err != nil {
		return err
	}
	formatedCode, err := format.Source(code)
	if err != nil {
		return err
	}

	err = os.MkdirAll("timerlayer", 0700)
	if err != nil {
		return err
	}

	return os.WriteFile(path.Join("timerlayer", "timerlayer.go"), formatedCode, 0600)
}

func buildPublicMethods() error {
	topLevelFunctionsToSkip := map[string]bool{
		"Init":                     true,
		"UserHasConnected":         true,
		"CheckEnabledTeamByTeamID": true,
		"VerifyOAuth2State":        true,
		"StoreOAuth2State":         true,
	}

	code, err := generateLayer("SQLStore", "transactional_store.go.tmpl", topLevelFunctionsToSkip)
	if err != nil {
		return err
	}
	formatedCode, err := format.Source(code)
	if err != nil {
		return err
	}

	return os.WriteFile(path.Join("sqlstore", "public_methods.go"), formatedCode, 0600)
}

type methodParam struct {
	Name string
	Type string
}

type methodData struct {
	Params          []methodParam
	Results         []string
	WithTransaction bool
}

type storeMetadata struct {
	Name    string
	Methods map[string]methodData
}

func extractMethodMetadata(method *ast.Field, src []byte) methodData {
	params := []methodParam{}
	results := []string{}
	withTransaction := false
	ast.Inspect(method.Type, func(expr ast.Node) bool {
		if e, ok := expr.(*ast.FuncType); ok {
			if method.Doc != nil {
				for _, comment := range method.Doc.List {
					if strings.Contains(comment.Text, WithTransactionComment) {
						withTransaction = true
						break
					}
				}
			}
			if e.Params != nil {
				for _, param := range e.Params.List {
					for _, paramName := range param.Names {
						params = append(params, methodParam{Name: paramName.Name, Type: string(src[param.Type.Pos()-1 : param.Type.End()-1])})
					}
				}
			}
			if e.Results != nil {
				for _, result := range e.Results.List {
					results = append(results, string(src[result.Type.Pos()-1:result.Type.End()-1]))
				}
			}
		}
		return true
	})
	return methodData{Params: params, Results: results, WithTransaction: withTransaction}
}

func extractStoreMetadata(topLevelFunctionsToSkip map[string]bool) (*storeMetadata, error) {
	// Create the AST by parsing src.
	fset := token.NewFileSet() // positions are relative to fset

	file, err := os.Open("store.go")
	if err != nil {
		return nil, fmt.Errorf("unable to open store.go file: %w", err)
	}
	src, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	file.Close()
	f, err := parser.ParseFile(fset, "", src, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return nil, err
	}

	metadata := storeMetadata{Methods: map[string]methodData{}}

	ast.Inspect(f, func(n ast.Node) bool {
		if x, ok := n.(*ast.TypeSpec); ok {
			if x.Name.Name == "Store" {
				for _, method := range x.Type.(*ast.InterfaceType).Methods.List {
					methodName := method.Names[0].Name
					if skip := topLevelFunctionsToSkip[methodName]; !skip {
						metadata.Methods[methodName] = extractMethodMetadata(method, src)
					}
				}
			}
		}
		return true
	})

	return &metadata, nil
}

func generateLayer(name, templateFile string, topLevelFunctionsToSkip map[string]bool) ([]byte, error) {
	out := bytes.NewBufferString("")
	metadata, err := extractStoreMetadata(topLevelFunctionsToSkip)
	if err != nil {
		return nil, err
	}
	metadata.Name = name

	myFuncs := template.FuncMap{
		"renameStoreMethod": func(methodName string) string {
			return strings.ToLower(methodName[0:1]) + methodName[1:]
		},
		"genErrorResultsVars": func(results []string, errName string) string {
			vars := []string{}
			for _, typeName := range results {
				switch {
				case isError(typeName):
					vars = append(vars, errName)
				case isString(typeName):
					vars = append(vars, "\"\"")
				case isInt(typeName):
					vars = append(vars, "0")
				case isBool(typeName):
					vars = append(vars, "false")
				default:
					vars = append(vars, "nil")
				}
			}
			return strings.Join(vars, ", ")
		},
		"joinResults": func(results []string) string {
			return strings.Join(results, ", ")
		},
		"joinResultsForSignature": func(results []string) string {
			if len(results) == 0 {
				return ""
			}
			if len(results) == 1 {
				return results[0]
			}
			return fmt.Sprintf("(%s)", strings.Join(results, ", "))
		},
		"genResultsVars": func(results []string, withNilError bool) string {
			vars := []string{}
			for i, typeName := range results {
				switch {
				case isError(typeName):
					if withNilError {
						vars = append(vars, "nil")
					} else {
						vars = append(vars, "err")
					}
				case i == 0:
					vars = append(vars, "result")
				default:
					vars = append(vars, fmt.Sprintf("resultVar%d", i))
				}
			}
			return strings.Join(vars, ", ")
		},
		"errorToBoolean": func(results []string) string {
			for _, typeName := range results {
				if isError(typeName) {
					return "err == nil"
				}
			}
			return "true"
		},
		"errorPresent": func(results []string) bool {
			for _, typeName := range results {
				if isError(typeName) {
					return true
				}
			}
			return false
		},
		"errorVar": func(results []string) string {
			for _, typeName := range results {
				if isError(typeName) {
					return "err"
				}
			}
			return ""
		},
		"joinParams": func(params []methodParam) string {
			paramsNames := make([]string, 0, len(params))
			for _, param := range params {
				tParams := ""
				if strings.HasPrefix(param.Type, "...") {
					tParams = "..."
				}
				paramsNames = append(paramsNames, param.Name+tParams)
			}
			return strings.Join(paramsNames, ", ")
		},
		"joinParamsWithType": func(params []methodParam) string {
			paramsWithType := []string{}
			for _, param := range params {
				paramsWithType = append(paramsWithType, fmt.Sprintf("%s %s", param.Name, param.Type))
			}
			return strings.Join(paramsWithType, ", ")
		},
	}

	t := template.Must(template.New(templateFile).Funcs(myFuncs).ParseFiles("generators/" + templateFile))
	if err = t.Execute(out, metadata); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
