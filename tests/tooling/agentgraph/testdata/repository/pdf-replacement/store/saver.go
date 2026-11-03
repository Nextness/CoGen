package store

// Saver is an interface for saving PDF selections. CallSaver invokes Save
// through this interface, so the concrete target is unresolved.
type Saver interface {
	Save(id string) error
}

// CallSaver invokes Save through an interface whose concrete target is unknown.
func CallSaver(s Saver, id string) error {
	return s.Save(id)
}
