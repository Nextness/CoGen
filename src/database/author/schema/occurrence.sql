-- name: InsertAuthorOccurrence :execresult
INSERT INTO author_occurrences (
    person_id,
    citation_name,
    first_name,
    last_name,
    orcid
) VALUES (
    sqlc.arg(person_id),
    sqlc.arg(citation_name),
    sqlc.arg(first_name),
    sqlc.arg(last_name),
    sqlc.arg(orcid)
);

-- name: GetAuthorOccurrenceByID :one
SELECT
    id,
    person_id,
    citation_name,
    first_name,
    last_name,
    orcid,
    created_at
FROM author_occurrences
WHERE id = sqlc.arg(id);

-- name: ListAuthorOccurrencesByPersonID :many
SELECT
    id,
    person_id,
    citation_name,
    first_name,
    last_name,
    orcid,
    created_at
FROM author_occurrences
WHERE person_id = sqlc.arg(person_id)
ORDER BY id;

-- name: GetAuthorOccurrenceForRun :one
SELECT
    occurrence.id,
    occurrence.person_id,
    occurrence.citation_name,
    occurrence.first_name,
    occurrence.last_name,
    occurrence.orcid,
    occurrence.created_at,
    person.orcid AS person_orcid
FROM author_occurrences occurrence
LEFT JOIN people person ON person.id=occurrence.person_id
WHERE occurrence.id = sqlc.arg(id)
  AND EXISTS (
      SELECT 1 FROM authorships membership
      JOIN work_revisions revision ON revision.id=membership.work_revision_id
      WHERE membership.author_occurrence_id=occurrence.id
        AND revision.pipeline_run_id = sqlc.arg(pipeline_run_id)
  );
