-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES (
	$1,
	$2,
	$3,
	$4,
	$5,
	$6
	)
	RETURNING *;

-- name: GetFeeds :many
SELECT feeds.name AS name, feeds.url AS url, users.name AS user_name
FROM feeds 
INNER JOIN users
ON feeds.user_id = users.id;

-- name: GetFeedIDByUrl :one
SELECT id
FROM feeds
WHERE feeds.url = $1;

-- name: MarkFeedFetched :exec
UPDATE feeds
SET updated_at = NOW(), last_fetched_at = NOW()
WHERE id = $1;

-- name: GetNextFeedToFetch :one
SELECT id, url FROM feeds
ORDER BY last_fetched_at ASC NULLS FIRST
LIMIT 1;
