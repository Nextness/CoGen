// mutation.go implements conditional save and removal for the companion PDF
// store. Every changed operation runs in one immediate companion transaction,
// validates the expected document counter, inserts or reuses a verified blob,
// moves the selection with compare-and-swap, deletes outgoing bytes when
// unreferenced, and records one durable outbox event before commit.
package pdfstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"analysis/backend/database/work"
	"analysis/backend/manifest"
	generated "analysis/backend/pdfstore/internal/sql"
)

var (
	// ErrPDFInvalid identifies a malformed save or removal request.
	ErrPDFInvalid = errors.New("PDF mutation is invalid")
	// ErrPDFNotRegistered identifies a DOI without a companion inventory row.
	ErrPDFNotRegistered = errors.New("PDF document is not registered")
	// ErrPDFStateConflict identifies a stale expected document change counter.
	ErrPDFStateConflict = errors.New("PDF document state conflict")
	// ErrPDFIntegrity identifies stored content that fails its size or digest checks.
	ErrPDFIntegrity = errors.New("PDF document integrity failure")
)

// MutationOperation names the observable result of a save or removal.
type MutationOperation string

const (
	// OperationUploaded selects content for an unavailable registration.
	OperationUploaded MutationOperation = "uploaded"
	// OperationReplaced changes an available registration's selected content.
	OperationReplaced MutationOperation = "replaced"
	// OperationRemoved clears an available registration's selected content.
	OperationRemoved MutationOperation = "removed"
	// OperationUnchanged reports an idempotent save or removal.
	OperationUnchanged MutationOperation = "unchanged"
)

// SaveInput identifies one conditional manual PDF save.
type SaveInput struct {
	DOI                   string
	WorkID                int64
	ExpectedChangeCounter int64
	InitiatingRunID       int64
	InitiatingRevisionID  int64
	Data                  []byte
}

// SaveResult reports one save's outcome and current document state.
type SaveResult struct {
	Changed         bool
	Operation       MutationOperation
	Document        Document
	ByteSize        int64
	EventKey        string
	OutgoingDeleted bool
}

// RemoveInput identifies one conditional manual PDF removal.
type RemoveInput struct {
	DOI                   string
	WorkID                int64
	ExpectedChangeCounter int64
	InitiatingRunID       int64
	InitiatingRevisionID  int64
}

// RemoveResult reports one removal's outcome and current document state.
type RemoveResult struct {
	Changed         bool
	Operation       MutationOperation
	Document        Document
	EventKey        string
	OutgoingDeleted bool
}

// StateConflictError reports a stale expected counter with the current state.
type StateConflictError struct {
	Expected int64
	State    Document
}

// Error formats the stale-counter conflict without exposing document bytes.
func (e *StateConflictError) Error() string {
	return fmt.Sprintf("%s: expected change counter %d, current %d", ErrPDFStateConflict, e.Expected, e.State.ChangeCounter)
}

// Unwrap supports errors.Is(err, ErrPDFStateConflict).
func (e *StateConflictError) Unwrap() error { return ErrPDFStateConflict }

// Save validates complete PDF bytes and conditionally uploads or replaces one
// document selection. Identical bytes are a verified no-op that leaves the
// counter, timestamps, bytes, and outbox unchanged.
func (s *Store) Save(ctx context.Context, input SaveInput) (SaveResult, error) {
	doi := work.NormalizeDOI(input.DOI)
	if doi == "" {
		return SaveResult{}, fmt.Errorf("%w: DOI is required", ErrPDFInvalid)
	}
	if input.WorkID <= 0 {
		return SaveResult{}, fmt.Errorf("%w: work ID must be positive", ErrPDFInvalid)
	}
	if input.ExpectedChangeCounter <= 0 {
		return SaveResult{}, fmt.Errorf("%w: expected change counter must be positive", ErrPDFInvalid)
	}
	if input.InitiatingRunID < 0 || input.InitiatingRevisionID < 0 || (input.InitiatingRunID > 0) != (input.InitiatingRevisionID > 0) {
		return SaveResult{}, fmt.Errorf("%w: initiating run and revision context must be provided together", ErrPDFInvalid)
	}
	hash, err := ValidatePDF(input.Data, DefaultMaxPDFBytes)
	if err != nil {
		return SaveResult{}, fmt.Errorf("%w: %w", ErrPDFInvalid, err)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return SaveResult{}, err
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)

	current, err := queries.GetPDFDocument(ctx, doi)
	if err == sql.ErrNoRows {
		return SaveResult{}, fmt.Errorf("%w: DOI %q", ErrPDFNotRegistered, doi)
	}
	if err != nil {
		return SaveResult{}, fmt.Errorf("read PDF document: %w", err)
	}
	state := documentFromRow(current)
	if state.ChangeCounter != input.ExpectedChangeCounter {
		return SaveResult{}, &StateConflictError{Expected: input.ExpectedChangeCounter, State: state}
	}
	if state.Status != StatusNotAvailable && state.Status != StatusAvailable {
		return SaveResult{}, fmt.Errorf("PDF inventory document %q has unsupported status %q", doi, state.Status)
	}
	if state.Status == StatusAvailable && state.ContentHash == hash {
		size, err := storedBlobSize(ctx, queries, hash)
		if err != nil {
			return SaveResult{}, err
		}
		data, err := storedBlobData(ctx, queries, hash)
		if err != nil {
			return SaveResult{}, err
		}
		if err := validateStoredBlob(hash, size, data); err != nil {
			return SaveResult{}, err
		}
		return SaveResult{Changed: false, Operation: OperationUnchanged, Document: state, ByteSize: size}, nil
	}

	var beforeSize *int64
	if state.Status == StatusAvailable && state.ContentHash != "" {
		beforeSize, err = optionalBlobSize(ctx, queries, state.ContentHash)
		if err != nil {
			return SaveResult{}, err
		}
	}
	now := timestamp(s.now())
	if err := queries.InsertPDFBlob(ctx, generated.InsertPDFBlobParams{
		ContentHash: hash, ByteSize: int64(len(input.Data)), Data: input.Data, CreatedAt: now,
	}); err != nil {
		return SaveResult{}, fmt.Errorf("insert PDF blob: %w", err)
	}
	size, err := storedBlobSize(ctx, queries, hash)
	if err != nil {
		return SaveResult{}, err
	}
	data, err := storedBlobData(ctx, queries, hash)
	if err != nil {
		return SaveResult{}, err
	}
	if err := validateStoredBlob(hash, size, data); err != nil {
		return SaveResult{}, err
	}

	result, err := queries.MarkPDFDocumentAvailable(ctx, generated.MarkPDFDocumentAvailableParams{
		ContentHash:     sql.NullString{String: hash, Valid: true},
		InventoriedAt:   sql.NullString{String: now, Valid: true},
		UpdatedAt:       now,
		Doi:             doi,
		ExpectedCounter: input.ExpectedChangeCounter,
	})
	if err != nil {
		return SaveResult{}, fmt.Errorf("store PDF document: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return SaveResult{}, fmt.Errorf("read PDF document update result: %w", err)
	} else if affected != 1 {
		concurrent, err := queries.GetPDFDocument(ctx, doi)
		if err == sql.ErrNoRows {
			return SaveResult{}, fmt.Errorf("%w: DOI %q", ErrPDFNotRegistered, doi)
		}
		if err != nil {
			return SaveResult{}, fmt.Errorf("read concurrent PDF document state: %w", err)
		}
		return SaveResult{}, &StateConflictError{Expected: input.ExpectedChangeCounter, State: documentFromRow(concurrent)}
	}

	deleted := false
	if state.Status == StatusAvailable && state.ContentHash != "" && state.ContentHash != hash {
		result, err := queries.DeletePDFBlobIfUnreferenced(ctx, state.ContentHash)
		if err != nil {
			return SaveResult{}, fmt.Errorf("delete outgoing PDF blob: %w", err)
		}
		deleted, err = rowsChanged(result)
		if err != nil {
			return SaveResult{}, fmt.Errorf("read outgoing PDF blob deletion result: %w", err)
		}
	}

	operation := OperationUploaded
	action := manifest.AuditPDFDocumentInventoried
	if state.Status == StatusAvailable {
		operation = OperationReplaced
		action = manifest.AuditPDFDocumentReplaced
	}
	afterState := Document{
		DOI: doi, Status: StatusAvailable, ContentHash: hash,
		InventoriedAt: now, UpdatedAt: now, ChangeCounter: input.ExpectedChangeCounter + 1,
	}
	beforeJSON, err := marshalEvidence(evidenceFromDocument(state, beforeSize, nil))
	if err != nil {
		return SaveResult{}, err
	}
	var outgoingDeleted *bool
	if state.Status == StatusAvailable {
		outgoingDeleted = &deleted
	}
	afterJSON, err := marshalEvidence(evidenceFromDocument(afterState, &size, outgoingDeleted))
	if err != nil {
		return SaveResult{}, err
	}
	eventKey, err := s.recordMutation(ctx, queries, mutationEvent{
		DOI: doi, WorkID: input.WorkID, Action: action, Operation: operation,
		InitiatingRunID: input.InitiatingRunID, InitiatingRevisionID: input.InitiatingRevisionID,
		BeforeJSON: beforeJSON, AfterJSON: afterJSON,
	}, now)
	if err != nil {
		return SaveResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return SaveResult{}, fmt.Errorf("commit PDF document save: %w", err)
	}
	return SaveResult{
		Changed: true, Operation: operation, Document: afterState, ByteSize: int64(len(input.Data)),
		EventKey: eventKey, OutgoingDeleted: deleted,
	}, nil
}

// Remove conditionally clears one available document selection while retaining
// its registration. Removing an already unavailable document with the current
// counter is a no-op that leaves timestamps, counter, and outbox unchanged.
func (s *Store) Remove(ctx context.Context, input RemoveInput) (RemoveResult, error) {
	doi := work.NormalizeDOI(input.DOI)
	if doi == "" {
		return RemoveResult{}, fmt.Errorf("%w: DOI is required", ErrPDFInvalid)
	}
	if input.WorkID <= 0 {
		return RemoveResult{}, fmt.Errorf("%w: work ID must be positive", ErrPDFInvalid)
	}
	if input.ExpectedChangeCounter <= 0 {
		return RemoveResult{}, fmt.Errorf("%w: expected change counter must be positive", ErrPDFInvalid)
	}
	if input.InitiatingRunID < 0 || input.InitiatingRevisionID < 0 || (input.InitiatingRunID > 0) != (input.InitiatingRevisionID > 0) {
		return RemoveResult{}, fmt.Errorf("%w: initiating run and revision context must be provided together", ErrPDFInvalid)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return RemoveResult{}, err
	}
	defer tx.Rollback()
	queries := s.queries.WithTx(tx)

	current, err := queries.GetPDFDocument(ctx, doi)
	if err == sql.ErrNoRows {
		return RemoveResult{}, fmt.Errorf("%w: DOI %q", ErrPDFNotRegistered, doi)
	}
	if err != nil {
		return RemoveResult{}, fmt.Errorf("read PDF document: %w", err)
	}
	state := documentFromRow(current)
	if state.ChangeCounter != input.ExpectedChangeCounter {
		return RemoveResult{}, &StateConflictError{Expected: input.ExpectedChangeCounter, State: state}
	}
	if state.Status == StatusNotAvailable {
		return RemoveResult{Changed: false, Operation: OperationUnchanged, Document: state}, nil
	}
	if state.Status != StatusAvailable {
		return RemoveResult{}, fmt.Errorf("PDF inventory document %q has unsupported status %q", doi, state.Status)
	}

	var beforeSize *int64
	if state.ContentHash != "" {
		beforeSize, err = optionalBlobSize(ctx, queries, state.ContentHash)
		if err != nil {
			return RemoveResult{}, err
		}
	}
	now := timestamp(s.now())
	result, err := queries.ClearPDFDocumentSelection(ctx, generated.ClearPDFDocumentSelectionParams{
		UpdatedAt: now, Doi: doi, ExpectedCounter: input.ExpectedChangeCounter,
	})
	if err != nil {
		return RemoveResult{}, fmt.Errorf("clear PDF document selection: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return RemoveResult{}, fmt.Errorf("read PDF document removal result: %w", err)
	} else if affected != 1 {
		concurrent, err := queries.GetPDFDocument(ctx, doi)
		if err == sql.ErrNoRows {
			return RemoveResult{}, fmt.Errorf("%w: DOI %q", ErrPDFNotRegistered, doi)
		}
		if err != nil {
			return RemoveResult{}, fmt.Errorf("read concurrent PDF document state: %w", err)
		}
		return RemoveResult{}, &StateConflictError{Expected: input.ExpectedChangeCounter, State: documentFromRow(concurrent)}
	}

	deleted := false
	if state.ContentHash != "" {
		result, err := queries.DeletePDFBlobIfUnreferenced(ctx, state.ContentHash)
		if err != nil {
			return RemoveResult{}, fmt.Errorf("delete outgoing PDF blob: %w", err)
		}
		deleted, err = rowsChanged(result)
		if err != nil {
			return RemoveResult{}, fmt.Errorf("read outgoing PDF blob deletion result: %w", err)
		}
	}
	afterState := Document{DOI: doi, Status: StatusNotAvailable, UpdatedAt: now, ChangeCounter: input.ExpectedChangeCounter + 1}
	beforeJSON, err := marshalEvidence(evidenceFromDocument(state, beforeSize, nil))
	if err != nil {
		return RemoveResult{}, err
	}
	afterJSON, err := marshalEvidence(evidenceFromDocument(afterState, nil, &deleted))
	if err != nil {
		return RemoveResult{}, err
	}
	eventKey, err := s.recordMutation(ctx, queries, mutationEvent{
		DOI: doi, WorkID: input.WorkID, Action: manifest.AuditPDFDocumentRemoved,
		Operation: OperationRemoved, InitiatingRunID: input.InitiatingRunID,
		InitiatingRevisionID: input.InitiatingRevisionID, BeforeJSON: beforeJSON, AfterJSON: afterJSON,
	}, now)
	if err != nil {
		return RemoveResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return RemoveResult{}, fmt.Errorf("commit PDF document removal: %w", err)
	}
	return RemoveResult{
		Changed: true, Operation: OperationRemoved, Document: afterState,
		EventKey: eventKey, OutgoingDeleted: deleted,
	}, nil
}

// mutationEvent is the bounded operational evidence recorded for one changed PDF save or removal.
type mutationEvent struct {
	DOI                  string
	WorkID               int64
	Action               manifest.AuditAction
	Operation            MutationOperation
	InitiatingRunID      int64
	InitiatingRevisionID int64
	BeforeJSON           string
	AfterJSON            string
}

// recordMutation inserts one user-scoped outbox event and returns its event key.
func (s *Store) recordMutation(ctx context.Context, queries *generated.Queries, event mutationEvent, occurredAt string) (string, error) {
	eventKey, err := newCorrelationID()
	if err != nil {
		return "", err
	}
	correlationID, err := newCorrelationID()
	if err != nil {
		return "", err
	}
	metadata := map[string]any{
		"source": "manual", "doi": event.DOI, "operation": string(event.Operation),
	}
	if event.InitiatingRunID > 0 {
		metadata["initiating_run_id"] = event.InitiatingRunID
	}
	if event.InitiatingRevisionID > 0 {
		metadata["initiating_revision_id"] = event.InitiatingRevisionID
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	if err := insertOutbox(ctx, queries, OutboxEvent{
		EventKey: eventKey, Actor: "user", EntityType: "work",
		EntityID: strconv.FormatInt(event.WorkID, 10), Action: string(event.Action),
		BeforeJSON: event.BeforeJSON, AfterJSON: event.AfterJSON,
		MetadataJSON: string(encoded), CorrelationID: correlationID,
	}, occurredAt); err != nil {
		return "", err
	}
	return eventKey, nil
}

// documentEvidence is the bounded before/after state recorded in PDF audit.
type documentEvidence struct {
	Status          string `json:"status"`
	ContentHash     string `json:"content_hash,omitempty"`
	ByteSize        *int64 `json:"byte_size,omitempty"`
	InventoriedAt   string `json:"inventoried_at,omitempty"`
	ChangeCounter   int64  `json:"change_counter"`
	OutgoingDeleted *bool  `json:"outgoing_deleted,omitempty"`
}

// evidenceFromDocument maps one document state and optional byte size into audit evidence.
func evidenceFromDocument(state Document, byteSize *int64, outgoingDeleted *bool) documentEvidence {
	return documentEvidence{
		Status: state.Status, ContentHash: state.ContentHash, ByteSize: byteSize,
		InventoriedAt: state.InventoriedAt, ChangeCounter: state.ChangeCounter,
		OutgoingDeleted: outgoingDeleted,
	}
}

// marshalEvidence encodes bounded before/after operational evidence.
func marshalEvidence(evidence documentEvidence) (string, error) {
	encoded, err := json.Marshal(evidence)
	if err != nil {
		return "", fmt.Errorf("encode PDF audit evidence: %w", err)
	}
	return string(encoded), nil
}

// optionalBlobSize reads outgoing blob size metadata without reading its bytes.
func optionalBlobSize(ctx context.Context, queries *generated.Queries, hash string) (*int64, error) {
	if hash == "" {
		return nil, nil
	}
	size, err := queries.GetPDFBlobSize(ctx, hash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read outgoing PDF blob size: %w", err)
	}
	return &size, nil
}

// documentFromRow maps one generated document row into the public state.
func documentFromRow(row generated.PdfDocument) Document {
	return Document{
		DOI: row.Doi, Status: row.Status, ContentHash: row.ContentHash.String,
		InventoriedAt: row.InventoriedAt.String, UpdatedAt: row.UpdatedAt,
		ChangeCounter: row.ChangeCounter,
	}
}

// storedBlobSize reads one blob's declared size or reports an integrity failure.
func storedBlobSize(ctx context.Context, queries *generated.Queries, hash string) (int64, error) {
	size, err := queries.GetPDFBlobSize(ctx, hash)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("%w: selected blob %s is missing", ErrPDFIntegrity, hash)
	}
	if err != nil {
		return 0, fmt.Errorf("read PDF blob size: %w", err)
	}
	return size, nil
}

// storedBlobData reads one blob's bytes or reports an integrity failure.
func storedBlobData(ctx context.Context, queries *generated.Queries, hash string) ([]byte, error) {
	data, err := queries.GetPDFBlobData(ctx, hash)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: selected blob %s is missing", ErrPDFIntegrity, hash)
	}
	if err != nil {
		return nil, fmt.Errorf("read PDF blob data: %w", err)
	}
	return data, nil
}

// validateStoredBlob verifies a stored blob's size and content digest.
func validateStoredBlob(contentHash string, byteSize int64, data []byte) error {
	if contentHash == "" || byteSize < 0 {
		return fmt.Errorf("%w: stored blob metadata is incomplete", ErrPDFIntegrity)
	}
	if int64(len(data)) != byteSize {
		return fmt.Errorf("%w: stored blob byte size does not match its data", ErrPDFIntegrity)
	}
	digest := sha256.Sum256(data)
	if contentHash != hex.EncodeToString(digest[:]) {
		return fmt.Errorf("%w: stored blob digest does not match its content hash", ErrPDFIntegrity)
	}
	return nil
}

// rowsChanged reports whether one mutation affected a row.
func rowsChanged(result sql.Result) (bool, error) {
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
