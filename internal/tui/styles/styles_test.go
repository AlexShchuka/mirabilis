package styles

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestHuhThemeBuilds(t *testing.T) {
	for _, dark := range []bool{true, false} {
		if s := HuhTheme().Theme(dark); s == nil {
			t.Fatalf("HuhTheme().Theme(%v) = nil", dark)
		}
		if s := HuhThemeDanger().Theme(dark); s == nil {
			t.Fatalf("HuhThemeDanger().Theme(%v) = nil", dark)
		}
	}
}

func TestDangerThemeDiffersFromBase(t *testing.T) {
	base := HuhTheme().Theme(true)
	danger := HuhThemeDanger().Theme(true)
	if base.Focused.FocusedButton.GetBackground() == danger.Focused.FocusedButton.GetBackground() {
		t.Error("danger theme affirmative button shares background with the base theme; destructive action must read as dangerous")
	}
}

func TestNoBackgroundInRecoloredStyles(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "styles.go", nil, 0)
	if err != nil {
		t.Fatalf("parse styles.go: %v", err)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != "Background" {
			return true
		}
		pos := fset.Position(call.Pos())
		if isDangerButton(f, call) {
			return true
		}
		t.Errorf("%s: .Background( call found in recolored styles — host bg must be inherited (§3 фон не трогаем)", pos)
		return true
	})
}

func isDangerButton(f *ast.File, target *ast.CallExpr) bool {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "HuhThemeDanger" {
			continue
		}
		found := false
		ast.Inspect(fn, func(n ast.Node) bool {
			if n == target {
				found = true
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}
