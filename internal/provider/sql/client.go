package sql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"

	// Register the MySQL driver for SingleStore SQL connections.
	_ "github.com/go-sql-driver/mysql"
)

const (
	defaultUsername = "admin"
	defaultPort     = 3306
	defaultTLS      = "preferred"
)

// ConnectionConfig holds the parameters required to connect to a SingleStore workspace.
type ConnectionConfig struct {
	Endpoint string
	Username string
	Password string
	Database string
	Port     int
	TLS      string
}

// Client executes SQL statements against a SingleStore workspace.
type Client struct {
	db *sql.DB
}

// Open creates a SQL client for the given connection configuration.
func Open(ctx context.Context, cfg ConnectionConfig) (*Client, error) {
	if cfg.Username == "" {
		cfg.Username = defaultUsername
	}

	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}

	if cfg.TLS == "" {
		cfg.TLS = defaultTLS
	}

	dsn := buildDSN(cfg)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening SQL connection: %w", err)
	}

	db.SetConnMaxLifetime(time.Hour)
	db.SetMaxIdleConns(config.TestMaxIdleConns)
	db.SetMaxOpenConns(config.TestMaxOpenConns)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("pinging SQL connection: %w", err)
	}

	return &Client{db: db}, nil
}

func buildDSN(cfg ConnectionConfig) string {
	params := url.Values{}
	params.Set("parseTime", "true")
	params.Set("interpolateParams", "true")
	params.Set("timeout", "30s")
	params.Set("tls", cfg.TLS)
	params.Set("multiStatements", "true")

	hostPort := fmt.Sprintf("%s:%d", cfg.Endpoint, cfg.Port)

	return fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?%s",
		cfg.Username,
		cfg.Password,
		hostPort,
		cfg.Database,
		params.Encode(),
	)
}

// Exec runs a SQL statement.
func (c *Client) Exec(ctx context.Context, query string) error {
	if strings.TrimSpace(query) == "" {
		return nil
	}

	_, err := c.db.ExecContext(ctx, query)
	if err != nil {
		return err
	}

	return nil
}

// Query runs a read-only SQL statement and returns each row as a string map.
func (c *Client) Query(ctx context.Context, query string) ([]map[string]string, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]string

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]string, len(columns))
		for i, column := range columns {
			row[column] = valueToString(values[i])
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// Close closes the underlying database connection.
func (c *Client) Close() error {
	if c.db == nil {
		return nil
	}

	return c.db.Close()
}

func valueToString(value any) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
