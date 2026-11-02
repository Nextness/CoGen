// reservation.go provides a cross-process companion write reservation. The
// immediate transaction holds SQLite's reserved lock so PDF replacement and
// removal cannot interleave between anchor validation and metadata commit.
package pdfstore

import (
	"context"
	"database/sql"
	"fmt"
)

// WriteReservation holds one immediate companion transaction until release.
type WriteReservation struct {
	tx *sql.Tx
}

// ReserveWrite begins one immediate companion transaction and holds its write
// reservation. Callers must release it on every exit path.
func (s *Store) ReserveWrite(ctx context.Context) (*WriteReservation, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("reserve PDF write: %w", err)
	}
	return &WriteReservation{tx: tx}, nil
}

// Release rolls back the reservation transaction and releases its SQLite lock.
func (r *WriteReservation) Release() error {
	if r == nil || r.tx == nil {
		return nil
	}
	err := r.tx.Rollback()
	r.tx = nil
	return err
}
