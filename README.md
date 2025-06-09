# Gator

Blog aggregator project for boot.dev

Made with Go 1.24.3

## Requirements
- Go
- PostgreSQL

## Description

A CLI application that scrapes RSS Feeds for new posts.  Can support multiple users per database.

You must have Postgres and Go to run the program.  All database queries were set up for Postgres.

## Installation

Option 1: Install from github

```console
go install github.com/lucoand/gator@latest
```

Option 2: Download and build

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

