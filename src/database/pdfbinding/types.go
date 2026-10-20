// types.go defines the application types exchanged with the metadata
// PDF-binding family.
package pdfbinding

// DeliveredEvent carries one companion outbox event awaiting metadata audit delivery.
type DeliveredEvent struct {
	EventKey      string
	OccurredAt    string
	Actor         string
	PipelineRunID int64
	EntityType    string
	EntityID      string
	Action        string
	MetadataJSON  string
	CorrelationID string
}
