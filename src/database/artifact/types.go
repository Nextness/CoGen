// types.go defines the artifact family application types. Generated sqlc row
// types stay inside the private internal/sql package and never cross this
// family boundary.
package artifact

// Run artifact role values for content-addressed configuration snapshots.
const (
	RunArtifactWorkspaceConfig  = "workspace_config"
	RunArtifactResolvedManifest = "resolved_manifest"
	RunArtifactInputManifest    = "input_manifest"
)

// Artifact is a content-addressed immutable payload.
type Artifact struct {
	ID          int64  `json:"id"`
	ContentHash string `json:"content_hash"`
	ByteSize    int64  `json:"byte_size"`
	ContentType string `json:"content_type"`
	CreatedAt   string `json:"created_at"`
}

// Blob stores the raw bytes for an artifact inline in the database.
type Blob struct {
	ID            int64  `json:"id"`
	ArtifactID    int64  `json:"artifact_id"`
	PipelineRunID int64  `json:"pipeline_run_id"`
	Data          []byte `json:"-"`
	CreatedAt     string `json:"created_at"`
}

// RelatedArtifact is one artifact related to a run with the relationship that
// connects it. An artifact with several relationships appears once per
// relationship.
type RelatedArtifact struct {
	Artifact
	Role   string `json:"relationship_role"`
	Detail string `json:"relationship_detail"`
}

// Content is one artifact's stored bytes with its media type and first run role.
type Content struct {
	ContentType string
	Role        string
	Data        []byte
}

// Preview is a bounded artifact prefix with its stored metadata.
type Preview struct {
	ContentType string
	ByteSize    int64
	BlobSize    int64
	Data        []byte
}

// CreateInput identifies one content identity to insert or locate.
type CreateInput struct {
	ContentHash string
	ContentType string
	ByteSize    int64
}

// CreateWithBlobInput identifies one content identity and its bytes for a run.
type CreateWithBlobInput struct {
	ContentHash   string
	ContentType   string
	ByteSize      int64
	PipelineRunID int64
	Data          []byte
}

// CreateBlobInput identifies one artifact's inline bytes.
type CreateBlobInput struct {
	ArtifactID    int64
	PipelineRunID int64
	Data          []byte
}

// LinkInput identifies one run-artifact role assignment.
type LinkInput struct {
	PipelineRunID int64
	ArtifactID    int64
	Role          string
}
