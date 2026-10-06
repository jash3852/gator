# Gator

Gator is a command-line RSS feed aggregator written in Go and backed by PostgreSQL.

It allows users to register accounts, add and follow RSS feeds, continuously fetch posts from those feeds, and browse the latest posts directly from the terminal.

## Requirements

To run Gator, you need:

- [Go](https://go.dev/) installed
- [PostgreSQL](https://www.postgresql.org/) installed and running

You will also need a PostgreSQL database for Gator.

For example:

```bash
createdb gator
```

## Installation

Install Gator using `go install`:

```bash
go install github.com/jash3852/BlogAggregator@latest
```

Make sure your Go binary directory is included in your `PATH`.

You can check your Go binary directory with:

```bash
go env GOPATH
```

Installed Go programs are typically placed in:

```text
$GOPATH/bin
```

After installation, you can run the compiled CLI directly instead of using `go run .`.

> Note: If the repository/module is renamed to `gator`, the recommended installation command would be:
>
> ```bash
> go install github.com/jash3852/gator@latest
> ```

## Configuration

Gator reads its configuration from:

```text
~/.gatorconfig.json
```

Create that file in your home directory.

Example:

```json
{
  "db_url": "postgres://postgres:postgres@localhost:5432/gator",
  "current_user_name": ""
}
```

Change the PostgreSQL connection string if your username, password, host, port, or database name are different.

The application updates `current_user_name` automatically when you register or log in as a user.

## Database Setup

Before using Gator, apply the database migrations so that the required tables exist.

The database contains tables for:

- users
- feeds
- feed follows
- posts

If you are using Goose for migrations, run the migrations against your Gator database before starting the application.

For example:

```bash
goose postgres "postgres://postgres:postgres@localhost:5432/gator" up
```

## Usage

During development, you can run commands with:

```bash
go run . <command>
```

After installing the application, run the compiled CLI instead.

### Register a user

```bash
gator register alice
```

Creates a new user and makes that user the current user.

### Log in

```bash
gator login alice
```

Changes the currently logged-in user.

The user must already exist.

### List users

```bash
gator users
```

Lists all registered users and identifies the currently logged-in user.

### Add a feed

```bash
gator addfeed "Hacker News" "https://news.ycombinator.com/rss"
```

Adds an RSS feed to the database and automatically follows it for the current user.

### List feeds

```bash
gator feeds
```

Lists the feeds stored in the database.

### Follow a feed

```bash
gator follow "https://news.ycombinator.com/rss"
```

Follows an existing feed as the current user.

### Unfollow a feed

```bash
gator unfollow "https://news.ycombinator.com/rss"
```

Stops following a feed.

### List followed feeds

```bash
gator following
```

Lists all feeds followed by the current user.

### Aggregate feeds

```bash
gator agg 1m
```

Starts the long-running RSS aggregator.

The argument specifies how frequently Gator should fetch a feed. Go duration strings are supported, for example:

```text
10s
30s
1m
5m
1h
```

The aggregator runs continuously until it is stopped with `Ctrl+C`.

A reasonable development interval is:

```bash
gator agg 30s
```

Avoid using unnecessarily short intervals because Gator sends requests to third-party RSS servers.

### Browse posts

```bash
gator browse
```

Displays the latest posts from feeds followed by the current user.

By default, Gator displays 2 posts.

You can specify a different limit:

```bash
gator browse 10
```

### Reset the database

```bash
gator reset
```

Deletes all users and associated data.

Because related records use cascading foreign keys, associated feeds, follows, and posts may also be deleted.

This command is intended for development use.

## Example Workflow

Register a user:

```bash
gator register alice
```

Add a feed:

```bash
gator addfeed "Hacker News" "https://news.ycombinator.com/rss"
```

Start the aggregator in one terminal:

```bash
gator agg 1m
```

Then, in another terminal, browse the collected posts:

```bash
gator browse 10
```

## Development

Clone the repository:

```bash
git clone https://github.com/jash3852/BlogAggregator.git
cd BlogAggregator
```

Download dependencies:

```bash
go mod download
```

Build the application:

```bash
go build
```

Or run it during development:

```bash
go run . <command>
```

Go programs are statically compiled. Once the application has been built or installed, the Go toolchain is not required just to execute the resulting binary.