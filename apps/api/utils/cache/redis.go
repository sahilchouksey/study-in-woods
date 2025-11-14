package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrNotFound = errors.New("key not found in cache")
	ErrNil      = redis.Nil
)

// RedisCache wraps redis client with common cache operations
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(redisURL string) (*RedisCache, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCache{
		client: client,
	}, nil
}

// Get retrieves a value from cache
func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

// Set stores a value in cache with expiration
func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// SetJSON stores a JSON-encoded value in cache
func (r *RedisCache) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.Set(ctx, key, jsonData, expiration)
}

// GetJSON retrieves and decodes a JSON value from cache
func (r *RedisCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := r.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

// Delete removes a key from cache
func (r *RedisCache) Delete(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists in cache
func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Increment increments a counter and returns the new value
func (r *RedisCache) Increment(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// IncrementBy increments a counter by a specific amount
func (r *RedisCache) IncrementBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.IncrBy(ctx, key, value).Result()
}

// Expire sets an expiration time on a key
func (r *RedisCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// TTL returns the remaining time to live of a key
func (r *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// SetNX sets a value only if the key doesn't exist (for distributed locks)
func (r *RedisCache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, expiration).Result()
}

// GetSet atomically sets a key to a new value and returns the old value
func (r *RedisCache) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	return r.client.GetSet(ctx, key, value).Result()
}

// FlushAll removes all keys from the current database (use with caution!)
func (r *RedisCache) FlushAll(ctx context.Context) error {
	return r.client.FlushDB(ctx).Err()
}

// Close closes the Redis connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// GetClient returns the underlying Redis client for advanced operations
func (r *RedisCache) GetClient() *redis.Client {
	return r.client
}

// Keys returns all keys matching a pattern
func (r *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.client.Keys(ctx, pattern).Result()
}

// MGet retrieves multiple values at once
func (r *RedisCache) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	return r.client.MGet(ctx, keys...).Result()
}

// MSet sets multiple key-value pairs at once
func (r *RedisCache) MSet(ctx context.Context, values map[string]interface{}) error {
	return r.client.MSet(ctx, values).Err()
}

// HSet sets a field in a hash
func (r *RedisCache) HSet(ctx context.Context, key string, field string, value interface{}) error {
	return r.client.HSet(ctx, key, field, value).Err()
}

// HGet gets a field from a hash
func (r *RedisCache) HGet(ctx context.Context, key string, field string) (string, error) {
	val, err := r.client.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return "", ErrNotFound
	}
	return val, err
}

// HGetAll gets all fields from a hash
func (r *RedisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

// HDel deletes fields from a hash
func (r *RedisCache) HDel(ctx context.Context, key string, fields ...string) error {
	return r.client.HDel(ctx, key, fields...).Err()
}

// SAdd adds members to a set
func (r *RedisCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SAdd(ctx, key, members...).Err()
}

// SMembers gets all members of a set
func (r *RedisCache) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, key).Result()
}

// SIsMember checks if a value is a member of a set
func (r *RedisCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return r.client.SIsMember(ctx, key, member).Result()
}

// SRem removes members from a set
func (r *RedisCache) SRem(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SRem(ctx, key, members...).Err()
}
