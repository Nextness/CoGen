package store

// Store persists PDF selections.
type Store struct{}

// Save stores one PDF selection.
func (s *Store) Save(id string) error {
	return nil
}

// Delete removes one PDF selection. It has no callers in this fixture and is
// used as the empty-result case for caller queries.
func (s *Store) Delete(id string) error {
	return nil
}
