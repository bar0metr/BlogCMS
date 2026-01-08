package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"blogcms/internal/config"
	"blogcms/internal/postgres"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	var cfgPath string
	var username string
	var password string
	var cost int

	flag.StringVar(&cfgPath, "config", "", "path to YAML config file (optional)")
	flag.StringVar(&username, "username", "admin", "admin username")
	flag.StringVar(&password, "password", "", "admin password (if empty, read from stdin)")
	flag.IntVar(&cost, "cost", bcrypt.DefaultCost, "bcrypt cost (default: bcrypt.DefaultCost)")
	flag.Parse()

	username = strings.TrimSpace(username)
	if username == "" {
		fatalf("username is required")
	}
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		fatalf("invalid bcrypt cost %d (allowed %d..%d)", cost, bcrypt.MinCost, bcrypt.MaxCost)
	}

	cfg, err := config.LoadFromPath(cfgPath)
	if err != nil {
		fatalf("load config: %v", err)
	}

	pass, err := readPassword(password)
	if err != nil {
		fatalf("read password: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(pass), cost)
	if err != nil {
		fatalf("bcrypt hash: %v", err)
	}

	db, err := postgres.Open(cfg.DB.DSN, postgres.Options{
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime,
		PingTimeout:     cfg.DB.PingTimeout,
	})
	if err != nil {
		fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const q = `
INSERT INTO users (username, password_hash)
VALUES ($1, $2)
ON CONFLICT (username)
DO UPDATE SET password_hash = EXCLUDED.password_hash;
`
	if _, err := db.ExecContext(ctx, q, username, string(hash)); err != nil {
		fatalf("upsert user: %v", err)
	}

	fmt.Printf("Admin user %q created/updated successfully.\n", username)
}

func readPassword(flagValue string) (string, error) {
	if strings.TrimSpace(flagValue) != "" {
		return flagValue, nil
	}

	fmt.Fprint(os.Stderr, "Enter password: ")
	b1, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}

	fmt.Fprint(os.Stderr, "Confirm password: ")
	b2, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}

	p1 := string(b1)
	p2 := string(b2)
	if p1 == "" {
		return "", fmt.Errorf("empty password")
	}
	if p1 != p2 {
		return "", fmt.Errorf("passwords do not match")
	}
	return p1, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
