package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"blogcms/internal/cmsio"
	"blogcms/internal/config"
	"blogcms/internal/postgres"
	"blogcms/internal/version"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "export":
		runExport(os.Args[2:])
	case "import":
		runImport(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `BlogCMS IO utility (export/import)
Version: %s

Usage:
  cmsio export -config ./configs/config.yml -out ./blogcms-backup.tar.gz [--no-uploads] [--no-settings] [--uploads-referenced-only]
  cmsio import -config ./configs/config.yml -in  ./blogcms-backup.tar.gz [--no-truncate] [--no-overwrite-uploads] [--no-settings]

Notes:
  - Uses the same YAML config as the server (db + storage.upload_dir).
  - Import expects the DB schema to already exist (run migrations first).

`, version.Version)
}

func runExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	var cfgPath string
	var outPath string
	noUploads := fs.Bool("no-uploads", false, "do not include uploads directory in archive")
	noSettings := fs.Bool("no-settings", false, "do not include settings table in archive")
	refOnly := fs.Bool("uploads-referenced-only", false, "include only files referenced from posts (best-effort)")
	uploadBase := fs.String("upload-base", "", "uploads URL prefix for reference detection (default: from config or /uploads/)")
	fs.StringVar(&cfgPath, "config", "", "path to YAML config file (optional)")
	fs.StringVar(&outPath, "out", "", "output archive path (default: ./blogcms-export-<timestamp>.tar.gz)")
	_ = fs.Parse(args)

	cfg, err := config.LoadFromPath(cfgPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	if outPath == "" {
		outPath = fmt.Sprintf("blogcms-export-%s.tar.gz", time.Now().UTC().Format("20060102-150405"))
	}
	outPath, _ = filepath.Abs(outPath)

	db, err := postgres.Open(cfg.DB.DSN, postgres.Options{
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime,
		PingTimeout:     cfg.DB.PingTimeout,
	})
	if err != nil {
		log.Fatalf("db error: %v", err)
	}
	defer db.Close()

	opt := cmsio.ExportOptions{
		IncludeUploads:        !*noUploads,
		IncludeSettings:       !*noSettings,
		UploadsReferencedOnly: *refOnly,
		UploadBasePrefix:      *uploadBase,
	}
	if opt.UploadBasePrefix == "" {
		opt.UploadBasePrefix = cfg.Storage.PublicBaseURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := cmsio.ExportToFile(ctx, db.DB, cfg.Storage.UploadDir, outPath, opt); err != nil {
		log.Fatalf("export error: %v", err)
	}

	fmt.Printf("OK: %s\n", outPath)
}

func runImport(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	var cfgPath string
	var inPath string
	noTruncate := fs.Bool("no-truncate", false, "do not truncate tables before import")
	noOverwrite := fs.Bool("no-overwrite-uploads", false, "do not overwrite existing upload files (skips on conflict)")
	noSettings := fs.Bool("no-settings", false, "ignore settings from archive")
	fs.StringVar(&cfgPath, "config", "", "path to YAML config file (optional)")
	fs.StringVar(&inPath, "in", "", "input archive path")
	_ = fs.Parse(args)

	if inPath == "" {
		log.Fatalf("import error: -in is required")
	}

	cfg, err := config.LoadFromPath(cfgPath)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	db, err := postgres.Open(cfg.DB.DSN, postgres.Options{
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime,
		PingTimeout:     cfg.DB.PingTimeout,
	})
	if err != nil {
		log.Fatalf("db error: %v", err)
	}
	defer db.Close()

	opt := cmsio.ImportOptions{
		TruncateTables:   !*noTruncate,
		OverwriteUploads: !*noOverwrite,
		IncludeSettings:  !*noSettings,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := cmsio.ImportFromFile(ctx, db.DB, cfg.Storage.UploadDir, inPath, opt); err != nil {
		log.Fatalf("import error: %v", err)
	}

	fmt.Printf("OK: imported from %s\n", inPath)
}
