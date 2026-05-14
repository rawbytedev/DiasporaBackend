package cache

import (
	"context"

	"github.com/rawbytedev/zerokv"
	"github.com/rawbytedev/zerokv/badgerdb"
	"github.com/rawbytedev/zerokv/encoders"
)

type CacheStore struct {
	badger zerokv.Core
	batch  zerokv.Batch
	enc    encoders.Encoder
}

// InvalidatePrefix removes all cache entries that start with the given prefix.
func (c *CacheStore) InvalidatePrefix(context context.Context, prefix string) error {
	inter := c.badger.Scan([]byte(prefix))
	for inter.Next() {
		c.badger.Delete(context, inter.Key())
	}
	return inter.Error()
}

// NewCache initializes a new CacheStore with the given directory path for storage.
func NewCache(path string, enc encoders.Encoder) (*CacheStore, error) {
	cache, err := badgerdb.NewBadgerDB(badgerdb.Config{Dir: path})
	if err != nil {
		return nil, err
	}
	if enc == nil {
		enc = encoders.NewJsonEncoder()
	}
	return &CacheStore{badger: cache, enc: enc}, nil
}

// Set stores a key-value pair in the cache.
func (c *CacheStore) Set(ctx context.Context, key string, value any) error {
	EnValue, err := c.enc.Encode(value)
	if err != nil {
		return err
	}
	return c.badger.Put(ctx, []byte(key), EnValue)
}

// Get retrieves the value associated with a key from the cache.
func (c *CacheStore) Get(ctx context.Context, key string, val any) error {
	value, err := c.badger.Get(ctx, []byte(key))
	if err != nil {
		return err
	}
	return c.enc.Decode(value, val)
}

// Delete removes a key-value pair from the cache.
func (c *CacheStore) Delete(ctx context.Context, key string) error {
	return c.badger.Delete(ctx, []byte(key))
}

// Close closes the cache.
func (c *CacheStore) Close() error {
    if c.badger != nil {
        return c.badger.Close()   // underlying DB close
    }
    return nil
}

// BatchPut adds a key-value pair to the batch for later commit.
func (c *CacheStore) BatchPut(ctx context.Context, key []byte, value []byte) error {
	if c.batch == nil {
		c.batch = c.badger.Batch()
	}
	return c.batch.Put(key, value)
}

// BatchDelete adds a key to the batch for later deletion.
func (c *CacheStore) BatchDelete(ctx context.Context, key []byte) error {
	if c.batch == nil {
		c.batch = c.badger.Batch()
	}
	return c.batch.Delete(key)
}

// Flush commits the batch to the cache.
func (c *CacheStore) Flush(ctx context.Context) error {
	return c.batch.Commit(ctx)
}
