-- name: ListCompanies :many
SELECT *
FROM companies
WHERE (sqlc.narg('active')::boolean    IS NULL OR active     = sqlc.narg('active')::boolean)
  AND (sqlc.narg('board_kind')::text   IS NULL OR board_kind = sqlc.narg('board_kind')::text)
ORDER BY priority ASC, name ASC;

-- name: GetCompany :one
SELECT *
FROM companies
WHERE id = $1;

-- name: CreateCompany :one
INSERT INTO companies (
    name, board_kind, board_slug, priority, tags, notes, active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetActiveCompanies :many
SELECT *
FROM companies
WHERE active = TRUE
  AND (sqlc.narg('board_kind')::text IS NULL OR board_kind = sqlc.narg('board_kind')::text)
ORDER BY priority ASC, name ASC;
