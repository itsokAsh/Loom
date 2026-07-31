package dburl

import (
	"net/url"
	"strings"
)

// WithDatabaseName returns a copy of a Postgres URL with the database name replaced.
func WithDatabaseName(dsn, dbName string) string {
	dbName = strings.TrimSpace(dbName)
	if dbName == "" {
		return dsn
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	u.Path = "/" + dbName
	return u.String()
}
