package buffer

import (
	"fmt"
	"path/filepath"

	"go.etcd.io/bbolt"
)

const (
	bucketName = "retry"
)

// Open initializes and returns a new bbolt DB instance.
// The database file will be created at the specified path if it doesn't exist.
func Open(path string) (*bbolt.DB, error) {
	db, err := bbolt.Open(filepath.Clean(path), 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt db: %w", err)
	}

	// Create the bucket if it doesn't exist
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})

	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	return db, nil
}
