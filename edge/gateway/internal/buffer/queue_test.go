package buffer_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bolt "go.etcd.io/bbolt"

	"github.com/tradephantom/axcp-spec/edge/gateway/internal/buffer"
)

func TestQueue(t *testing.T) {
	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "bolt-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Open the database
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	require.NoError(t, err)
	defer db.Close()

	// Create a new queue
	queue := buffer.NewQueue(db)

	t.Run("Test empty queue", func(t *testing.T) {
		// Test Len on empty queue
		count, err := queue.Len()
		assert.NoError(t, err)
		assert.Equal(t, 0, count)

		// Test Pop on empty queue
		items, err := queue.Pop(10)
		assert.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("Test push and pop", func(t *testing.T) {
		// Push some items
		testData := [][]byte{
			[]byte("test1"),
			[]byte("test2"),
			[]byte("test3"),
		}

		for i, data := range testData {
			err := queue.Push([]byte{byte(i)}, data)
			assert.NoError(t, err)
		}

		// Verify length
		count, err := queue.Len()
		assert.NoError(t, err)
		assert.Equal(t, len(testData), count)

		// Pop items
		items, err := queue.Pop(2)
		assert.NoError(t, err)
		assert.Len(t, items, 2)

		// Verify remaining length
		count, err = queue.Len()
		assert.NoError(t, err)
		assert.Equal(t, 1, count)

		// Pop remaining items
		items, err = queue.Pop(10)
		assert.NoError(t, err)
		assert.Len(t, items, 1)

		// Queue should be empty now
		count, err = queue.Len()
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("Test ordering", func(t *testing.T) {
		// Push items with timestamps
		err := queue.Push([]byte("key1"), []byte("value1"))
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond) // Ensure different timestamps

		err = queue.Push([]byte("key2"), []byte("value2"))
		require.NoError(t, err)

		// Should get items in FIFO order
		items, err := queue.Pop(2)
		assert.NoError(t, err)
		assert.Len(t, items, 2)
		assert.Equal(t, "value1", string(items[0]))
		assert.Equal(t, "value2", string(items[1]))
	})
}
