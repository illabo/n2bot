package storage

// Config for storage backend package.
type Config struct {
	// BackendType is the type of DB used to store per user per download data.
	// Currently only "level" could be set to use goleveldb as the storage backend.
	BackendType string
	// Path is the relative path to database file in case of goleveldb, boltdb, badgerdb
	// or connection url for SQLs. Note that only goleveldb is available right now.
	Path string
}
