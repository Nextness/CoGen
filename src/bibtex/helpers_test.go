// helpers_test.go contains shared test infrastructure for bibtex tests.
// It has no build tag and is always compiled.

package bibtex

const simpleBib = `@article{key1,
  author = {Alice, Bob},
  title = {Test},
  year = {2024},
  doi = {10.1234/test},
}
`

// parser supports the package test suite's parser setup or assertions.
func parser() *Parser {
	return NewParser(nil)
}
