/*
Copyright 2024 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deletecomplete

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// maxDelegationDepth bounds how far the check follows a helper call. One level
// covers this provider, where a few Observe methods delegate to a shared
// ObserveX helper in the same package.
const maxDelegationDepth = 2

// minExpectedObserveMethods guards against the walk silently matching nothing,
// which would make the check below vacuous. The provider has 23 controllers.
const minExpectedObserveMethods = 20

// TestObserveCanReportResourceAbsent is a structural regression guard.
//
// The reconciler removes a managed resource's finalizer only once Observe
// returns ResourceExists: false, and while Observe reports it present it re-runs
// Delete on every pass. A controller whose Observe can only ever report
// ResourceExists: true therefore never terminates when deleted - and if its
// Delete has side effects, those side effects repeat on every pass.
//
// Rather than assert behaviour per controller (which needs a live Keycloak),
// this parses the sources and checks the shape that causes the wedge: every
// ResourceExists value reachable from Observe must not be the literal `true`
// and nothing else.
//
// A controller that genuinely can never be absent belongs in the allowlist
// below, with a reason. Adding one is a deliberate act.
func TestObserveCanReportResourceAbsent(t *testing.T) {
	// Controllers whose Observe legitimately always reports present.
	allowed := map[string]string{}

	// Root is the controller directory, so each subdirectory is a controller.
	dirs, err := os.ReadDir("..")
	if err != nil {
		t.Fatalf("cannot list controller packages: %v", err)
	}

	inspected := 0

	for _, dir := range dirs {
		if !dir.IsDir() || dir.Name() == "deletecomplete" {
			continue
		}

		funcs, observes, err := parseController(dir.Name())
		if err != nil {
			t.Fatalf("cannot parse %s: %v", dir.Name(), err)
		}

		if len(observes) == 0 {
			continue
		}

		for _, fn := range observes {
			inspected++

			if canReportAbsent(fn, funcs, 0) {
				continue
			}

			if reason, ok := allowed[dir.Name()]; ok {
				t.Logf("%s: cannot report absent (%s)", dir.Name(), reason)
				continue
			}

			t.Errorf("%s: Observe can only report ResourceExists: true, so deleting this "+
				"resource never releases its finalizer", dir.Name())
		}
	}

	if inspected < minExpectedObserveMethods {
		t.Fatalf("only inspected %d Observe methods, expected at least %d - "+
			"the walk is not matching controller sources", inspected, minExpectedObserveMethods)
	}

	t.Logf("inspected %d Observe methods across the provider", inspected)
}

// parseController returns the package-level funcs of a controller package, for
// following delegation, and its Observe methods.
func parseController(dir string) (map[string]*ast.FuncDecl, []*ast.FuncDecl, error) {
	paths, err := filepath.Glob(filepath.Join("..", dir, "*.go"))
	if err != nil {
		return nil, nil, err
	}

	funcs := map[string]*ast.FuncDecl{}

	var observes []*ast.FuncDecl

	fset := token.NewFileSet()

	for _, path := range paths {
		// Test files hold their own Observe-shaped helpers and would confuse
		// both the method list and the delegation map.
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, nil, err
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			if fn.Recv == nil {
				funcs[fn.Name.Name] = fn
				continue
			}

			if fn.Name.Name == "Observe" {
				observes = append(observes, fn)
			}
		}
	}

	return funcs, observes, nil
}

// canReportAbsent reports whether a ResourceExists value reachable from fn is
// anything other than the literal identifier `true`.
//
// Only the field's value counts. In particular the `ok` from an enclosing type
// assertion says nothing about what is returned, so matching on identifier names
// elsewhere in the body would mask the bug.
func canReportAbsent(fn *ast.FuncDecl, funcs map[string]*ast.FuncDecl, depth int) bool {
	if fn.Body == nil {
		return false
	}

	found := false

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if found {
			return false
		}

		switch v := n.(type) {
		case *ast.KeyValueExpr:
			ident, ok := v.Key.(*ast.Ident)
			if !ok || ident.Name != "ResourceExists" {
				return true
			}

			// `ResourceExists: true` is the wedge. false, a variable, or a call
			// can all report absence.
			if lit, isIdent := v.Value.(*ast.Ident); isIdent && lit.Name == "true" {
				return true
			}

			found = true

			return false

		case *ast.CallExpr:
			if depth >= maxDelegationDepth {
				return true
			}

			ident, ok := v.Fun.(*ast.Ident)
			if !ok {
				return true
			}

			// Follow a same-package helper Observe delegates to, so a shared
			// ObserveX helper is judged on its own body.
			if helper, ok := funcs[ident.Name]; ok && helper != fn {
				if canReportAbsent(helper, funcs, depth+1) {
					found = true
				}
			}
		}

		return true
	})

	return found
}
