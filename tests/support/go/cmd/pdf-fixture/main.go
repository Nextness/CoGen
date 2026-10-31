// Command pdf-fixture writes a deterministic PDF fixture to standard output.
package main

import (
	"flag"
	"os"

	"analysis/tests/support/go/pdffixture"
)

// main writes one deterministic PDF fixture to standard output.
func main() {
	complexFixture := flag.Bool("complex", false, "write a complex fixture with crop origins, rotation, user units, and an existing annotation")
	flag.Parse()
	texts := flag.Args()
	if len(texts) == 0 {
		texts = []string{"Selectable fixture PDF text"}
	}
	var data []byte
	if *complexFixture {
		data = pdffixture.ComplexBytes(texts...)
	} else {
		data = pdffixture.Bytes(texts...)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		os.Exit(1)
	}
}
