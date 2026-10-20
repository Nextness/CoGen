// helpers_test.go contains shared test infrastructure for bibtex tests.
// It has no build tag and is always compiled.

package testhelpers

import subject "analysis/backend/bibtex"

const SimpleBib = `@article{key1,
  author = {Alice, Bob},
  title = {Test},
  year = {2024},
  doi = {10.1234/test},
}
`

// Parser supports the package test suite's parser setup or assertions.
func Parser() *subject.Parser {
	return subject.NewParser(nil)
}
