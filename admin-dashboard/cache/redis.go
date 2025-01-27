package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// Global variables
var (
	RedisClient  *redis.Client
	redisEnabled bool
	ctx          = context.Background()
	singleFlight singleflight.Group
)

const (
	THREAD_CACHE_DURATION      = 5 * time.Minute
	PROFILE_PIC_CACHE_DURATION = 24 * time.Hour
)

type ThreadPreview struct {
	ID                string
	ThreadID          string
	FromUser          string
	Content           string
	Timestamp         time.Time
	Platform          string
	BotEnabled        bool
	ProfilePictureURL string
}

// Initialize Redis connection
func InitRedis() {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT"),
		Username: os.Getenv("REDIS_USERNAME"),
		Password: os.Getenv("REDIS_PASSWORD"),
	})

	// Test connection
	if _, err := RedisClient.Ping(ctx).Result(); err != nil {
		redisEnabled = false
		log.Printf("Redis connection failed: %v", err)
	} else {
		redisEnabled = true
		log.Println("Redis connected successfully")
	}
}

// BulkGetProfilePictures fetches multiple profile pictures in a single Redis pipeline
func BulkGetProfilePictures(userIDs []string) (map[string]string, error) {
	if !redisEnabled || len(userIDs) == 0 {
		return make(map[string]string), nil
	}

	pipe := RedisClient.Pipeline()
	cmds := make(map[string]*redis.StringCmd)

	// Create pipeline commands
	for _, userID := range userIDs {
		key := fmt.Sprintf("profile:%s:picture", userID)
		cmds[userID] = pipe.Get(ctx, key)
	}

	// Execute pipeline
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("bulk cache get failed: %v", err)
	}

	// Collect results
	results := make(map[string]string)
	for userID, cmd := range cmds {
		if val, err := cmd.Result(); err == nil {
			results[userID] = val
		}
	}

	return results, nil
}

// CacheProfilePicture stores a profile picture URL in Redis
func CacheProfilePicture(userID, url string) error {
	if !redisEnabled {
		return nil
	}
	key := fmt.Sprintf("profile:%s:picture", userID)
	return RedisClient.Set(ctx, key, url, PROFILE_PIC_CACHE_DURATION).Err()
}

// GetProfilePicture retrieves a profile picture URL from Redis or falls back to DB
func GetProfilePicture(userID string, dbFetch func(string) (string, error)) (string, error) {
	if !redisEnabled {
		return dbFetch(userID)
	}

	key := fmt.Sprintf("profile:%s:picture", userID)

	// Attempt cache
	if url, err := RedisClient.Get(ctx, key).Result(); err == nil {
		log.Printf("✅ Cache HIT for profile picture: %s", userID)
		return url, nil
	}

	log.Printf("❌ Cache MISS for profile picture: %s", userID)

	// Cache miss - fetch from DB
	url, err := dbFetch(userID)
	if err != nil {
		return "", err
	}

	// Update cache async
	go func() {
		if err := CacheProfilePicture(userID, url); err != nil {
			log.Printf("Failed to cache profile picture: %v", err)
		} else {
			log.Printf("✅ Successfully cached profile picture for: %s", userID)
		}
	}()

	return url, nil
}

// CacheThreadPreview stores a thread preview in Redis
func CacheThreadPreview(threadID string, preview ThreadPreview) error {
	if !redisEnabled {
		return nil
	}
	key := fmt.Sprintf("thread:%s:preview", threadID)
	return RedisClient.Set(ctx, key, preview, THREAD_CACHE_DURATION).Err()
}

// GetThreadPreview retrieves a thread preview from Redis or falls back to DB
func GetThreadPreview(threadID string, dbFetch func(string) (ThreadPreview, error)) (ThreadPreview, error) {
	if !redisEnabled {
		return dbFetch(threadID)
	}

	key := fmt.Sprintf("thread:%s:preview", threadID)

	// Attempt cache
	data, err := RedisClient.Get(ctx, key).Bytes()
	if err == nil {
		log.Printf("✅ Cache HIT for thread preview: %s", threadID)
		var preview ThreadPreview
		if err := json.Unmarshal(data, &preview); err == nil {
			return preview, nil
		}
	}

	log.Printf("❌ Cache MISS for thread preview: %s", threadID)

	// Cache miss - fetch from DB
	preview, err := dbFetch(threadID)
	if err != nil {
		return ThreadPreview{}, err
	}

	// Update cache async
	go func() {
		if err := CacheThreadPreview(threadID, preview); err != nil {
			log.Printf("Failed to cache thread preview: %v", err)
		}
	}()

	return preview, nil
}

// InvalidateThreadCache removes a thread preview from Redis
func InvalidateThreadCache(threadID string) error {
	if !redisEnabled {
		return nil
	}
	key := fmt.Sprintf("thread:%s:preview", threadID)
	return RedisClient.Del(ctx, key).Err()
}
