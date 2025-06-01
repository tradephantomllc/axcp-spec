package internal

import (
	"time"

	bolt "go.etcd.io/bbolt"
)

type Buffer struct{ db *bolt.DB }

func NewBuffer(path string) (*Buffer, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}

	// Utilizziamo una funzione di callback con la firma corretta (un solo valore di ritorno error)
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("patch"))
		return err
	})
	if err != nil {
		return nil, err
	}

	return &Buffer{db: db}, nil
}
