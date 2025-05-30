package internal

import (
	"time"

	bolt "go.etcd.io/bbolt"
)

type Buffer struct{ db *bolt.DB }

func NewBuffer(path string) (*Buffer, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil { return nil, err }
	db.Update(func(tx *bolt.Tx) (_, _ error) {
		_, _ = tx.CreateBucketIfNotExists([]byte("patch"))
		return
	})
	return &Buffer{db: db}, nil
}
