package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lucoand/gator/internal/config"
	"github.com/lucoand/gator/internal/database"
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
	funcs map[string]func(*state, command) error
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		fmt.Println("ERROR: Unable to generate http request.")
		return &RSSFeed{}, err
	}
	req.Header.Set("User-Agent", "gator")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("ERROR: Bad http response.")
		return &RSSFeed{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("ERROR: Could not read http response body.")
		return &RSSFeed{}, err
	}
	var feed RSSFeed
	err = xml.Unmarshal(body, &feed)
	if err != nil {
		fmt.Println("ERROR: Could not unmarshal xml.")
		return &RSSFeed{}, err
	}
	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)
	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
		if feed.Channel.Item[i].Link == "" && feed.Channel.Item[i].GUID != "" {
			feed.Channel.Item[i].Link = feed.Channel.Item[i].GUID
		}
		// fmt.Println("TITLE:", feed.Channel.Item[i].Title)
		// fmt.Println("LINK:", feed.Channel.Item[i].Link)
	}
	return &feed, nil
}

func (c *commands) run(s *state, cmd command) error {
	f, ok := c.funcs[cmd.name]
	if !ok {
		return fmt.Errorf("ERROR: Unknown command %v", cmd.name)
	}
	err := f(s, cmd)
	return err
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.funcs[name] = f
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return errors.New("ERROR: expected one argument after \"login\"\nUsage: gator login <username>")
	}
	_, err := s.db.GetUser(context.Background(), cmd.args[0])
	if err != nil {
		fmt.Printf("ERROR: User %v not registered in database!  Try \"gator register <username>\" first\n", cmd.args[0])
		return err
	}
	err = config.SetUser(cmd.args[0], *s.cfg)
	return err
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return errors.New("ERROR: expected one argument after \"register\"\nUsage: gator register <username>")
	}
	var args database.CreateUserParams
	args.ID = uuid.New()
	args.Name = cmd.args[0]
	currentTime := time.Now()
	args.CreatedAt = currentTime
	args.UpdatedAt = currentTime
	user, err := s.db.CreateUser(context.Background(), args)
	if err != nil {
		fmt.Println("ERROR: Name already exists.")
		return err
	}
	fmt.Printf("User %v was created.\n", user.Name)
	// fmt.Printf("%v %v %v %v\n", user.ID, user.CreatedAt, user.UpdatedAt, user.Name)
	err = config.SetUser(user.Name, *s.cfg)
	return err
}

func handlerReset(s *state, _ command) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("This will DELETE ALL data from the database, including user and feed data.  Are you sure? (yes/[no]): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "yes" {
		err := s.db.DeleteAllUsers(context.Background())
		if err != nil {
			fmt.Println("ERROR: Could not reset users table.")
			return err
		}
		fmt.Println("Users table reset successfully.")
	} else {
		fmt.Println("Aborted.")
	}
	return nil
}

func handlerUsers(s *state, _ command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		fmt.Println("ERROR: Could not retrieve list of users.")
		return err
	}
	for _, user := range users {
		output := "* " + user
		if user == s.cfg.Username {
			output += " (current)"
		}
		fmt.Println(output)
	}
	return nil
}

func handleAgg(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("ERROR: agg requires an argument.\nUsage: \"gator agg <interval>\".  Interval can be of a form like 1m, 1h, etc.  Must be at least 1m.")
	}
	const minInterval = 1 * time.Minute
	timeBetweenRequests, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		fmt.Printf("ERROR: Unable to parse duration \"%v\"\n", cmd.args[0])
		return err
	}
	if timeBetweenRequests < minInterval {
		fmt.Printf("Interval too short!  Must be at least %v\n", minInterval)
		return nil
	}
	fmt.Printf("Collecting feeds every %v\n", timeBetweenRequests)
	ticker := time.NewTicker(timeBetweenRequests)
	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}
}

func handleAddfeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("ERROR: addfeed requires two arguments.\nUsage: gator addfeed <feedName> <url>")
	}
	currentTime := time.Now()
	arg := database.CreateFeedParams{}
	arg.ID = uuid.New()
	arg.CreatedAt = currentTime
	arg.UpdatedAt = currentTime
	arg.Name = cmd.args[0]
	arg.Url = cmd.args[1]
	arg.UserID = user.ID
	feed, err := s.db.CreateFeed(context.Background(), arg)
	if err != nil {
		fmt.Println("ERROR: Could not create feed.")
		return err
	}
	_, err = addFollow(s, user.ID, feed.ID)
	if err != nil {
		return err
	}
	// fmt.Printf("%v %v %v %v %v %v\n", feed.ID, feed.CreatedAt, feed.UpdatedAt, feed.Name, feed.Url, feed.UserID)
	fmt.Printf("Added feed to database:\n")
	fmt.Printf("%v\n", feed.Name)
	return nil
}

func handleFeeds(s *state, _ command) error {
	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		fmt.Println("ERROR: Could not retrieve feeds from database.")
		return err
	}
	for _, feed := range feeds {
		fmt.Printf("%v %v %v\n", feed.Name, feed.Url, feed.UserName)
	}
	return nil
}

func addFollow(s *state, userID uuid.UUID, feedID uuid.UUID) (database.CreateFeedFollowRow, error) {
	var arg database.CreateFeedFollowParams
	arg.UserID = userID
	arg.FeedID = feedID
	currentTime := time.Now()
	arg.CreatedAt = currentTime
	arg.UpdatedAt = currentTime
	arg.ID = uuid.New()
	feed_follow, err := s.db.CreateFeedFollow(context.Background(), arg)
	if err != nil {
		fmt.Println("Could not add feed follow to database.")
		return database.CreateFeedFollowRow{}, err
	}
	return feed_follow, nil
}

func handleFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("ERROR: follow command requires one argument.\n Usage: gator follow <url>")
	}
	feedID, err := s.db.GetFeedIDByUrl(context.Background(), cmd.args[0])
	if err != nil {
		fmt.Println("Unable to retrieve feed from database.  This likely means the feed has not been added.\nTry adding with \"gator addfeed <feedname> <url>\"")
		return err
	}
	feed_follow, err := addFollow(s, user.ID, feedID)
	if err != nil {
		return err
	}
	fmt.Printf("%v %v\n", feed_follow.FeedName, feed_follow.UserName)
	return nil
}

func handleFollowing(s *state, _ command, user database.User) error {
	feeds, err := s.db.GetFeedFollowsForUser(context.Background(), user.Name)
	if err != nil {
		fmt.Printf("ERROR: Could not retrieve follows for user %v\n", user.Name)
		return err
	}
	if len(feeds) < 1 {
		fmt.Printf("No feeds followed by user %v\n", user.Name)
		return nil
	}
	fmt.Printf("Feeds followed by user %v:\n", feeds[0].UserName)
	for _, feed := range feeds {
		fmt.Println(feed.FeedName)
	}
	return nil
}

func handleUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("ERROR: unfollow requires a feed url\nUsage: gator unfollow <url>")
	}
	var arg database.DeleteFeedFollowByUserNameAndFeedUrlParams
	arg.UserName = user.Name
	arg.FeedUrl = cmd.args[0]
	count, err := s.db.DeleteFeedFollowByUserNameAndFeedUrl(context.Background(), arg)
	if err != nil {
		fmt.Println("ERROR: Unable to delete follow.")
		return err
	}
	if count < 1 {
		fmt.Printf("User %v was already not following %v\n", arg.UserName, arg.FeedUrl)
	} else {
		fmt.Printf("%v unfollowed for user %v\n", arg.FeedUrl, arg.UserName)
	}
	return nil
}

func handleBrowse(s *state, cmd command, user database.User) error {
	limit := 2
	if len(cmd.args) > 0 {
		arg, err := strconv.Atoi(cmd.args[0])
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			fmt.Println("Could not parse optional limit argument.\nUsage: gator browse [limit].  limit must be a decimal value.")
			fmt.Println("Defaulting to limit= 2")
		} else {
			limit = arg
		}
	}

	posts, err := s.db.GetPostsForUser(context.Background(), user.ID)
	if err != nil {
		fmt.Printf("ERROR: Could not get posts for user %v\n", user.Name)
		return err
	}
	num_posts := len(posts)
	if num_posts < limit {
		fmt.Printf("Limit was %v but only found %v posts.\n", limit, num_posts)
		limit = num_posts
	}

	fmt.Printf("Most recent %v posts for user %v\n\n", limit, user.Name)
	for i := range limit {
		// fmt.Printf("%v %v %v %v\n\n", posts[i].Title, posts[i].Url, posts[i].Description, posts[i].PublishedAt)
		fmt.Println("TITLE:", posts[i].Title)
		fmt.Println("URL:", posts[i].Url)
		fmt.Println("DESCRIPTION:", posts[i].Description)
		fmt.Println("PUBLISHED AT:", posts[i].PublishedAt)
		fmt.Println("")
	}
	return nil
}

func scrapeFeeds(s *state) error {
	feedRow, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		fmt.Println("ERROR: Could not retrieve feed data from database.")
		return err
	}
	err = s.db.MarkFeedFetched(context.Background(), feedRow.ID)
	if err != nil {
		fmt.Println("ERROR: Unable to mark feed as fetched.")
		return err
	}
	feed, err := fetchFeed(context.Background(), feedRow.Url)
	if err != nil {
		fmt.Printf("ERROR: Could not fetch feed from %v\n", feedRow.Url)
		return err
	}
	fmt.Printf("Checking feed %v for new posts.\n\n", feed.Channel.Title)
	count := 0
	// fmt.Printf("Found %v posts.\n", len(feed.Channel.Item))
	for _, item := range feed.Channel.Item {
		published_at, err := dateparse.ParseAny(item.PubDate)
		if err != nil {
			fmt.Printf("ERROR: Could not parse PubDate for %v.\n", item.Link)
		}
		// fmt.Printf("%v\n", published_at)
		var arg database.CreatePostParams
		arg.ID = uuid.New()
		arg.Title = item.Title
		arg.Url = item.Link
		arg.Description = item.Description
		arg.PublishedAt = published_at
		arg.FeedID = feedRow.ID
		post, err := s.db.CreatePost(context.Background(), arg)
		if err != nil {
			// fmt.Println(item.Link)
			err = checkPostError(err)
			if err != nil {
				return err
			}

		} else {
			fmt.Printf("%v %v %v %v\n\n", post.Title, post.Url, post.Description, post.PublishedAt)
			fmt.Println("TITLE:", post.Title)
			fmt.Println("URL:", post.Url)
			fmt.Println("DESCRIPTION:", post.Description)
			fmt.Println("PUBLISHED AT:", post.PublishedAt)
			fmt.Println("")
			count += 1
		}
	}
	if count == 0 {
		fmt.Printf("No new posts found.\n\n")
	} else {
		fmt.Printf("Found %v new posts.\n\n", count)
	}
	return nil
}

func checkPostError(err error) error {
	if pqErr, ok := err.(*pq.Error); ok {
		if pqErr.Constraint == "posts_url_key" {
			return nil
		}
	}
	return err
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.cfg.Username)
		if err != nil {
			return err
		}
		return handler(s, cmd, user)
	}
}

func main() {
	cfg := config.Read()
	var s state
	s.cfg = &cfg
	db, err := sql.Open("postgres", cfg.DB_url)
	if err != nil {
		fmt.Println("ERROR: Unable to connect to database:", err)
		os.Exit(1)
	}
	dbQueries := database.New(db)
	s.db = dbQueries
	var cmds commands
	cmds.funcs = make(map[string]func(*state, command) error)
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handleAgg)
	cmds.register("addfeed", middlewareLoggedIn(handleAddfeed))
	cmds.register("feeds", handleFeeds)
	cmds.register("follow", middlewareLoggedIn(handleFollow))
	cmds.register("following", middlewareLoggedIn(handleFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handleUnfollow))
	cmds.register("browse", middlewareLoggedIn(handleBrowse))
	argv := os.Args
	if len(argv) < 2 {
		fmt.Println("Not enough arguemnts provided.")
		os.Exit(1)
	}
	name := argv[1]
	if len(argv) == 2 {
		argv = []string{}
	} else {
		argv = argv[2:]
	}
	var cmd command
	cmd.name = name
	cmd.args = argv
	err = cmds.run(&s, cmd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
