package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func loadConfig() Config {
	businessTimeZone := envString("SCRM_BUSINESS_TIME_ZONE", "Asia/Shanghai")
	businessLocation, err := time.LoadLocation(businessTimeZone)
	if err != nil {
		log.Printf("invalid SCRM_BUSINESS_TIME_ZONE=%q, using Asia/Shanghai\n", businessTimeZone)
		businessTimeZone = "Asia/Shanghai"
		businessLocation = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	setBusinessLocation(businessLocation)
	return Config{
		Addr:              envString("SCRM_API_ADDR", "127.0.0.1:8080"),
		ReadHeaderTimeout: envDuration("SCRM_READ_HEADER_TIMEOUT", 2*time.Second),
		ReadTimeout:       envDuration("SCRM_READ_TIMEOUT", 10*time.Second),
		WriteTimeout:      envDuration("SCRM_WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:       envDuration("SCRM_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:   envDuration("SCRM_SHUTDOWN_TIMEOUT", 10*time.Second),
		RateLimitRPS:      envInt("SCRM_RATE_LIMIT_RPS", 80),
		RateLimitBurst:    envInt("SCRM_RATE_LIMIT_BURST", 160),
		APIToken:          envString("SCRM_API_TOKEN", ""),
		BusinessTimeZone:  businessTimeZone,
		BusinessLocation:  businessLocation,
		WorkerInterval:    envDuration("SCRM_WORKER_INTERVAL", 500*time.Millisecond),
		WorkerBatchSize:   envInt("SCRM_WORKER_BATCH_SIZE", 20),
		WorkerMaxAttempts: envInt("SCRM_WORKER_MAX_ATTEMPTS", 3),
		WorkerRetryDelay:  envDuration("SCRM_WORKER_RETRY_DELAY", 2*time.Second),
		DatabaseURL:       envString("SCRM_DATABASE_URL", ""),
		DBMaxOpenConns:    envInt("SCRM_DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    envInt("SCRM_DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetime: envDuration("SCRM_DB_CONN_MAX_LIFETIME", 30*time.Minute),
		DBPingTimeout:     envDuration("SCRM_DB_PING_TIMEOUT", 2*time.Second),
		AutoMigrate:       envBool("SCRM_AUTO_MIGRATE", false),
		CacheTTL:          envDuration("SCRM_CACHE_TTL", 5*time.Second),
		CacheMaxEntries:   envInt("SCRM_CACHE_MAX_ENTRIES", 512),
		RedisAddr:         envString("SCRM_REDIS_ADDR", ""),
		RedisPassword:     envString("SCRM_REDIS_PASSWORD", ""),
		RedisDB:           envInt("SCRM_REDIS_DB", 0),
		RedisKeyPrefix:    envString("SCRM_REDIS_KEY_PREFIX", "scrm:cache:"),
	}
}

func openDatabase(ctx context.Context, cfg Config) (*sql.DB, string, error) {
	if cfg.DatabaseURL == "" {
		log.Printf("AI SCRM storage mode=memory database_url=empty\n")
		return nil, "memory", nil
	}
	db, err := sql.Open("postgres", postgresURLWithTimeZone(cfg.DatabaseURL, cfg.BusinessTimeZone))
	if err != nil {
		return nil, "", fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(max(1, cfg.DBMaxOpenConns))
	db.SetMaxIdleConns(max(0, cfg.DBMaxIdleConns))
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.DBPingTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("ping postgres: %w", err)
	}
	if cfg.AutoMigrate {
		if err := runMigrations(ctx, db, "migrations"); err != nil {
			_ = db.Close()
			return nil, "", err
		}
	}
	log.Printf("AI SCRM storage mode=postgres max_open=%d max_idle=%d conn_lifetime=%s auto_migrate=%t timezone=%s\n", cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxLifetime, cfg.AutoMigrate, cfg.BusinessTimeZone)
	return db, "postgres", nil
}

func postgresURLWithTimeZone(databaseURL, timezone string) string {
	if strings.TrimSpace(timezone) == "" {
		return databaseURL
	}
	if parsed, err := url.Parse(databaseURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		values := parsed.Query()
		if values.Get("TimeZone") == "" && values.Get("timezone") == "" {
			values.Set("TimeZone", timezone)
			parsed.RawQuery = values.Encode()
		}
		return parsed.String()
	}
	if strings.Contains(databaseURL, "TimeZone=") || strings.Contains(databaseURL, "timezone=") {
		return databaseURL
	}
	return databaseURL + " TimeZone='" + strings.ReplaceAll(timezone, "'", "") + "'"
}

func closeDatabase(db *sql.DB) {
	if db == nil {
		return
	}
	if err := db.Close(); err != nil {
		log.Printf("close postgres: %v\n", err)
	}
}

func runMigrations(ctx context.Context, db *sql.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}
		if strings.TrimSpace(string(content)) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("run migration %s: %w", file, err)
		}
		log.Printf("migration applied file=%s\n", file)
	}
	return nil
}
