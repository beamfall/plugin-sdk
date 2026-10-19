package sdktest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const sdkModulePath = "github.com/beamfall/plugin-sdk"

var fsWriteCalls = map[string]bool{
	"Chmod":      true,
	"Chown":      true,
	"Chtimes":    true,
	"Create":     true,
	"CreateTemp": true,
	"Link":       true,
	"Mkdir":      true,
	"MkdirAll":   true,
	"OpenFile":   true,
	"Remove":     true,
	"RemoveAll":  true,
	"Rename":     true,
	"Symlink":    true,
	"Truncate":   true,
	"WriteFile":  true,
}

var httpOutboundCalls = map[string]bool{
	"Get":                   true,
	"Head":                  true,
	"NewRequest":            true,
	"NewRequestWithContext": true,
	"Post":                  true,
	"PostForm":              true,
}

// deniedImports are packages plugin domain code may never import, regardless of
// file: raw sockets ("net"), mail ("net/smtp"), process spawning ("os/exec"),
// dynamic loading ("plugin"), and the type-safety escapes ("syscall",
// "unsafe"). "net/http" has its own rule (adapter seam files only) and is not
// listed here; the rest of the network-capable net/* tree is caught by
// networkCapableImport rather than enumerated here.
var deniedImports = map[string]bool{
	"net":      true,
	"net/smtp": true,
	"os/exec":  true,
	"plugin":   true,
	"syscall":  true,
	"unsafe":   true,
}

// nonNetworkNetPackages are the standard-library net/* packages that carry no
// outbound-connection capability: pure value and parsing types with no dialer.
// They are the only net/* imports allowed in plugin domain code — an importer
// that parses feed URLs (OPML/podcast migrations) needs net/url, and net/netip
// is an address value type. Every other net/* package can open a socket
// (net/rpc and net/textproto dial, net/http/httputil proxies), so
// networkCapableImport denies them. "net/http" is deliberately absent: it keeps
// its dedicated scrobble.Adapter-seam rule rather than being a flat allow.
var nonNetworkNetPackages = map[string]bool{
	"net/url":   true,
	"net/netip": true,
}

// networkCapableImport reports whether importPath is a network-capable net/*
// package that plugin domain code (including an importer's Source) may not
// import. It is the transitive-reach half of the gate: because checkAllowedImports
// walks the whole module, a network package imported through a local helper
// package still reds — the importer never has to touch the network directly for
// the capability to be present. "net" and "net/smtp" carry their own
// deniedImports messages and "net/http" its adapter-seam rule, so this predicate
// only needs to catch the remaining net/* tree (net/rpc, net/textproto,
// net/http/httputil, …) while exempting the parse-only value packages.
func networkCapableImport(importPath string) bool {
	if importPath == "net/http" || nonNetworkNetPackages[importPath] {
		return false
	}
	return strings.HasPrefix(importPath, "net/")
}

// osProcessCalls are os package functions that spawn or attach to processes —
// banned alongside os/exec, which they would otherwise sidestep.
var osProcessCalls = map[string]bool{
	"FindProcess":  true,
	"StartProcess": true,
}

// RequireAllowedImports is a targeted static gate over the calling module's
// non-test Go files, NOT a full dependency allowlist (third-party modules are
// reviewed via go.mod in registry admission). It fails when plugin
// domain code:
//
//   - imports a denied package: "net", "net/smtp", "os/exec", "plugin",
//     "syscall", or "unsafe";
//   - imports any other network-capable net/* package (net/rpc, net/textproto,
//     net/http/httputil, …) anywhere — the transitive-reach guard, since a
//     module-wide walk catches such a package pulled in through a local helper;
//     only the parse-only value packages net/url and net/netip are permitted;
//   - imports "net/http" outside a registered scrobble.Adapter seam file, or
//     references an HTTP outbound function (call or captured function value)
//     outside Adapter.Submit;
//   - constructs an http.Client/http.Transport or touches
//     http.DefaultClient/http.DefaultTransport anywhere — adapters must use
//     the injected scrobble Doer, which carries the endpoint allowlist;
//   - references a filesystem-mutating os function (or ioutil.WriteFile)
//     outside SDK-declared hook seams, including capturing it as a function
//     value, or calls os.StartProcess/os.FindProcess;
//   - uses a dot import, which would defeat the selector-based checks above.
//
// The gate is syntactic and per-file: it cannot see through local aliasing of
// package identifiers to variables, code generation, or what a third-party
// dependency does internally. It is one admission gate among several, not a
// sandbox.
func RequireAllowedImports(t testing.TB) {
	t.Helper()
	root, ok, err := externalCallerModuleRoot()
	if err != nil {
		t.Fatalf("sdktest allowed imports: %v", err)
	}
	if !ok {
		return
	}
	if err := checkAllowedImports(root); err != nil {
		t.Fatal(err)
	}
}

func externalCallerModuleRoot() (string, bool, error) {
	pcs := make([]uintptr, 256)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.File, "_test.go") {
			root, modulePath, err := findModuleRoot(filepath.Dir(frame.File))
			if err == nil && modulePath != sdkModulePath {
				return root, true, nil
			}
		}
		if !more {
			return "", false, nil
		}
	}
}

func findModuleRoot(dir string) (string, string, error) {
	for {
		mod := filepath.Join(dir, "go.mod")
		data, err := os.ReadFile(mod)
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				fields := strings.Fields(line)
				if len(fields) == 2 && fields[0] == "module" {
					return dir, fields[1], nil
				}
			}
			return "", "", fmt.Errorf("%s has no module directive", mod)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", fmt.Errorf("no go.mod found")
		}
		dir = parent
	}
}

func checkAllowedImports(root string) error {
	var issues []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			switch {
			case name == "vendor" || name == "testdata":
				return filepath.SkipDir
			case strings.HasPrefix(name, ".") && file != root:
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		parsed, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			rel = file
		}
		issues = append(issues, allowedImportIssues(fset, rel, parsed)...)
		return nil
	})
	if err != nil {
		return fmt.Errorf("sdktest allowed imports: %w", err)
	}
	if len(issues) == 0 {
		return nil
	}
	sort.Strings(issues)
	return fmt.Errorf("sdktest allowed imports:\n%s", strings.Join(issues, "\n"))
}

func allowedImportIssues(fset *token.FileSet, rel string, file *ast.File) []string {
	imports := map[string]string{}
	var issues []string
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := importedName(spec, importPath)
		if name != "" {
			imports[name] = importPath
		}
	}
	adapterTypes := scrobbleAdapterTypes(file)
	allowsHTTP := len(adapterTypes) > 0 && declaresScrobbleAdapterFactory(file, imports)
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if spec.Name != nil && spec.Name.Name == "." {
			issues = append(issues, issueAt(fset, rel, spec.Pos(), fmt.Sprintf("dot import of %q is not allowed in plugin domain modules (it defeats the import gate)", importPath)))
			continue
		}
		switch {
		case importPath == "net/http":
			if !allowsHTTP {
				issues = append(issues, issueAt(fset, rel, spec.Pos(), `"net/http" is allowed only in registered scrobble.Adapter seam files`))
			}
		case deniedImports[importPath]:
			issues = append(issues, issueAt(fset, rel, spec.Pos(), fmt.Sprintf("%q is not allowed in plugin domain modules", importPath)))
		case networkCapableImport(importPath):
			issues = append(issues, issueAt(fset, rel, spec.Pos(), fmt.Sprintf("%q is a network-capable package; importer and plugin domain code may not open outbound connections (only net/url and net/netip are permitted from the net/* tree)", importPath)))
		}
	}
	issues = append(issues, usageIssues(fset, rel, file, imports, adapterTypes)...)
	return issues
}

// usageIssues walks the ENTIRE file (function bodies and package-level
// declarations alike) for selector references and composite literals that the
// gate bans. Inspecting every SelectorExpr — not just those in call position —
// closes the function-value-capture escape (`f := os.WriteFile; f(...)`).
func usageIssues(fset *token.FileSet, rel string, file *ast.File, imports map[string]string, adapterTypes map[string]bool) []string {
	var issues []string
	submitBodies := adapterSubmitBodies(file, adapterTypes)
	inSubmit := func(pos token.Pos) bool {
		for _, body := range submitBodies {
			if pos >= body.Pos() && pos <= body.End() {
				return true
			}
		}
		return false
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			if isNetHTTPSelector(node.Type, imports, "Client") || isNetHTTPSelector(node.Type, imports, "Transport") {
				issues = append(issues, issueAt(fset, rel, node.Pos(), "constructing an http.Client/http.Transport is not allowed; adapters must use the injected scrobble Doer"))
			}
		case *ast.SelectorExpr:
			if isNetHTTPSelector(node, imports, "DefaultClient") || isNetHTTPSelector(node, imports, "DefaultTransport") {
				issues = append(issues, issueAt(fset, rel, node.Sel.Pos(), "http.DefaultClient/http.DefaultTransport are not allowed; adapters must use the injected scrobble Doer"))
				return true
			}
			if isHTTPOutboundCall(node, imports) && !inSubmit(node.Pos()) {
				issues = append(issues, issueAt(fset, rel, node.Sel.Pos(), "HTTP outbound calls are allowed only inside scrobble.Adapter.Submit"))
				return true
			}
			recv, ok := node.X.(*ast.Ident)
			if !ok {
				return true
			}
			switch imports[recv.Name] {
			case "os":
				if fsWriteCalls[node.Sel.Name] {
					issues = append(issues, issueAt(fset, rel, node.Sel.Pos(), "filesystem writes are not allowed outside SDK-declared hook seams"))
				}
				if osProcessCalls[node.Sel.Name] {
					issues = append(issues, issueAt(fset, rel, node.Sel.Pos(), "process control (os.StartProcess/os.FindProcess) is not allowed in plugin domain modules"))
				}
			case "io/ioutil":
				if node.Sel.Name == "WriteFile" {
					issues = append(issues, issueAt(fset, rel, node.Sel.Pos(), "filesystem writes are not allowed outside SDK-declared hook seams"))
				}
			}
		}
		return true
	})
	return issues
}

// adapterSubmitBodies returns the body node of every Submit method declared on
// a registered scrobble adapter type in this file.
func adapterSubmitBodies(file *ast.File, adapterTypes map[string]bool) []*ast.BlockStmt {
	var bodies []*ast.BlockStmt
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if fn.Name.Name == "Submit" && adapterTypes[funcReceiverTypeName(fn)] {
			bodies = append(bodies, fn.Body)
		}
	}
	return bodies
}

// isNetHTTPSelector reports whether expr is the selector <net/http pkg>.<name>.
func isNetHTTPSelector(expr ast.Expr, imports map[string]string, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && imports[pkg.Name] == "net/http"
}

func importedName(spec *ast.ImportSpec, importPath string) string {
	if spec.Name != nil {
		if spec.Name.Name == "_" || spec.Name.Name == "." {
			return ""
		}
		return spec.Name.Name
	}
	return path.Base(importPath)
}

func scrobbleAdapterTypes(file *ast.File) map[string]bool {
	methods := map[string]map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		typ := receiverTypeName(fn.Recv.List[0].Type)
		if typ == "" {
			continue
		}
		if methods[typ] == nil {
			methods[typ] = map[string]bool{}
		}
		methods[typ][fn.Name.Name] = true
	}
	out := map[string]bool{}
	for typ, set := range methods {
		if set["Service"] && set["Class"] && set["IdentitySchemes"] && set["DeclaredEndpoints"] && set["Submit"] {
			out[typ] = true
		}
	}
	return out
}

func declaresScrobbleAdapterFactory(file *ast.File, imports map[string]string) bool {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Type.Results == nil {
			continue
		}
		for _, result := range fn.Type.Results.List {
			if isScrobbleAdapterSelector(result.Type, imports) {
				return true
			}
		}
	}
	return false
}

func isScrobbleAdapterSelector(expr ast.Expr, imports map[string]string) bool {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		pkg, ok := t.X.(*ast.Ident)
		return ok && imports[pkg.Name] == "github.com/beamfall/plugin-sdk/scrobble" && t.Sel.Name == "Adapter"
	case *ast.ParenExpr:
		return isScrobbleAdapterSelector(t.X, imports)
	default:
		return false
	}
}

func isHTTPOutboundCall(sel *ast.SelectorExpr, imports map[string]string) bool {
	if recv, ok := sel.X.(*ast.Ident); ok && imports[recv.Name] == "net/http" {
		return httpOutboundCalls[sel.Sel.Name]
	}
	if sel.Sel.Name != "Do" {
		return false
	}
	recv, ok := sel.X.(*ast.SelectorExpr)
	if !ok || recv.Sel.Name != "DefaultClient" {
		return false
	}
	pkg, ok := recv.X.(*ast.Ident)
	return ok && imports[pkg.Name] == "net/http"
}

func funcReceiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	return receiverTypeName(fn.Recv.List[0].Type)
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	default:
		return ""
	}
}

func issueAt(fset *token.FileSet, rel string, pos token.Pos, msg string) string {
	p := fset.Position(pos)
	return fmt.Sprintf("%s:%d: %s", rel, p.Line, msg)
}
