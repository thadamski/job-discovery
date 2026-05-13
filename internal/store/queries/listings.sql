-- name: InsertListingIfNew :one
-- Returns the inserted row, or no rows when (company_id, external_id)
-- already exists. Callers MUST treat ErrNoRows as "duplicate, not a first
-- insert" — this is the signal used to gate jobhunt.listing.discovered
-- publishing.
INSERT INTO listings (
    company_id, external_id, title, location, url, description, raw_payload, posted_at
) VALUES (
    @company_id, @external_id, @title, @location, @url, @description, @raw_payload, @posted_at
)
ON CONFLICT (company_id, external_id) DO NOTHING
RETURNING *;

-- name: GetListing :one
SELECT *
FROM listings
WHERE id = @id;

-- name: ListListings :many
-- min_score is intentionally NOT in this query: scores live in the
-- job-scoring service. The handler calls job-scoring over HTTP and
-- intersects the result client-side when min_score is set.
SELECT l.*
FROM listings AS l
JOIN companies AS c ON c.id = l.company_id
WHERE (sqlc.narg('status')::text     IS NULL OR l.status     = sqlc.narg('status')::text)
  AND (sqlc.narg('company_id')::uuid IS NULL OR l.company_id = sqlc.narg('company_id')::uuid)
  AND (sqlc.narg('board_slug')::text IS NULL OR c.board_slug = sqlc.narg('board_slug')::text)
ORDER BY l.fetched_at DESC
LIMIT  @lim
OFFSET @off;

-- name: UpdateListingStatus :one
UPDATE listings
SET status = @status
WHERE id = @id
RETURNING *;
