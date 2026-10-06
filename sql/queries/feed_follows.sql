-- name: CreateFeedFollow :one
WITH inserted_follow AS (
    INSERT INTO feed_follows (
        id,
        created_at,
        updated_at,
        user_id,
        feed_id
    )
    VALUES ($1, $2, $3, $4, $5)
    RETURNING *
)
SELECT
    inserted_follow.id,
    inserted_follow.created_at,
    inserted_follow.updated_at,
    inserted_follow.user_id,
    inserted_follow.feed_id,
    users.name AS user_name,
    feeds.name AS feed_name
FROM inserted_follow
JOIN users ON users.id = inserted_follow.user_id
JOIN feeds ON feeds.id = inserted_follow.feed_id;

-- name: GetFeedFollowsForUser :many
SELECT
    feed_follows.id,
    feed_follows.created_at,
    feed_follows.updated_at,
    feed_follows.user_id,
    feed_follows.feed_id,
    users.name AS user_name,
    feeds.name AS feed_name
FROM feed_follows
JOIN users ON users.id = feed_follows.user_id
JOIN feeds ON feeds.id = feed_follows.feed_id
WHERE feed_follows.user_id = $1
ORDER BY feeds.name;

-- name: DeleteFeedFollow :exec
DELETE FROM feed_follows
WHERE user_id = $1 AND feed_id = $2;