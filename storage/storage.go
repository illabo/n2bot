package storage

import (
	"fmt"
)

// DBInstancer is the interface to wrap different storage types
// and to provide just a minimal set of methods utilized by the application
type DBInstancer interface {
	Get(key []byte) ([]byte, error)
	GetAll() (map[string][]byte, error)
	Set(key []byte, value []byte) error
	Delete(key []byte) error
	Close() error
}

// NewInstance returns new instance of database wrapped in common DBInstancer interface
func NewInstance(cfg *Config) (DBInstancer, error) {
	if "level" == cfg.BackendType {
		return newLevelInstance(cfg)
	}
	return nil, fmt.Errorf("%s not implemented yet", cfg.BackendType)
}
