package db

import (
	"github.com/dgraph-io/badger/v3"
	"github.com/tav/validate-rosetta/log"
)

// Store is an internal datastore for validate-rosetta data.
type Store struct {
	db *badger.DB
}

// Close closes the underlying Badger database.
func (s *Store) Close() error {
	return s.db.Close()
}

// New initializes the Store at the given path.
func New(dir string) (*Store, error) {
	opts := badger.DefaultOptions(dir).WithLogger(log.Badger{})
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &Store{
		db: db,
	}, nil
}
