package complexity

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
)

var generatedMarker = []byte("DO NOT EDIT.")

// IsGeneratedSource reports whether src follows Go's standard generated-file
// header convention ("// Code generated ... DO NOT EDIT." before the package clause).
func IsGeneratedSource(src []byte) bool {
	if !bytes.Contains(src, generatedMarker) {
		return false
	}
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, "", src, parser.PackageClauseOnly|parser.ParseComments)
	if err != nil || astFile == nil {
		return false
	}
	return ast.IsGenerated(astFile)
}
