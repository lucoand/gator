-- name: CreateFeedFollow :one
WITH inserted_feed_follow AS (
	INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id)
		VALUES (
		$1,
		$2,
		$3,
		$4,
		$5
		)
		RETURNING *
)
SELECT
	inserted_feed_follow.*,
	feeds.name as feed_name,
	users.name as user_name
FROM inserted_feed_follow
INNER JOIN feeds
ON feeds.id = inserted_feed_follow.feed_id
INNER JOIN users
ON users.id = inserted_feed_follow.user_id;

-- name: GetFeedFollowsForUser :many
SELECT
	feeds.name AS feed_name,
	users.name as user_name
FROM feed_follows
INNER JOIN feeds
ON feeds.id = feed_follows.feed_id
INNER JOIN users
ON users.id = feed_follows.user_id
WHERE users.name = $1;

-- name: DeleteFeedFollowByUserNameAndFeedUrl :one
-- @param user_name: string
-- @param feed_url: string
WITH deleted AS (
DELETE FROM feed_follows
USING users, feeds
WHERE users.id = feed_follows.user_id
AND feeds.id = feed_follows.feed_id
AND users.name = sqlc.arg('user_name')
AND feeds.url = sqlc.arg('feed_url')
RETURNING 1
)
SELECT COUNT(*) FROM deleted;
