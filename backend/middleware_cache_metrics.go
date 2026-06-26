package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	redis "github.com/redis/go-redis/v9"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func runtimeMiddleware(next http.Handler, cfg Config, metrics *apiMetrics) http.Handler {
	limiter := newIPRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = newID("req")
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
		r = r.WithContext(ctx)

		if !authorizedRequest(r, cfg) {
			writeUnauthorized(w, requestID)
			duration := time.Since(started)
			if metrics != nil {
				metrics.Record(r.Method, r.URL.Path, http.StatusUnauthorized, duration)
			}
			log.Printf("request_id=%s method=%s path=%s status=%d duration_ms=%d remote=%s authorized=false\n", requestID, r.Method, r.URL.Path, http.StatusUnauthorized, duration.Milliseconds(), r.RemoteAddr)
			return
		}

		if r.Method != http.MethodOptions && !limiter.Allow(clientIP(r)) {
			writeTooManyRequests(w, requestID)
			duration := time.Since(started)
			if metrics != nil {
				metrics.Record(r.Method, r.URL.Path, http.StatusTooManyRequests, duration)
			}
			log.Printf("request_id=%s method=%s path=%s status=%d duration_ms=%d remote=%s limited=true\n", requestID, r.Method, r.URL.Path, http.StatusTooManyRequests, duration.Milliseconds(), r.RemoteAddr)
			return
		}

		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		duration := time.Since(started)
		if metrics != nil {
			metrics.Record(r.Method, r.URL.Path, status, duration)
		}
		log.Printf("request_id=%s method=%s path=%s status=%d duration_ms=%d bytes=%d remote=%s\n", requestID, r.Method, r.URL.Path, status, duration.Milliseconds(), recorder.bytes, r.RemoteAddr)
	})
}

func authorizedRequest(r *http.Request, cfg Config) bool {
	if cfg.APIToken == "" || r.Method == http.MethodOptions || r.URL.Path == "/api/health" {
		return true
	}
	token := strings.TrimSpace(r.Header.Get("X-SCRM-API-Token"))
	if token == "" {
		token = strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	}
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(cfg.APIToken)) == 1
}

func writeUnauthorized(w http.ResponseWriter, requestID string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}

func newIPRateLimiter(rps, burst int) *ipRateLimiter {
	if rps < 0 {
		rps = 0
	}
	if burst < rps {
		burst = rps
	}
	return &ipRateLimiter{
		rps:       float64(rps),
		burst:     float64(burst),
		clients:   map[string]*tokenBucket{},
		lastSweep: time.Now(),
	}
}

func (l *ipRateLimiter) Allow(ip string) bool {
	if l.rps == 0 || l.burst == 0 {
		return true
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	bucket, ok := l.clients[ip]
	if !ok {
		l.clients[ip] = &tokenBucket{tokens: l.burst - 1, last: now}
		l.sweepLocked(now)
		return true
	}
	elapsed := now.Sub(bucket.last).Seconds()
	bucket.tokens = minFloat(l.burst, bucket.tokens+elapsed*l.rps)
	bucket.last = now
	if bucket.tokens < 1 {
		l.sweepLocked(now)
		return false
	}
	bucket.tokens--
	l.sweepLocked(now)
	return true
}

func (l *ipRateLimiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	for ip, bucket := range l.clients {
		if now.Sub(bucket.last) > 5*time.Minute {
			delete(l.clients, ip)
		}
	}
	l.lastSweep = now
}

func newResponseCache(ttl time.Duration, maxEntries int) *responseCache {
	if ttl <= 0 || maxEntries <= 0 {
		return nil
	}
	return &responseCache{
		ttl:        ttl,
		maxEntries: maxEntries,
		entries:    map[string]cacheEntry{},
		lastSweep:  time.Now(),
	}
}

func (c *responseCache) Get(key string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	now := time.Now()
	c.mu.RLock()
	entry, ok := c.entries[key]
	if !ok || now.After(entry.expiresAt) {
		c.mu.RUnlock()
		if ok {
			c.Delete(key)
		}
		return nil, false
	}
	body := append([]byte(nil), entry.body...)
	c.mu.RUnlock()
	return body, true
}

func (c *responseCache) Set(key string, body []byte) {
	if c == nil {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sweepLocked(now)
	if len(c.entries) >= c.maxEntries {
		for existingKey := range c.entries {
			delete(c.entries, existingKey)
			break
		}
	}
	c.entries[key] = cacheEntry{
		body:      append([]byte(nil), body...),
		expiresAt: now.Add(c.ttl),
	}
}

func (c *responseCache) Delete(key string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.entries, key)
	c.mu.Unlock()
}

func (c *responseCache) DeletePrefix(prefix string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	for key := range c.entries {
		if strings.HasPrefix(key, prefix) {
			delete(c.entries, key)
		}
	}
	c.mu.Unlock()
}

func (c *responseCache) Stats() map[string]any {
	if c == nil {
		return map[string]any{"enabled": false}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]any{
		"enabled":    true,
		"backend":    "local",
		"ttlSeconds": int(c.ttl.Seconds()),
		"entries":    len(c.entries),
		"maxEntries": c.maxEntries,
	}
}

func (c *responseCache) sweepLocked(now time.Time) {
	if now.Sub(c.lastSweep) < time.Minute && len(c.entries) < c.maxEntries {
		return
	}
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
	c.lastSweep = now
}

func (c *responseCache) Close() error {
	return nil
}

func openCache(ctx context.Context, cfg Config) cacheStore {
	if cfg.CacheTTL <= 0 {
		log.Printf("AI SCRM cache disabled ttl=%s\n", cfg.CacheTTL)
		return nil
	}
	if cfg.RedisAddr != "" {
		cache, err := newRedisResponseCache(ctx, cfg)
		if err == nil {
			log.Printf("AI SCRM cache backend=redis addr=%s ttl=%s prefix=%s\n", cfg.RedisAddr, cfg.CacheTTL, cfg.RedisKeyPrefix)
			return cache
		}
		log.Printf("AI SCRM redis cache unavailable addr=%s error=%v fallback=local\n", cfg.RedisAddr, err)
	}
	cache := newResponseCache(cfg.CacheTTL, cfg.CacheMaxEntries)
	if cache != nil {
		log.Printf("AI SCRM cache backend=local ttl=%s max_entries=%d\n", cfg.CacheTTL, cfg.CacheMaxEntries)
	}
	return cache
}

func closeCache(cache cacheStore) {
	if cache == nil {
		return
	}
	if err := cache.Close(); err != nil {
		log.Printf("close cache: %v\n", err)
	}
}

func newRedisResponseCache(ctx context.Context, cfg Config) (*redisResponseCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		PoolSize:     max(10, cfg.DBMaxOpenConns),
		MinIdleConns: min(4, max(1, cfg.DBMaxIdleConns)),
	})
	pingCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &redisResponseCache{
		client:    client,
		ttl:       cfg.CacheTTL,
		keyPrefix: cfg.RedisKeyPrefix,
	}, nil
}

func (c *redisResponseCache) key(key string) string {
	return c.keyPrefix + key
}

func (c *redisResponseCache) Get(key string) ([]byte, bool) {
	if c == nil || c.client == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	body, err := c.client.Get(ctx, c.key(key)).Bytes()
	if err != nil {
		return nil, false
	}
	return body, true
}

func (c *redisResponseCache) Set(key string, body []byte) {
	if c == nil || c.client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := c.client.Set(ctx, c.key(key), body, c.ttl).Err(); err != nil {
		log.Printf("redis cache set key=%s error=%v\n", key, err)
	}
}

func (c *redisResponseCache) DeletePrefix(prefix string) {
	if c == nil || c.client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	iter := c.client.Scan(ctx, 0, c.key(prefix)+"*", 100).Iterator()
	keys := []string{}
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= 100 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				log.Printf("redis cache delete prefix=%s error=%v\n", prefix, err)
				return
			}
			keys = keys[:0]
		}
	}
	if err := iter.Err(); err != nil {
		log.Printf("redis cache scan prefix=%s error=%v\n", prefix, err)
		return
	}
	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			log.Printf("redis cache delete prefix=%s error=%v\n", prefix, err)
		}
	}
}

func (c *redisResponseCache) Stats() map[string]any {
	if c == nil || c.client == nil {
		return map[string]any{"enabled": false}
	}
	stats := c.client.PoolStats()
	return map[string]any{
		"enabled":    true,
		"backend":    "redis",
		"ttlSeconds": int(c.ttl.Seconds()),
		"keyPrefix":  c.keyPrefix,
		"hits":       stats.Hits,
		"misses":     stats.Misses,
		"totalConns": stats.TotalConns,
		"idleConns":  stats.IdleConns,
		"staleConns": stats.StaleConns,
		"timeouts":   stats.Timeouts,
	}
}

func (c *redisResponseCache) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func newAPIMetrics() *apiMetrics {
	return &apiMetrics{
		startedAt: time.Now(),
		bucketsMS: []float64{
			10, 25, 50, 100, 250, 500, 1000, 2500, 5000,
		},
		endpoints: map[string]*endpointMetric{},
	}
}

func (m *apiMetrics) Record(method, path string, status int, duration time.Duration) {
	if m == nil {
		return
	}
	route := routeMetricLabel(path)
	statusClass := strconv.Itoa(status/100) + "xx"
	key := method + "\x00" + route + "\x00" + statusClass
	durationMS := float64(duration.Microseconds()) / 1000

	m.mu.Lock()
	defer m.mu.Unlock()
	metric := m.endpoints[key]
	if metric == nil {
		metric = &endpointMetric{Buckets: make([]uint64, len(m.bucketsMS))}
		m.endpoints[key] = metric
	}
	metric.Requests++
	if status >= 400 {
		metric.Errors++
	}
	metric.DurationSumMS += durationMS
	for i, bucket := range m.bucketsMS {
		if durationMS <= bucket {
			metric.Buckets[i]++
		}
	}
}

func (m *apiMetrics) Snapshot() (time.Time, []float64, map[string]endpointMetric) {
	if m == nil {
		return time.Now(), nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := map[string]endpointMetric{}
	for key, metric := range m.endpoints {
		copied := *metric
		copied.Buckets = append([]uint64(nil), metric.Buckets...)
		snapshot[key] = copied
	}
	return m.startedAt, append([]float64(nil), m.bucketsMS...), snapshot
}

func routeMetricLabel(path string) string {
	parts := pathParts(path, "/api/")
	if len(parts) == 0 {
		return path
	}
	switch parts[0] {
	case "customers":
		if len(parts) == 1 {
			return "/api/customers"
		}
		if parts[1] == "batch-tags" || parts[1] == "batch-tags-async" {
			return "/api/customers/" + parts[1]
		}
		if len(parts) >= 3 {
			return "/api/customers/{id}/" + parts[2]
		}
		return "/api/customers/{id}"
	case "tags":
		if len(parts) == 1 {
			return "/api/tags"
		}
		if len(parts) >= 3 {
			return "/api/tags/{name}/" + parts[2]
		}
		return "/api/tags/{name}"
	case "tag-groups":
		return "/api/tag-groups"
	case "tasks":
		if len(parts) == 1 {
			return "/api/tasks"
		}
		if len(parts) >= 3 {
			return "/api/tasks/{id}/" + parts[2]
		}
		return "/api/tasks/{id}"
	case "events":
		if len(parts) >= 2 {
			return "/api/events/" + parts[1]
		}
		return "/api/events"
	case "stores":
		if len(parts) == 1 {
			return "/api/stores"
		}
		if len(parts) >= 3 {
			return "/api/stores/{id}/" + parts[2]
		}
		return "/api/stores/{id}"
	case "guides":
		if len(parts) == 1 {
			return "/api/guides"
		}
		if len(parts) >= 3 {
			return "/api/guides/{id}/" + parts[2]
		}
		return "/api/guides/{id}"
	case "groups":
		if len(parts) == 1 {
			return "/api/groups"
		}
		return "/api/groups/" + parts[1]
	default:
		return "/api/" + parts[0]
	}
}
