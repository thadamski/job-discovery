-- name: ListCompanies :many
SELECT *
FROM companies
WHERE (sqlc.narg('active')::boolean  IS NULL OR active     = sqlc.narg('active')::boolean)
  AND (sqlc.narg('board_kind')::text IS NULL OR board_kind = sqlc.narg('board_kind')::text)
ORDER BY priority ASC, name ASC;

-- name: GetCompany :one
SELECT *
FROM companies
WHERE id = @id;

-- name: CreateCompany :one
INSERT INTO companies (
    name, board_kind, board_slug, priority, tags, notes, active
) VALUES (
    @name, @board_kind, @board_slug, @priority, @tags, @notes, @active
)
RETURNING *;

-- name: GetActiveCompanies :many
SELECT *
FROM companies
WHERE active = TRUE
  AND (sqlc.narg('board_kind')::text IS NULL OR board_kind = sqlc.narg('board_kind')::text)
ORDER BY priority ASC, name ASC;
