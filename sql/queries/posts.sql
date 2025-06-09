-- name: CreatePost :one
INSERT INTO posts (
	id,
	created_at,
	updated_at,
	title,
	url,
	description,
	published_at,
	feed_id
)
VALUES (
	$1,
	NOW(),
	NOW(),
	$2,
	$3,
	$4,
	$5,
	$6
	)
	RETURNING *;

-- name: GetPostsForUser :many
SELECT
	posts.id AS id,
	posts.created_at AS created_at,
	posts.updated_at AS updated_at,
	posts.title AS title,
	posts.url AS url,
	posts.description AS description,
	posts.published_at AS published_at,
	posts.feed_id AS feed_id
FROM posts
INNER JOIN feed_follows
ON posts.feed_id = feed_follows.feed_id
WHERE feed_follows.user_id = $1
ORDER BY posts.published_at DESC;
