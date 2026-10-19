// Command godoc2md renders one Go package's documentation as Markdown.
//
// It is the SDK's offline, stdlib-only API-reference generator (no external
// module fetch, honoring the zero-external-dependency invariant). Driven by
// script/gen-docs.sh; not part of the published SDK surface.
//
// Usage: go run ./script/godoc2md <package dir> <import path>
package main

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: godoc2md <package dir> <import path>")
		os.Exit(2)
	}
	dir := os.Args[1]
	importPath := os.Args[2]
	pkg, fset, err := loadPackage(dir, importPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "godoc2md:", err)
		os.Exit(1)
	}
	out := render(pkg, fset, importPath)
	if _, err := os.Stdout.WriteString(out); err != nil {
		fmt.Fprintln(os.Stderr, "godoc2md:", err)
		os.Exit(1)
	}
}

func loadPackage(dir, importPath string) (*doc.Package, *token.FileSet, error) {
	names, err := sourceFileNames(dir)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(names))
	for _, name := range names {
		parsed, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, parsed)
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("no non-test Go files found in %s", dir)
	}
	docPkg, err := doc.NewFromFiles(fset, files, importPath)
	if err != nil {
		return nil, nil, err
	}
	return docPkg, fset, nil
}

func sourceFileNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func render(pkg *doc.Package, fset *token.FileSet, importPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# package %s\n\n", pkg.Name)
	fmt.Fprintf(&b, "`import \"%s\"`\n\n", importPath)
	writeDocText(&b, pkg, pkg.Doc)
	writeConsts(&b, pkg, fset, pkg.Consts, "Constants")
	writeConsts(&b, pkg, fset, pkg.Vars, "Variables")
	writeFuncs(&b, pkg, fset, pkg.Funcs, "## ")
	writeTypes(&b, pkg, fset)
	return b.String()
}

func writeDocText(b *strings.Builder, pkg *doc.Package, text string) {
	if text == "" {
		return
	}
	printer := pkg.Printer()
	printer.TextWidth = -1
	parsed := pkg.Parser().Parse(text)
	b.Write(printer.Markdown(parsed))
	b.WriteString("\n")
}

func writeConsts(b *strings.Builder, pkg *doc.Package, fset *token.FileSet, values []*doc.Value, heading string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "## %s\n\n", heading)
	for _, v := range values {
		writeDecl(b, fset, v.Decl)
		writeDocText(b, pkg, v.Doc)
	}
}

func writeFuncs(b *strings.Builder, pkg *doc.Package, fset *token.FileSet, funcs []*doc.Func, prefix string) {
	for _, f := range funcs {
		name := f.Name
		if f.Recv != "" {
			name = fmt.Sprintf("(%s) %s", f.Recv, f.Name)
		}
		fmt.Fprintf(b, "%sfunc %s\n\n", prefix, name)
		writeDecl(b, fset, f.Decl)
		writeDocText(b, pkg, f.Doc)
	}
}

func writeTypes(b *strings.Builder, pkg *doc.Package, fset *token.FileSet) {
	for _, t := range pkg.Types {
		fmt.Fprintf(b, "## type %s\n\n", t.Name)
		writeDecl(b, fset, t.Decl)
		writeDocText(b, pkg, t.Doc)
		writeConstsUnderType(b, pkg, fset, t)
		writeFuncs(b, pkg, fset, t.Funcs, "### ")
		writeFuncs(b, pkg, fset, t.Methods, "### ")
	}
}

func writeConstsUnderType(b *strings.Builder, pkg *doc.Package, fset *token.FileSet, t *doc.Type) {
	for _, v := range append(append([]*doc.Value{}, t.Consts...), t.Vars...) {
		writeDecl(b, fset, v.Decl)
		writeDocText(b, pkg, v.Doc)
	}
}

func writeDecl(b *strings.Builder, fset *token.FileSet, decl ast.Decl) {
	var src strings.Builder
	if err := format.Node(&src, fset, decl); err != nil {
		fmt.Fprintf(b, "```go\n// error rendering declaration: %v\n```\n\n", err)
		return
	}
	fmt.Fprintf(b, "```go\n%s\n```\n\n", src.String())
}
