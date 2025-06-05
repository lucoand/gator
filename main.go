package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
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
	}
	return &feed, nil
}

func (c *commands) run(s *state, cmd command) error {
	err := c.funcs[cmd.name](s, cmd)
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
	fmt.Printf("User %v was created.\n", args.Name)
	fmt.Printf("%v %v %v %v\n", user.ID, user.CreatedAt, user.UpdatedAt, user.Name)
	err = config.SetUser(args.Name, *s.cfg)
	return err
}

func handlerReset(s *state, _ command) error {
	err := s.db.DeleteAllUsers(context.Background())
	if err != nil {
		fmt.Println("ERROR: Could not reset users table.")
		return err
	}
	fmt.Println("Users table reset successfully.")
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

func handleAgg(s *state, _ command) error {
	feed, err := fetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		fmt.Println("Could not fetch feed.")
		return err
	}
	fmt.Printf("%v\n", feed)
	return nil
}

func handleAddfeed(s *state, cmd command) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("ERROR: addfeed requires two arguments.\nUsage: gator addfeed <feedName> <url>")
	}
	user, err := s.db.GetUser(context.Background(), s.cfg.Username)
	if err != nil {
		fmt.Println("ERROR: Could not retrieve user information from database.")
		return err
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
	fmt.Printf("%v %v %v %v %v %v\n", feed.ID, feed.CreatedAt, feed.UpdatedAt, feed.Name, feed.Url, feed.UserID)
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
	cmds.register("addfeed", handleAddfeed)
	cmds.register("feeds", handleFeeds)
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
