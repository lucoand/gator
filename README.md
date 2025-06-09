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

### Option 1:
Install from github

```console
go install github.com/lucoand/gator@latest
```

### Option 2:
Download and build

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

"postgres://username:password@host:port/database"

Default port for postgres is 5432.

For example:

#### macOS
"postgres://username:@localhost:5432/gator"

#### Linux
"postgres://postgres:yourpasshere@localhost:5432/gator"
