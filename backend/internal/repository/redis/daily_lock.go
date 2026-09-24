package rediscache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type DailyLock struct{ rdb *redis.Client }

func NewDailyLock(rdb *redis.Client) *DailyLock { return &DailyLock{rdb: rdb} }

const (
	dailyGenLockKey   = "daily:gen:lock"
	adminRateLimitTTL = 15 * time.Minute
	adminMaxFails     = 5
)

// AcquireGenLock takes the SETNX lock guarding concurrent daily game
// generation. It returns false when another worker holds it.
func (l *DailyLock) AcquireGenLock(ctx context.Context, ttl time.Duration) (bool, error) {
	return l.rdb.SetNX(ctx, dailyGenLockKey, 1, ttl).Result()
}

func (l *DailyLock) ReleaseGenLock(ctx context.Context) error {
	return l.rdb.Del(ctx, dailyGenLockKey).Err()
}

// HitAdminFail increments the failed admin password counter for an IP
// and reports whether the limit (5 fails in 15 minutes) is exceeded.
func (l *DailyLock) HitAdminFail(ctx context.Context, ip string) (blocked bool, err error) {
	key := "admin:ratelimit:" + ip
	n, err := l.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		l.rdb.Expire(ctx, key, adminRateLimitTTL)
	}
	return n > adminMaxFails, nil
}

func (l *DailyLock) AdminBlocked(ctx context.Context, ip string) (bool, error) {
	n, err := l.rdb.Get(ctx, "admin:ratelimit:"+ip).Int()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return n > adminMaxFails, nil
}
