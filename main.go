package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/jash3852/gator/internal/config"
	"github.com/jash3852/gator/internal/database"
)

type state struct {
    db  *database.Queries
    cfg *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func(*state, command) error
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.handlers[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	handler, ok := c.handlers[cmd.name]
	if !ok {
		return fmt.Errorf("unknown command: %s", cmd.name)
	}

	return handler(s, cmd)
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("username is required")
	}

	username := cmd.args[0]

	_, err := s.db.GetUser(context.Background(), username)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("user %q does not exist", username)
		}

		return err
	}

	if err := s.cfg.SetUser(username); err != nil {
		return err
	}

	fmt.Printf("User has been set to %s\n", username)

	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) == 0 {
		return fmt.Errorf("username is required")
	}

	username := cmd.args[0]
	now := time.Now()

	user, err := s.db.CreateUser(
		context.Background(),
		database.CreateUserParams{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
			Name:      username,
		},
	)
	if err != nil {
		return err
	}

	if err := s.cfg.SetUser(username); err != nil {
		return err
	}

	fmt.Printf("User %s was created\n", username)
	fmt.Printf("%+v\n", user)

	return nil
}

func handlerReset(s *state, cmd command) error {
	err := s.db.ResetUsers(context.Background())
	if err != nil {
		return err
	}

	fmt.Println("Database reset successfully")
	return nil
}

func handlerUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return err
	}

	for _, user := range users {
		if s.cfg.CurrentUserName == user.Name {
			fmt.Printf("%+v (current)\n", user.Name)
			continue
		} else {
			fmt.Printf("%+v\n", user.Name)
		}
	}

	return nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("usage: agg <time_between_reqs>")
	}

	timeBetweenRequests, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	fmt.Printf(
		"Collecting feeds every %s\n",
		timeBetweenRequests,
	)

	ticker := time.NewTicker(timeBetweenRequests)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		if err := scrapeFeeds(s); err != nil {
			fmt.Printf("Error scraping feed: %v\n", err)
		}
	}
}

func scrapeFeeds(s *state) error {
	ctx := context.Background()

	feed, err := s.db.GetNextFeedToFetch(ctx)
	if err != nil {
		return err
	}

	if err := s.db.MarkFeedFetched(ctx, feed.ID); err != nil {
		return err
	}

	fmt.Printf("Fetching feed: %s\n", feed.Name)

	rssFeed, err := fetchFeed(ctx, feed.Url)
	if err != nil {
		return err
	}

	for _, item := range rssFeed.Channel.Item {
		publishedAt, err := parsePublishedAt(item.PubDate)
		if err != nil {
			fmt.Printf(
				"could not parse date for %q: %v\n",
				item.Title,
				err,
			)

			publishedAt = sql.NullTime{
				Valid: false,
			}
		}

		now := time.Now()

		_, err = s.db.CreatePost(
			ctx,
			database.CreatePostParams{
				ID:          uuid.New(),
				CreatedAt:   now,
				UpdatedAt:   now,
				Title:       item.Title,
				Url:         item.Link,
				Description: toNullString(item.Description),
				PublishedAt: publishedAt,
				FeedID:      feed.ID,
			},
		)

		if err != nil {
			// Handle duplicate URL separately
			var pqErr *pq.Error

			if errors.As(err, &pqErr) && pqErr.Code == "23505" {
				continue
			}

			fmt.Printf(
				"error creating post %q: %v\n",
				item.Title,
				err,
			)
		}
	}

	return nil
}

func handlerAddFeed(
	s *state,
	cmd command,
	user database.User,
) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("usage: addfeed <name> <url>")
	}

	name := cmd.args[0]
	url := cmd.args[1]

	now := time.Now()

	feed, err := s.db.CreateFeed(
		context.Background(),
		database.CreateFeedParams{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
			Name:      name,
			Url:       url,
			UserID:    user.ID,
		},
	)
	if err != nil {
		return err
	}

	_, err = s.db.CreateFeedFollow(
		context.Background(),
		database.CreateFeedFollowParams{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
			UserID:    user.ID,
			FeedID:    feed.ID,
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf("%+v\n", feed)

	return nil
}

func handlerFeeds(s *state, cmd command) error {
	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return err
	}

	for _, feed := range feeds {
		fmt.Printf("Name: %+v\n", feed.Name)
		fmt.Printf("URL: %+v\n", feed.Url)
		user, err := s.db.GetUserByID(context.Background(), feed.UserID)
		if err != nil {
			return err
		}
		fmt.Printf("User: %+v\n", user.Name)
		fmt.Println()
	}

	return nil
}

func handlerFollow(
	s *state,
	cmd command,
	user database.User,
) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("usage: follow <url>")
	}

	feedURL := cmd.args[0]

	feed, err := s.db.GetFeedByURL(
		context.Background(),
		feedURL,
	)
	if err != nil {
		return err
	}

	now := time.Now()

	follow, err := s.db.CreateFeedFollow(
		context.Background(),
		database.CreateFeedFollowParams{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
			UserID:    user.ID,
			FeedID:    feed.ID,
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf(
		"%s is now following %s\n",
		follow.UserName,
		follow.FeedName,
	)

	return nil
}

func handlerFollowing(
	s *state,
	cmd command,
	user database.User,
) error {
	follows, err := s.db.GetFeedFollowsForUser(
		context.Background(),
		user.ID,
	)
	if err != nil {
		return err
	}

	for _, follow := range follows {
		fmt.Println(follow.FeedName)
	}

	return nil
}

func handlerUnfollow(
	s *state,
	cmd command,
	user database.User,
) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("usage: unfollow <url>")
	}

	feedURL := cmd.args[0]

	feed, err := s.db.GetFeedByURL(
		context.Background(),
		feedURL,
	)
	if err != nil {
		return err
	}

	err = s.db.DeleteFeedFollow(
		context.Background(),
		database.DeleteFeedFollowParams{
			UserID: user.ID,
			FeedID: feed.ID,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func handlerBrowse(
	s *state,
	cmd command,
	user database.User,
) error {
	limit := int32(2)

	if len(cmd.args) > 0 {
		parsedLimit, err := strconv.Atoi(cmd.args[0])
		if err != nil {
			return fmt.Errorf(
				"invalid limit: %w",
				err,
			)
		}

		if parsedLimit <= 0 {
			return fmt.Errorf(
				"limit must be greater than 0",
			)
		}

		limit = int32(parsedLimit)
	}

	posts, err := s.db.GetPostsForUser(
		context.Background(),
		database.GetPostsForUserParams{
			UserID: user.ID,
			Limit:  limit,
		},
	)
	if err != nil {
		return err
	}

	for _, post := range posts {
		fmt.Printf(
			"%s\n%s\n\n",
			post.Title,
			post.Url,
		)
	}

	return nil
}

func parsePublishedAt(dateString string) (sql.NullTime, error) {
	if dateString == "" {
		return sql.NullTime{
			Valid: false,
		}, nil
	}

	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, dateString)
		if err == nil {
			return sql.NullTime{
				Time:  t,
				Valid: true,
			}, nil
		}
	}

	return sql.NullTime{}, fmt.Errorf(
		"could not parse published date %q",
		dateString,
	)
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{
			Valid: false,
		}
	}

	return sql.NullString{
		String: s,
		Valid:  true,
	}
}

func middlewareLoggedIn(
	handler func(s *state, cmd command, user database.User) error,
) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(
			context.Background(),
			s.cfg.CurrentUserName,
		)
		if err != nil {
			return err
		}

		return handler(s, cmd, user)
	}
}

func main() {
	cfg, err := config.Read()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	db, err := sql.Open("postgres", cfg.DbURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer db.Close()

	dbQueries := database.New(db)

	s := &state{
		db:  dbQueries,
		cfg: &cfg,
	}

	cmds := commands{
		handlers: make(map[string]func(*state, command) error),
	}

	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("feeds", handlerFeeds)

	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("browse", middlewareLoggedIn(handlerBrowse))

	args := os.Args

	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "not enough arguments")
		os.Exit(1)
	}

	cmd := command{
		name: args[1],
		args: args[2:],
	}

	err = cmds.run(s, cmd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}