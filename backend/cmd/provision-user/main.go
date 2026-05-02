// provision-user is an administrative CLI that atomically provisions a House
// member: it creates the user in Firebase Auth (when missing), upserts the
// corresponding row in the application database, and links the user to the
// target House via house_members.
//
// The flow is idempotent: re-running with the same email and House converges
// to the same state without duplicating users or memberships.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/jackc/pgx/v5"

	"github.com/erielfranco/jullius-scan/backend/internal/database"
)

type cliOptions struct {
	email         string
	password      string
	passwordStdin bool
	name          string
	houseID       int64
	houseName     string
	role          string
	yes           bool
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		flag.CommandLine.Usage()
		os.Exit(2)
	}

	if err := run(context.Background(), opts, os.Stdin); err != nil {
		slog.Error("provisioning failed", "error", err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (*cliOptions, error) {
	fs := flag.NewFlagSet("provision-user", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: provision-user --email <addr> [--password <pwd> | --password-stdin] [--name "<full name>"] (--house-id <id> | --house-name "<name>") [--role member|owner] [--yes]

Environment:
  DATABASE_URL                    PostgreSQL connection string (required)
  FIREBASE_PROJECT_ID             Firebase project id (required)
  GOOGLE_APPLICATION_CREDENTIALS  path to a Firebase service account JSON with user-management permissions

Examples:
  go run ./cmd/provision-user --email alice@example.com --password 's3cr3t' --house-name "Casa Principal"
  echo 's3cr3t' | go run ./cmd/provision-user --email alice@example.com --password-stdin --house-id 1
`)
	}

	opts := &cliOptions{}
	fs.StringVar(&opts.email, "email", "", "email of the user to provision (required)")
	fs.StringVar(&opts.password, "password", "", "password for the new Firebase user; ignored if the user already exists")
	fs.BoolVar(&opts.passwordStdin, "password-stdin", false, "read the password from standard input instead of a flag")
	fs.StringVar(&opts.name, "name", "", "display name for the user (defaults to the email local-part)")
	fs.Int64Var(&opts.houseID, "house-id", 0, "id of the House to link the user to")
	fs.StringVar(&opts.houseName, "house-name", "", "name of the House to link the user to (must match exactly one row)")
	fs.StringVar(&opts.role, "role", "member", "membership role (member|owner)")
	fs.BoolVar(&opts.yes, "yes", false, "skip the interactive confirmation prompt")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	opts.email = strings.TrimSpace(strings.ToLower(opts.email))
	if opts.email == "" {
		return nil, errors.New("--email is required")
	}
	if opts.password != "" && opts.passwordStdin {
		return nil, errors.New("--password and --password-stdin are mutually exclusive")
	}
	if opts.houseID == 0 && opts.houseName == "" {
		return nil, errors.New("either --house-id or --house-name must be provided")
	}
	if opts.houseID != 0 && opts.houseName != "" {
		return nil, errors.New("--house-id and --house-name are mutually exclusive")
	}
	if opts.role != "member" && opts.role != "owner" {
		return nil, fmt.Errorf("--role must be 'member' or 'owner', got %q", opts.role)
	}
	if opts.name == "" {
		if at := strings.Index(opts.email, "@"); at > 0 {
			opts.name = opts.email[:at]
		} else {
			opts.name = opts.email
		}
	}
	return opts, nil
}

func run(ctx context.Context, opts *cliOptions, stdin io.Reader) error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL environment variable is required")
	}
	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	if projectID == "" {
		return errors.New("FIREBASE_PROJECT_ID environment variable is required")
	}

	if opts.passwordStdin {
		pwd, err := readPasswordFromReader(stdin)
		if err != nil {
			return fmt.Errorf("read password from stdin: %w", err)
		}
		opts.password = pwd
	}

	dbCtx, dbCancel := context.WithTimeout(ctx, 10*time.Second)
	defer dbCancel()
	db, err := database.Connect(dbCtx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer db.Close()

	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		return fmt.Errorf("init firebase app: %w", err)
	}
	authClient, err := app.Auth(ctx)
	if err != nil {
		return fmt.Errorf("init firebase auth client: %w", err)
	}

	houseID, houseName, err := resolveHouse(ctx, db, opts)
	if err != nil {
		return err
	}
	slog.Info("target house resolved", "house_id", houseID, "house_name", houseName)

	if !opts.yes {
		fmt.Fprintf(os.Stderr, "About to provision %q into house %q (id=%d, role=%s). Continue? [y/N] ", opts.email, houseName, houseID, opts.role)
		reader := bufio.NewReader(os.Stdin)
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(strings.ToLower(ans))
		if ans != "y" && ans != "yes" {
			return errors.New("aborted by operator")
		}
	}

	firebaseUID, preexisting, err := ensureFirebaseUser(ctx, authClient, opts)
	if err != nil {
		return fmt.Errorf("ensure firebase user: %w", err)
	}

	userID, err := upsertMembership(ctx, db, firebaseUID, opts.email, opts.name, houseID, opts.role)
	if err != nil {
		return fmt.Errorf("persist user/membership for firebase_uid=%s — re-run the same command to retry (idempotent): %w", firebaseUID, err)
	}

	fmt.Fprintln(os.Stdout, "provision-user: success")
	fmt.Fprintf(os.Stdout, "  users.id         = %d\n", userID)
	fmt.Fprintf(os.Stdout, "  firebase_uid     = %s\n", firebaseUID)
	fmt.Fprintf(os.Stdout, "  firebase_status  = %s\n", firebaseStatus(preexisting))
	fmt.Fprintf(os.Stdout, "  house_id         = %d\n", houseID)
	fmt.Fprintf(os.Stdout, "  role             = %s\n", opts.role)
	return nil
}

func firebaseStatus(preexisting bool) string {
	if preexisting {
		return "reused-existing"
	}
	return "created"
}

// resolveHouse returns the target House id and name, rejecting ambiguous or
// missing matches.
func resolveHouse(ctx context.Context, db *database.DB, opts *cliOptions) (int64, string, error) {
	if opts.houseID != 0 {
		var name string
		err := db.Pool.QueryRow(ctx,
			`SELECT name FROM houses WHERE id = $1`, opts.houseID,
		).Scan(&name)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", fmt.Errorf("no house with id %d", opts.houseID)
		}
		if err != nil {
			return 0, "", fmt.Errorf("lookup house by id: %w", err)
		}
		return opts.houseID, name, nil
	}

	rows, err := db.Pool.Query(ctx,
		`SELECT id, name FROM houses WHERE name = $1 ORDER BY id`, opts.houseName,
	)
	if err != nil {
		return 0, "", fmt.Errorf("lookup house by name: %w", err)
	}
	defer rows.Close()

	type match struct {
		id   int64
		name string
	}
	var matches []match
	for rows.Next() {
		var m match
		if err := rows.Scan(&m.id, &m.name); err != nil {
			return 0, "", fmt.Errorf("scan house row: %w", err)
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return 0, "", fmt.Errorf("iterate house rows: %w", err)
	}
	switch len(matches) {
	case 0:
		return 0, "", fmt.Errorf("no house with name %q", opts.houseName)
	case 1:
		return matches[0].id, matches[0].name, nil
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, fmt.Sprintf("%d", m.id))
		}
		return 0, "", fmt.Errorf("house name %q is ambiguous (matches ids: %s); use --house-id", opts.houseName, strings.Join(ids, ", "))
	}
}

// ensureFirebaseUser returns the Firebase UID for the email, creating the user
// if it does not already exist. preexisting indicates whether the user was
// found instead of created.
func ensureFirebaseUser(ctx context.Context, client *auth.Client, opts *cliOptions) (uid string, preexisting bool, err error) {
	user, err := client.GetUserByEmail(ctx, opts.email)
	if err == nil && user != nil {
		if opts.password != "" {
			slog.Warn("password ignored — user already exists in Firebase",
				"email", opts.email,
				"firebase_uid", user.UID,
			)
		}
		return user.UID, true, nil
	}
	if err != nil && !auth.IsUserNotFound(err) {
		return "", false, fmt.Errorf("firebase get user by email: %w", err)
	}

	if opts.password == "" {
		return "", false, errors.New("password is required when the Firebase user does not exist; pass --password or --password-stdin")
	}

	params := (&auth.UserToCreate{}).
		Email(opts.email).
		EmailVerified(false).
		Password(opts.password).
		DisplayName(opts.name).
		Disabled(false)

	created, err := client.CreateUser(ctx, params)
	if err != nil {
		return "", false, fmt.Errorf("firebase create user: %w", err)
	}
	slog.Info("firebase user created", "email", opts.email, "firebase_uid", created.UID)
	return created.UID, false, nil
}

// upsertMembership inserts or updates the users row and links it to the target
// House. It is safe to call multiple times for the same (firebase_uid, house_id)
// pair: the user record converges to the latest email/name and the membership
// is created at most once.
func upsertMembership(ctx context.Context, db *database.DB, firebaseUID, email, name string, houseID int64, role string) (int64, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO users (firebase_id, email, name)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (firebase_id) DO UPDATE
		   SET email = EXCLUDED.email,
		       name  = EXCLUDED.name
		 RETURNING id`,
		firebaseUID, email, name,
	).Scan(&userID)
	if err != nil {
		return 0, fmt.Errorf("upsert user: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO house_members (user_id, house_id, role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, house_id) DO NOTHING`,
		userID, houseID, role,
	); err != nil {
		return 0, fmt.Errorf("insert house_member: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}
	return userID, nil
}

func readPasswordFromReader(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	pwd := strings.TrimRight(string(data), "\r\n")
	if pwd == "" {
		return "", errors.New("empty password from stdin")
	}
	return pwd, nil
}
