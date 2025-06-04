package buffer

import (
	"encoding/binary"
	"time"

	"go.etcd.io/bbolt"
)

const queueBucketName = "message_queue"

type Queue struct {
	db *bbolt.DB
}

// NewQueue creates a new Queue instance with the given bbolt DB
func NewQueue(db *bbolt.DB) *Queue {
	// Ensure the bucket exists
	_ = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(queueBucketName))
		return err
	})
	return &Queue{db: db}
}

// Push adds a new item to the queue with a timestamp prefix for ordering
func (q *Queue) Push(key, val []byte) error {
	return q.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(queueBucketName))
		if err != nil {
			return err
		}

		// Create a key with timestamp prefix for ordering
		keyBuf := make([]byte, 8+len(key))
		binary.BigEndian.PutUint64(keyBuf, uint64(time.Now().UnixNano()))
		copy(keyBuf[8:], key)

		return b.Put(keyBuf, val)
	})
}

// Pop removes and returns up to n items from the queue
func (q *Queue) Pop(n int) ([][]byte, error) {
	var results [][]byte

	err := q.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(queueBucketName))
		if err != nil {
			return err
		}

		c := b.Cursor()
		count := 0

		for k, v := c.First(); k != nil && count < n; k, v = c.Next() {
			// Make a copy of the value before adding to results
			valCopy := make([]byte, len(v))
			copy(valCopy, v)

			results = append(results, valCopy)

			// Remove the item from the bucket
			if err := c.Delete(); err != nil {
				return err
			}
			count++
		}


		return nil
	})

	return results, err
}

// Len returns the number of items in the queue
func (q *Queue) Len() (int, error) {
	var count int
	err := q.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(queueBucketName))
		if b == nil {
			// Bucket doesn't exist yet, so count is 0
			count = 0
			return nil
		}

		stats := b.Stats()
		count = stats.KeyN
		return nil
	})

	return count, err
}
