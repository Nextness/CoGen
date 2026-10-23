// evaluator_unit_test.go verifies public evaluator phase behavior and detached result ownership.
package something_test

import (
	subject "analysis/backend/something"
	"testing"
)

// TestNewEvaluatorNilCheckedProgram verifies nil and empty checked programs produce empty maps.
func TestNewEvaluatorNilCheckedProgram(t *testing.T) {
	for _, checked := range []*subject.CheckedProgram{nil, {}, {Program: &subject.Program{}}} {
		if result := subject.NewEvaluator(checked, "").Evaluate(); len(result) != 0 {
			t.Fatalf("empty program=%v", result)
		}
	}
}

// TestEvaluatorRunsCheckedAssignments verifies evaluation follows checked statements and excludes private bindings.
func TestEvaluatorRunsCheckedAssignments(t *testing.T) {
	checked, problem := checkSource(`#priv hidden:=1;public:="text";count:=2;`)
	if problem != nil {
		t.Fatal(problem)
	}
	result := subject.NewEvaluator(checked, "").Evaluate()
	if result["public"] != "text" || result["count"] != 2 || len(result) != 2 {
		t.Fatalf("result=%v", result)
	}
}
