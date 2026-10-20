// ast_unit_test.go checks the compiler AST interfaces at compile time.
package something_test

import subject "analysis/backend/something"

// TypeRef interface assertions preserve the sealed AST contract without invoking no-op marker methods.
var (
	_ subject.TypeRef = subject.PrimString
	_ subject.TypeRef = subject.TypeName("test")
	_ subject.TypeRef = (*subject.EnumType)(nil)
	_ subject.TypeRef = (*subject.SetupType)(nil)
	_ subject.TypeRef = (*subject.MappingType)(nil)
	_ subject.TypeRef = (*subject.ArrayType)(nil)
	_ subject.TypeRef = (*subject.EnumKeyType)(nil)
)
