# Gator

Blog aggregator project for boot.dev

Made with Go 1.24.3

## Requirements
- Go
- PostgreSQL
- Goose (for setting up the database)

## Description

A CLI application that scrapes RSS Feeds for new posts.  Can support multiple users per database.

This has only been tested on Linux.  It MIGHT work on MacOS as well, but don't count on it.

You must have Postgres and Go to run the program.  All database queries were set up for Postgres.

## Installation

### Download and build

```console
git clone https://github.com/lucoand/gator.git
cd gator
```

From here, you can do either build or install:

```console
go build
```

```console
go install
```
Installing will put the `gator` executable in your path, which will make it easier to run.

## Configuration

### Step 1:
Create Database

Use whatever Postgres client you wish to create the database for gator.  I used psql on Linux.

```console
sudo -u postgres psql
```

Once you are logged in to the server, you should see:

```console
postgres=#
```

From here, create and connect to the database with the following commands:

```console
CREATE DATABASE gator;
\c gator
```

You should now see this prompt:

```console
gator=#
```

From here, set a password for the database if desired (required on Linux):

```console
ALTER USER postgres PASSWORD 'yourpasshere';
```

We will need this password later, if created.

### Step 2:
Database Setup 

I used Goose to handle setting up the database migrations.  To install Goose:

```console
go install github.com/pressly/goose/v3/cmd/goose@latest
```

If you have a different application that handles this, you will almost certainly need to alter the .sql files in sql/schema to fit your application. From here I am assuming you are using Goose.

Next, set up your connection string.  We will need this string both for database setup and for the config file later.

The string should be of this form:

```console
"postgres://username:password@host:port/database"
```

Default port for postgres is 5432.

For example:

#### macOS
```console
"postgres://username:@localhost:5432/gator"
```
#### Linux
```console
"postgres://postgres:yourpasshere@localhost:5432/gator"
```

You can verify your string by running psql, for example:

```console
psql "postgres://postgres:yourpasshere@localhost:5432/gator"
```

It should connect you to the `gator` database directly.

Armed with this working string, we can finally set up the database with goose.

From the root directory of the repo:
```console
cd sql/schema
goose postgres <connectionstring> up
```
Example:
```console
goose postgres "postgres://postgres:password@localhost:5432/gator" up
```

This will use the schema files to update the database with the tables `gator` requires.

### Step 3:
Create Config File

This is the final step!

Create a file called .gatorconfig.json in your HOME directory and populate it with this data:
```json
{
    "db_url":"<connection_string>?sslmode=disable",
    "current_user_name":""
}
```
Example:

```json
{
    "db_url":"postgres://postgres:password@localhost:5432/gator?sslmode=disable",
    "current_user_name":""
}
```
Make sure the quotation marks are there, even the empty quotes for `"current_user_name"`  If you like you can put a username into that field.

Example:
```json
"current_user_name":"lucoa"
```
However this it not necessary.  `gator` will modify this file based on your input when you register or login to different users in in the database.

That's it!  You're now ready to use `gator`!

## Usage

`gator` has a variety of commands:

```console
gator help
```
Displays a help message.

```console
gator reset
```
After confirmation, this will DELETE all the data from the database and start over from scratch!  THIS CANNOT BE UNDONE so use with caution!

```console
gator register <username>
```
Example:
```console
gator register lucoa
```
Registers `<username>` as a user in the database, and logs the user in.

```console
gator login <username>
```
Logs in an existing user, allowing you to switch between multiple users.  Usage is the same as register.

```console
gator users
```
Lists all users registered in the database.  Marks the current logged in user as (current) in the output.
```console
gator addfeed <feedname> <url>
```
Example:
```console
gator addfeed "NY Times World News" "https://rss.nytimes.com/services/xml/rss/nyt/World.xml"
```
Adds an RSS feed to the database and automatically follows it for the logged-in user.  If the feed is already in the database, you can instead use the next command to follow it.

```console
gator follow <url>
```
Example:
```console
gator register lucoa
gator addfeed "NY Times World News" "https://rss.nytimes.com/services/xml/rss/nyt/World.xml"
gator register tohru
gator follow "https://rss.nytimes.com/services/xml/rss/nyt/World.xml"
```
Follows a feed that is already in the database.
```console
gator feeds
```
Lists all the feeds by name and url that are tracked in the database.
```console
gator unfollow <url>
```
Example:
```console
gator unfollow "https://rss.nytimes.com/services/xml/rss/nyt/World.xml"
```
Unfollows the feed at `<url>` for the current logged in user.
```console
gator following
```
Lists all the feeds that are followed by the current logged in user.
```console
gator agg <interval>
```
Example:
```console
gator agg 5m
```
This is the main command of `gator`.  Fetches post data for feeds in the database.  Newly added feeds that haven't been fetched yet are prioritized, then the feed with the oldest `fetched_at` value.  This will cycle through all the feeds, one feed per `<interval>`.  Minimum interval is 1m (one minute).
This is meant to be running in the background while you do other things.  Ctrl+C to quit out.


```console
gator browse [limit]
```
Limit argument is optional.  Defaults to 2.

Example:
```console
gator browse 5
```
Lists a number of posts from the currently logged in user's feeds, most recently published posts first.
