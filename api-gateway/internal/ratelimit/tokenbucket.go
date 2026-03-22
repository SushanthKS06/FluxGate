package ratelimit

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type TokenBucket struct {
	client *redis.Client
	key    string
	rate   int // tokens per minute
	burst  int
}

func NewTokenBucket(client *redis.Client, key string, rate int) *TokenBucket {
	return &TokenBucket{
		client: client,
		key:    key,
		rate:   rate,
		burst:  rate, // burst = rate for simplicity
	}
}

func (tb *TokenBucket) Allow(ctx context.Context) (bool, error) {
	now := time.Now().Unix()
	key := tb.key + ":tokens"

	// Lua script for atomic token bucket
	script := `
		local tokens_key = KEYS[1]
		local timestamp_key = KEYS[2]
		local rate = tonumber(ARGV[1])
		local burst = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local cost = 1

		local last_refill = redis.call('GET', timestamp_key)
		if not last_refill then
			last_refill = now
		else
			last_refill = tonumber(last_refill)
		end

		local refill = math.floor((now - last_refill) * rate / 60)
		local current = redis.call('GET', tokens_key)
		if not current then
			current = burst
		else
			current = tonumber(current)
		end

		current = math.min(burst, current + refill)
		local allowed = current >= cost
		if allowed then
			current = current - cost
			redis.call('SET', tokens_key, current)
			redis.call('SET', timestamp_key, now)
			redis.call('EXPIRE', tokens_key, 60)
			redis.call('EXPIRE', timestamp_key, 60)
		end
		return allowed
	`

	result, err := tb.client.Eval(ctx, script, []string{key, tb.key + ":timestamp"}, tb.rate, tb.burst, now).Result()
	if err != nil {
		return false, err
	}
	return result.(int64) == 1, nil
}
