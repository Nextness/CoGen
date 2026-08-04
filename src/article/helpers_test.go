// helpers_test.go contains shared test helpers.
package article

// stringSliceEqual supports the package test suite's string slice equal setup or assertions.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// as supports the package test suite's as setup or assertions.
func as[T error](err error, target *T) bool {
	v, ok := err.(T)
	if ok {
		*target = v
	}
	return ok
}
