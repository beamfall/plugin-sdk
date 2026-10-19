package sdk

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const sdkImportPath = "github.com/beamfall/plugin-sdk"

// TestCandidateLiteralsAreKeyed documents and enforces that Candidate composite
// literals are keyed-only. Adding fields to Candidate must not silently change
// the meaning of existing SDK literals in production or test code.
func TestCandidateLiteralsAreKeyed(t *testing.T) {
	t.Parallel()

	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (entry.Name() == "vendor" || entry.Name()[0] == '.') {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		fileset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fileset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}

		candidateNames := map[string]bool{}
		dotImported := false
		for _, spec := range file.Imports {
			importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil || importPath != sdkImportPath {
				continue
			}
			if spec.Name == nil {
				candidateNames["sdk"] = true
			} else if spec.Name.Name == "." {
				dotImported = true
			} else {
				candidateNames[spec.Name.Name] = true
			}
		}

		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.CompositeLit)
			if !ok || !isSDKCandidateLiteral(literal.Type, file.Name.Name, candidateNames, dotImported) {
				return true
			}
			for _, element := range literal.Elts {
				if _, keyed := element.(*ast.KeyValueExpr); !keyed {
					t.Errorf("%s: Candidate literals are keyed-only", fileset.Position(element.Pos()))
					break
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func isSDKCandidateLiteral(expr ast.Expr, packageName string, importNames map[string]bool, dotImported bool) bool {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name == "Candidate" && (packageName == "sdk" || dotImported)
	case *ast.SelectorExpr:
		qualifier, ok := expr.X.(*ast.Ident)
		return ok && importNames[qualifier.Name] && expr.Sel.Name == "Candidate"
	default:
		return false
	}
}
