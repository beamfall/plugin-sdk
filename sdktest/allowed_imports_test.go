package sdktest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllowedImportsRejectsDomainHTTP(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"domain.go": `package plugin

import "net/http"

var _ = http.MethodGet
`,
	})

	err := checkAllowedImports(root)
	if err == nil || !strings.Contains(err.Error(), `"net/http" is allowed only in registered scrobble.Adapter seam files`) {
		t.Fatalf("checkAllowedImports() = %v, want net/http rejection", err)
	}
}

func TestAllowedImportsRejectsFSWrites(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"domain.go": `package plugin

import "os"

func write(path string) error {
	return os.WriteFile(path, []byte("x"), 0o600)
}
`,
	})

	err := checkAllowedImports(root)
	if err == nil || !strings.Contains(err.Error(), "filesystem writes are not allowed") {
		t.Fatalf("checkAllowedImports() = %v, want filesystem write rejection", err)
	}
}

func TestAllowedImportsAllowsDeclaredScrobbleAdapterHTTP(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"adapter.go": `package plugin

import (
	"context"
	"net/http"

	"github.com/beamfall/plugin-sdk/scrobble"
)

type Adapter struct{}

func AdapterFactory() scrobble.Adapter { return Adapter{} }

func (Adapter) Service() scrobble.Service { return "example" }
func (Adapter) Class() scrobble.MediaClass { return scrobble.ClassAudio }
func (Adapter) IdentitySchemes() []string { return []string{"example"} }
func (Adapter) DeclaredEndpoints() []string { return []string{"https://api.example.test"} }
func (Adapter) Submit(context.Context, scrobble.Scrobble) error {
	_ = http.MethodPost
	return nil
}
`,
	})

	if err := checkAllowedImports(root); err != nil {
		t.Fatalf("checkAllowedImports(): %v", err)
	}
}

func TestAllowedImportsRejectsUnregisteredAdapterHTTP(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"adapter.go": `package plugin

import (
	"context"
	"net/http"

	"github.com/beamfall/plugin-sdk/scrobble"
)

type Adapter struct{}

func (Adapter) Service() scrobble.Service { return "example" }
func (Adapter) Class() scrobble.MediaClass { return scrobble.ClassAudio }
func (Adapter) IdentitySchemes() []string { return []string{"example"} }
func (Adapter) DeclaredEndpoints() []string { return []string{"https://api.example.test"} }
func (Adapter) Submit(context.Context, scrobble.Scrobble) error {
	_ = http.MethodPost
	return nil
}
`,
	})

	err := checkAllowedImports(root)
	if err == nil || !strings.Contains(err.Error(), `"net/http" is allowed only in registered scrobble.Adapter seam files`) {
		t.Fatalf("checkAllowedImports() = %v, want unregistered adapter rejection", err)
	}
}

func TestAllowedImportsRejectsHTTPOutsideAdapterSubmit(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"adapter.go": `package plugin

import (
	"context"
	"net/http"

	"github.com/beamfall/plugin-sdk/scrobble"
)

type Adapter struct{}

func AdapterFactory() scrobble.Adapter { return Adapter{} }

func (Adapter) Service() scrobble.Service { return "example" }
func (Adapter) Class() scrobble.MediaClass { return scrobble.ClassAudio }
func (Adapter) IdentitySchemes() []string { return []string{"example"} }
func (Adapter) DeclaredEndpoints() []string { return []string{"https://api.example.test"} }
func (Adapter) Submit(context.Context, scrobble.Scrobble) error { return nil }

func stray() {
	_, _ = http.Get("https://api.example.test")
}
`,
	})

	err := checkAllowedImports(root)
	if err == nil || !strings.Contains(err.Error(), "HTTP outbound calls are allowed only inside scrobble.Adapter.Submit") {
		t.Fatalf("checkAllowedImports() = %v, want stray HTTP rejection", err)
	}
}

// TestAllowedImportsRejectsDotImport: a dot import would strip the package
// qualifier the selector-based checks key on, so the gate rejects it outright.
func TestAllowedImportsRejectsDotImport(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"domain.go": `package plugin

import . "strings"

var _ = ToUpper
`,
	})

	err := checkAllowedImports(root)
	if err == nil || !strings.Contains(err.Error(), `dot import of "strings" is not allowed in plugin domain modules`) {
		t.Fatalf("checkAllowedImports() = %v, want dot import rejection", err)
	}
}

// TestAllowedImportsRejectsFSWriteCapture: capturing os.WriteFile as a
// function value — never calling it — must still be rejected; the whole-file
// selector walk closes the function-value-capture escape.
func TestAllowedImportsRejectsFSWriteCapture(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"domain.go": `package plugin

import "os"

func capture() {
	f := os.WriteFile
	_ = f
}
`,
	})

	err := checkAllowedImports(root)
	if err == nil || !strings.Contains(err.Error(), "filesystem writes are not allowed") {
		t.Fatalf("checkAllowedImports() = %v, want function-value capture rejection", err)
	}
}

// TestAllowedImportsRejectsHTTPClientLiteralInAdapterFile: even a registered
// scrobble.Adapter seam file may not construct its own http.Client at package
// level — the injected scrobble Doer carries the endpoint allowlist a private
// client would bypass.
func TestAllowedImportsRejectsHTTPClientLiteralInAdapterFile(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"adapter.go": `package plugin

import (
	"context"
	"net/http"

	"github.com/beamfall/plugin-sdk/scrobble"
)

var client = &http.Client{}

type Adapter struct{}

func AdapterFactory() scrobble.Adapter { return Adapter{} }

func (Adapter) Service() scrobble.Service { return "example" }
func (Adapter) Class() scrobble.MediaClass { return scrobble.ClassAudio }
func (Adapter) IdentitySchemes() []string { return []string{"example"} }
func (Adapter) DeclaredEndpoints() []string { return []string{"https://api.example.test"} }
func (Adapter) Submit(context.Context, scrobble.Scrobble) error {
	_ = client
	return nil
}
`,
	})

	err := checkAllowedImports(root)
	if err == nil || !strings.Contains(err.Error(), "constructing an http.Client/http.Transport is not allowed") {
		t.Fatalf("checkAllowedImports() = %v, want http.Client literal rejection", err)
	}
}

// TestAllowedImportsRejectsDefaultClientReference: referencing
// http.DefaultClient anywhere — even in a registered adapter seam file — is
// rejected; the shared default client carries no endpoint allowlist.
func TestAllowedImportsRejectsDefaultClientReference(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"adapter.go": `package plugin

import (
	"context"
	"net/http"

	"github.com/beamfall/plugin-sdk/scrobble"
)

var _ = http.DefaultClient

type Adapter struct{}

func AdapterFactory() scrobble.Adapter { return Adapter{} }

func (Adapter) Service() scrobble.Service { return "example" }
func (Adapter) Class() scrobble.MediaClass { return scrobble.ClassAudio }
func (Adapter) IdentitySchemes() []string { return []string{"example"} }
func (Adapter) DeclaredEndpoints() []string { return []string{"https://api.example.test"} }
func (Adapter) Submit(context.Context, scrobble.Scrobble) error { return nil }
`,
	})

	err := checkAllowedImports(root)
	if err == nil || !strings.Contains(err.Error(), "http.DefaultClient/http.DefaultTransport are not allowed") {
		t.Fatalf("checkAllowedImports() = %v, want http.DefaultClient rejection", err)
	}
}

// TestAllowedImportsRejectsDeniedImports: the flatly denied packages are
// rejected regardless of file — raw sockets, syscalls, and the type-safety
// escape each red the gate on the import alone.
func TestAllowedImportsRejectsDeniedImports(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"net", `package plugin

import "net"

var _ = net.IPv4len
`},
		{"syscall", `package plugin

import "syscall"

var _ = syscall.Getpid
`},
		{"unsafe", `package plugin

import "unsafe"

var _ = unsafe.Sizeof(0)
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeAllowedImportFixture(t, map[string]string{
				"go.mod":    "module example.com/plugin\n\ngo 1.26.4\n",
				"domain.go": tc.source,
			})

			err := checkAllowedImports(root)
			want := fmt.Sprintf("%q is not allowed in plugin domain modules", tc.name)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("checkAllowedImports() = %v, want %s", err, want)
			}
		})
	}
}

// TestAllowedImportsRejectsProcessControl: os.StartProcess (and FindProcess)
// would sidestep the os/exec import ban, so referencing either — here as a
// package-level function value — is rejected.
func TestAllowedImportsRejectsProcessControl(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"domain.go": `package plugin

import "os"

var start = os.StartProcess
`,
	})

	err := checkAllowedImports(root)
	if err == nil || !strings.Contains(err.Error(), "process control (os.StartProcess/os.FindProcess) is not allowed") {
		t.Fatalf("checkAllowedImports() = %v, want process control rejection", err)
	}
}

// TestAllowedImportsAcceptsCleanDomainFile: a domain file touching none of
// the banned surfaces (reads are fine, writes are not) passes the gate.
func TestAllowedImportsAcceptsCleanDomainFile(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"domain.go": `package plugin

import (
	"os"
	"strings"
)

func clean(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(string(data)), nil
}
`,
	})

	if err := checkAllowedImports(root); err != nil {
		t.Fatalf("checkAllowedImports() = %v, want clean fixture accepted", err)
	}
}

// TestAllowedImportsRejectsNetworkCapableNetPackages: the broadened net/* guard
// reds every network-capable net subpackage beyond the enumerated net/net/smtp —
// net/rpc, net/textproto, and net/http/httputil each open or proxy connections.
func TestAllowedImportsRejectsNetworkCapableNetPackages(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{"net/rpc", `package plugin

import "net/rpc"

var _ = rpc.DefaultRPCPath
`},
		{"net/textproto", `package plugin

import "net/textproto"

var _ = textproto.TrimString
`},
		{"net/http/httputil", `package plugin

import "net/http/httputil"

var _ = httputil.ErrClosed
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := writeAllowedImportFixture(t, map[string]string{
				"go.mod":    "module example.com/plugin\n\ngo 1.26.4\n",
				"domain.go": tc.source,
			})

			err := checkAllowedImports(root)
			if err == nil || !strings.Contains(err.Error(), "is a network-capable package") {
				t.Fatalf("checkAllowedImports() = %v, want %s network-capable rejection", err, tc.name)
			}
		})
	}
}

// TestAllowedImportsAllowsParseOnlyNetPackages: net/url and net/netip carry no
// dialer, so the value/parse-only net packages an importer legitimately needs
// (feed-URL parsing) pass the broadened guard.
func TestAllowedImportsAllowsParseOnlyNetPackages(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"domain.go": `package plugin

import (
	"net/netip"
	"net/url"
)

func parse(raw string) (*url.URL, error) { return url.Parse(raw) }

var _ = netip.IPv4Unspecified
`,
	})

	if err := checkAllowedImports(root); err != nil {
		t.Fatalf("checkAllowedImports() = %v, want net/url + net/netip accepted", err)
	}
}

func writeAllowedImportFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, contents := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
