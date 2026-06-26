package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"
)

var (
	errNotFound   = errors.New("not found")
	errBadRequest = errors.New("bad request")
)

func (api *API) health(w http.ResponseWriter, r *http.Request) error {
	result := map[string]any{
		"status":  "ok",
		"time":    businessNow().Format(time.RFC3339),
		"storage": defaultString(api.storageMode, "memory"),
		"cache":   cacheStats(api.cache),
	}
	if api.db == nil {
		result["database"] = "disabled"
		return writeJSON(w, result)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 250*time.Millisecond)
	defer cancel()
	if err := api.db.PingContext(ctx); err != nil {
		result["status"] = "degraded"
		result["database"] = "unhealthy"
		result["databaseError"] = err.Error()
		return writeJSON(w, result)
	}
	result["database"] = "ok"
	stats := api.db.Stats()
	result["databasePool"] = map[string]any{
		"openConnections": stats.OpenConnections,
		"inUse":           stats.InUse,
		"idle":            stats.Idle,
		"waitCount":       stats.WaitCount,
	}
	return writeJSON(w, result)
}

func (api *API) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	api.writePrometheusMetrics(w, r.Context())
}

func (api *API) writePrometheusMetrics(w http.ResponseWriter, ctx context.Context) {
	startedAt, buckets, endpoints := api.metrics.Snapshot()
	fmt.Fprintf(w, "# HELP scrm_process_uptime_seconds Process uptime in seconds.\n")
	fmt.Fprintf(w, "# TYPE scrm_process_uptime_seconds gauge\n")
	fmt.Fprintf(w, "scrm_process_uptime_seconds %.0f\n", time.Since(startedAt).Seconds())

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	fmt.Fprintf(w, "# HELP scrm_runtime_goroutines Current goroutine count.\n")
	fmt.Fprintf(w, "# TYPE scrm_runtime_goroutines gauge\n")
	fmt.Fprintf(w, "scrm_runtime_goroutines %d\n", runtime.NumGoroutine())
	fmt.Fprintf(w, "# HELP scrm_runtime_heap_alloc_bytes Current heap allocation.\n")
	fmt.Fprintf(w, "# TYPE scrm_runtime_heap_alloc_bytes gauge\n")
	fmt.Fprintf(w, "scrm_runtime_heap_alloc_bytes %d\n", mem.HeapAlloc)

	keys := make([]string, 0, len(endpoints))
	for key := range endpoints {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Fprintf(w, "# HELP scrm_http_requests_total HTTP requests by method, route, and status class.\n")
	fmt.Fprintf(w, "# TYPE scrm_http_requests_total counter\n")
	fmt.Fprintf(w, "# HELP scrm_http_errors_total HTTP errors by method, route, and status class.\n")
	fmt.Fprintf(w, "# TYPE scrm_http_errors_total counter\n")
	fmt.Fprintf(w, "# HELP scrm_http_request_duration_ms HTTP request duration histogram in milliseconds.\n")
	fmt.Fprintf(w, "# TYPE scrm_http_request_duration_ms histogram\n")
	for _, key := range keys {
		parts := strings.Split(key, "\x00")
		if len(parts) != 3 {
			continue
		}
		method, route, statusClass := parts[0], parts[1], parts[2]
		metric := endpoints[key]
		labels := fmt.Sprintf(`method="%s",route="%s",status_class="%s"`, promLabel(method), promLabel(route), promLabel(statusClass))
		fmt.Fprintf(w, "scrm_http_requests_total{%s} %d\n", labels, metric.Requests)
		fmt.Fprintf(w, "scrm_http_errors_total{%s} %d\n", labels, metric.Errors)
		for i, bucket := range buckets {
			fmt.Fprintf(w, "scrm_http_request_duration_ms_bucket{%s,le=\"%.0f\"} %d\n", labels, bucket, metric.Buckets[i])
		}
		fmt.Fprintf(w, "scrm_http_request_duration_ms_bucket{%s,le=\"+Inf\"} %d\n", labels, metric.Requests)
		fmt.Fprintf(w, "scrm_http_request_duration_ms_sum{%s} %.3f\n", labels, metric.DurationSumMS)
		fmt.Fprintf(w, "scrm_http_request_duration_ms_count{%s} %d\n", labels, metric.Requests)
	}

	if api.db != nil {
		stats := api.db.Stats()
		fmt.Fprintf(w, "# HELP scrm_db_open_connections Current open database connections.\n")
		fmt.Fprintf(w, "# TYPE scrm_db_open_connections gauge\n")
		fmt.Fprintf(w, "scrm_db_open_connections %d\n", stats.OpenConnections)
		fmt.Fprintf(w, "scrm_db_in_use_connections %d\n", stats.InUse)
		fmt.Fprintf(w, "scrm_db_idle_connections %d\n", stats.Idle)
		fmt.Fprintf(w, "scrm_db_wait_count_total %d\n", stats.WaitCount)
		api.writeQueueMetrics(w, ctx)
	}
	api.writeCacheMetrics(w)
}

func (api *API) writeQueueMetrics(w http.ResponseWriter, ctx context.Context) {
	metricCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	queuedEvents, err := api.queryIntDB(metricCtx, `SELECT count(*) FROM event_inbox WHERE status = 'queued'`)
	if err != nil {
		fmt.Fprintf(w, "scrm_metric_query_error{query=\"queued_events\"} 1\n")
		return
	}
	queuedTaskItems, err := api.queryIntDB(metricCtx, `SELECT count(*) FROM task_items WHERE status = 'queued'`)
	if err != nil {
		fmt.Fprintf(w, "scrm_metric_query_error{query=\"queued_task_items\"} 1\n")
		return
	}
	runningTaskItems, err := api.queryIntDB(metricCtx, `SELECT count(*) FROM task_items WHERE status = 'running'`)
	if err != nil {
		fmt.Fprintf(w, "scrm_metric_query_error{query=\"running_task_items\"} 1\n")
		return
	}
	deadLetterTaskItems, err := api.queryIntDB(metricCtx, `SELECT count(*) FROM task_items WHERE status = 'dead_letter'`)
	if err != nil {
		fmt.Fprintf(w, "scrm_metric_query_error{query=\"dead_letter_task_items\"} 1\n")
		return
	}
	fmt.Fprintf(w, "# HELP scrm_event_inbox_queued Queued event inbox records.\n")
	fmt.Fprintf(w, "# TYPE scrm_event_inbox_queued gauge\n")
	fmt.Fprintf(w, "scrm_event_inbox_queued %d\n", queuedEvents)
	fmt.Fprintf(w, "# HELP scrm_task_items_queued Queued task items.\n")
	fmt.Fprintf(w, "# TYPE scrm_task_items_queued gauge\n")
	fmt.Fprintf(w, "scrm_task_items_queued %d\n", queuedTaskItems)
	fmt.Fprintf(w, "scrm_task_items_running %d\n", runningTaskItems)
	fmt.Fprintf(w, "# HELP scrm_task_items_dead_letter Dead-lettered task items.\n")
	fmt.Fprintf(w, "# TYPE scrm_task_items_dead_letter gauge\n")
	fmt.Fprintf(w, "scrm_task_items_dead_letter %d\n", deadLetterTaskItems)
}

func (api *API) writeCacheMetrics(w http.ResponseWriter) {
	stats := cacheStats(api.cache)
	enabled, _ := stats["enabled"].(bool)
	if !enabled {
		fmt.Fprintf(w, "scrm_cache_enabled 0\n")
		return
	}
	fmt.Fprintf(w, "scrm_cache_enabled 1\n")
	if backend, ok := stats["backend"].(string); ok {
		fmt.Fprintf(w, "scrm_cache_backend_info{backend=\"%s\"} 1\n", promLabel(backend))
	}
	if entries, ok := stats["entries"].(int); ok {
		fmt.Fprintf(w, "scrm_cache_entries %d\n", entries)
	}
	for _, name := range []string{"hits", "misses", "totalConns", "idleConns", "timeouts"} {
		if value, ok := stats[name]; ok {
			fmt.Fprintf(w, "scrm_cache_%s %v\n", promMetricName(name), value)
		}
	}
}

func (api *API) bootstrap(w http.ResponseWriter, r *http.Request) error {
	summary := map[string]any(nil)
	if api.db != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		var err error
		summary, err = api.summarySnapshotDB(ctx, metricDateFromRequest(r), false)
		if err != nil {
			return err
		}
	}
	api.mu.RLock()
	defer api.mu.RUnlock()
	if summary == nil {
		summary = api.summaryData()
	}
	return writeJSON(w, map[string]any{
		"summary":        summary,
		"stores":         api.stores,
		"guides":         api.guides,
		"customers":      api.customers,
		"handover":       api.handover,
		"touches":        api.touches,
		"groups":         api.groups,
		"groupMassTasks": api.groupMassTasks,
		"groupWelcomes":  api.groupWelcomes,
		"groupSOPs":      api.groupSOPs,
		"groupCalendar":  api.groupCalendar,
		"groupReminders": api.groupReminders,
		"groupTagGroups": api.groupTagGroups,
		"tags":           api.tags,
		"tagGroups":      api.tagGroups,
		"autoRules":      api.autoRules,
		"preTagRules":    api.preTagRules,
	})
}

func (api *API) summary(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		cacheKey := requestCacheKey("summary", r)
		if api.serveCachedJSON(w, r, cacheKey) {
			return nil
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		summary, err := api.summarySnapshotDB(ctx, metricDateFromRequest(r), false)
		if err != nil {
			return err
		}
		return api.writeCachedJSON(w, r, cacheKey, summary)
	}
	api.mu.RLock()
	defer api.mu.RUnlock()
	return writeJSON(w, api.summaryData())
}

func (api *API) refreshMetricsHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: metrics refresh requires postgres mode", errBadRequest)
	}
	if r.Method != http.MethodPost {
		return errNotFound
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	summary, err := api.summarySnapshotDB(ctx, metricDateFromRequest(r), true)
	if err != nil {
		return err
	}
	api.invalidateCache("summary:")
	return writeJSON(w, map[string]any{"status": "refreshed", "summary": summary})
}

func (api *API) summaryData() map[string]any {
	return map[string]any{
		"todayPool":       128,
		"attributionRate": "96.8%",
		"pending":         len(api.handover) + 160,
		"touchRate":       "82%",
		"trend":           []int{36, 42, 48, 38, 54, 66, 58},
		"suggestions":     []string{"南山店物料码入池环比 -20%", "高意向客户 48 人 24 小时未跟进", "天河代理店关注优先完成率更高"},
	}
}

type metricSnapshot struct {
	Key       string
	Value     float64
	Extra     json.RawMessage
	CreatedAt string
}

func (api *API) summarySnapshotDB(ctx context.Context, metricDate string, forceRefresh bool) (map[string]any, error) {
	snapshots, err := api.metricSnapshotsDB(ctx, metricDate)
	if err != nil {
		return nil, err
	}
	if forceRefresh || !metricSnapshotsComplete(snapshots) {
		snapshots, err = api.refreshMetricSnapshotsDB(ctx, metricDate)
		if err != nil {
			return nil, err
		}
	}
	return summaryFromMetricSnapshots(metricDate, snapshots), nil
}

func (api *API) metricSnapshotsDB(ctx context.Context, metricDate string) (map[string]metricSnapshot, error) {
	rows, err := api.db.QueryContext(ctx, `
		SELECT metric_key, metric_value::float8, extra::text,
			to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM metric_snapshots
		WHERE metric_date = $1::date AND scope_type = 'global' AND scope_id = 'all'
	`, metricDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	snapshots := map[string]metricSnapshot{}
	for rows.Next() {
		var snapshot metricSnapshot
		var extra string
		if err := rows.Scan(&snapshot.Key, &snapshot.Value, &extra, &snapshot.CreatedAt); err != nil {
			return nil, err
		}
		snapshot.Extra = json.RawMessage(extra)
		snapshots[snapshot.Key] = snapshot
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func (api *API) refreshMetricSnapshotsDB(ctx context.Context, metricDate string) (map[string]metricSnapshot, error) {
	totalCustomers, err := api.queryIntDB(ctx, `SELECT count(*) FROM customers`)
	if err != nil {
		return nil, err
	}
	todayPool, err := api.queryIntDB(ctx, `SELECT count(*) FROM customers WHERE created_at >= $1::date AND created_at < $1::date + INTERVAL '1 day'`, metricDate)
	if err != nil {
		return nil, err
	}
	attributed, err := api.queryIntDB(ctx, `
		SELECT count(*)
		FROM customer_attributions
		WHERE source_store_id <> '' AND first_staff_id <> '' AND source_code_id <> ''
	`)
	if err != nil {
		return nil, err
	}
	touched, err := api.queryIntDB(ctx, `
		SELECT count(DISTINCT c.id)
		FROM customers c
		WHERE EXISTS (SELECT 1 FROM customer_tags ct WHERE ct.customer_id = c.id)
			OR EXISTS (SELECT 1 FROM customer_events ce WHERE ce.customer_id = c.id)
	`)
	if err != nil {
		return nil, err
	}
	pendingTasks, err := api.queryIntDB(ctx, `SELECT count(*) FROM task_items WHERE status IN ('queued', 'running')`)
	if err != nil {
		return nil, err
	}
	pendingEvents, err := api.queryIntDB(ctx, `SELECT count(*) FROM event_inbox WHERE status = 'queued'`)
	if err != nil {
		return nil, err
	}
	handoverCustomers, err := api.queryIntDB(ctx, `SELECT count(*) FROM store_handover_items WHERE status IN ('待同步', '待处理')`)
	if err != nil {
		return nil, err
	}
	highIntentCustomers, err := api.queryIntDB(ctx, `
		SELECT count(DISTINCT ct.customer_id)
		FROM customer_tags ct
		JOIN tags t ON t.id = ct.tag_id
		WHERE t.name = '高意向'
	`)
	if err != nil {
		return nil, err
	}
	topStoreName, topStoreCount, err := api.topStoreMetricDB(ctx)
	if err != nil {
		return nil, err
	}
	trend, err := api.customerTrendDB(ctx, metricDate)
	if err != nil {
		return nil, err
	}

	attributionRate := percent(attributed, totalCustomers)
	touchRate := percent(touched, totalCustomers)
	pending := pendingTasks + pendingEvents + handoverCustomers
	suggestions := dashboardSuggestions(topStoreName, topStoreCount, highIntentCustomers, pendingTasks, pendingEvents, handoverCustomers)

	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	entries := []struct {
		key   string
		value float64
		extra any
	}{
		{key: "today_pool", value: float64(todayPool), extra: map[string]any{"unit": "customers"}},
		{key: "attribution_rate", value: attributionRate, extra: map[string]any{"unit": "percent", "numerator": attributed, "denominator": totalCustomers}},
		{key: "pending_items", value: float64(pending), extra: map[string]any{"tasks": pendingTasks, "events": pendingEvents, "handoverCustomers": handoverCustomers}},
		{key: "touch_rate", value: touchRate, extra: map[string]any{"unit": "percent", "numerator": touched, "denominator": totalCustomers}},
		{key: "trend", value: 0, extra: map[string]any{"values": trend}},
		{key: "suggestions", value: 0, extra: map[string]any{"items": suggestions}},
	}
	for _, entry := range entries {
		extra, err := json.Marshal(entry.extra)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO metric_snapshots (id, metric_date, scope_type, scope_id, metric_key, metric_value, extra, created_at)
			VALUES ($1, $2::date, 'global', 'all', $3, $4, $5::jsonb, now())
			ON CONFLICT (metric_date, scope_type, scope_id, metric_key) DO UPDATE SET
				metric_value = EXCLUDED.metric_value,
				extra = EXCLUDED.extra,
				created_at = now()
		`, newID("met"), metricDate, entry.key, entry.value, string(extra)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return api.metricSnapshotsDB(ctx, metricDate)
}

func (api *API) queryIntDB(ctx context.Context, query string, args ...any) (int, error) {
	var value int
	err := api.db.QueryRowContext(ctx, query, args...).Scan(&value)
	return value, err
}

func (api *API) topStoreMetricDB(ctx context.Context) (string, int, error) {
	var name string
	var total int
	err := api.db.QueryRowContext(ctx, `
		SELECT source_store_name, count(*)::int
		FROM customers
		WHERE source_store_name <> ''
		GROUP BY source_store_name
		ORDER BY count(*) DESC, source_store_name ASC
		LIMIT 1
	`).Scan(&name, &total)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil
	}
	return name, total, err
}

func (api *API) customerTrendDB(ctx context.Context, metricDate string) ([]int, error) {
	rows, err := api.db.QueryContext(ctx, `
		WITH days AS (
			SELECT generate_series($1::date - INTERVAL '6 days', $1::date, INTERVAL '1 day')::date AS day
		)
		SELECT count(c.id)::int
		FROM days d
		LEFT JOIN customers c ON c.created_at >= d.day AND c.created_at < d.day + INTERVAL '1 day'
		GROUP BY d.day
		ORDER BY d.day
	`, metricDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := []int{}
	maxCount := 0
	for rows.Next() {
		var count int
		if err := rows.Scan(&count); err != nil {
			return nil, err
		}
		counts = append(counts, count)
		if count > maxCount {
			maxCount = count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if maxCount == 0 {
		return []int{0, 0, 0, 0, 0, 0, 0}, nil
	}
	trend := make([]int, 0, len(counts))
	for _, count := range counts {
		if count == 0 {
			trend = append(trend, 0)
			continue
		}
		value := count * 100 / maxCount
		if value < 12 {
			value = 12
		}
		trend = append(trend, value)
	}
	return trend, nil
}

func metricSnapshotsComplete(snapshots map[string]metricSnapshot) bool {
	required := []string{"today_pool", "attribution_rate", "pending_items", "touch_rate", "trend", "suggestions"}
	for _, key := range required {
		if _, ok := snapshots[key]; !ok {
			return false
		}
	}
	return true
}

func summaryFromMetricSnapshots(metricDate string, snapshots map[string]metricSnapshot) map[string]any {
	trend := metricExtraInts(snapshots["trend"].Extra, "values", []int{0, 0, 0, 0, 0, 0, 0})
	suggestions := metricExtraStrings(snapshots["suggestions"].Extra, "items", []string{})
	snapshotAt := ""
	for _, snapshot := range snapshots {
		if snapshot.CreatedAt > snapshotAt {
			snapshotAt = snapshot.CreatedAt
		}
	}
	return map[string]any{
		"todayPool":       metricInt(snapshots["today_pool"].Value),
		"attributionRate": fmt.Sprintf("%.1f%%", snapshots["attribution_rate"].Value),
		"pending":         metricInt(snapshots["pending_items"].Value),
		"touchRate":       fmt.Sprintf("%.0f%%", snapshots["touch_rate"].Value),
		"trend":           trend,
		"suggestions":     suggestions,
		"snapshotDate":    metricDate,
		"snapshotAt":      snapshotAt,
		"source":          "metric_snapshots",
	}
}

func metricExtraInts(raw json.RawMessage, key string, fallback []int) []int {
	var payload map[string][]int
	if len(raw) == 0 || json.Unmarshal(raw, &payload) != nil {
		return fallback
	}
	if values, ok := payload[key]; ok {
		return values
	}
	return fallback
}

func metricExtraStrings(raw json.RawMessage, key string, fallback []string) []string {
	var payload map[string][]string
	if len(raw) == 0 || json.Unmarshal(raw, &payload) != nil {
		return fallback
	}
	if values, ok := payload[key]; ok {
		return values
	}
	return fallback
}

func metricDateFromRequest(r *http.Request) string {
	value := strings.TrimSpace(r.URL.Query().Get("date"))
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return value
	}
	return today()
}

func metricInt(value float64) int {
	if value < 0 {
		return int(value - 0.5)
	}
	return int(value + 0.5)
}

func percent(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

func dashboardSuggestions(topStoreName string, topStoreCount, highIntentCustomers, pendingTasks, pendingEvents, handoverCustomers int) []string {
	suggestions := []string{}
	if topStoreName != "" {
		suggestions = append(suggestions, fmt.Sprintf("%s 当前入池最高，共 %d 人，建议复盘来源码和导购承接。", topStoreName, topStoreCount))
	}
	if highIntentCustomers > 0 {
		suggestions = append(suggestions, fmt.Sprintf("高意向客户 %d 人，建议优先进入导购跟进和转化任务。", highIntentCustomers))
	}
	if pendingTasks+pendingEvents+handoverCustomers > 0 {
		suggestions = append(suggestions, fmt.Sprintf("当前待处理 %d 项：任务 %d、事件 %d、交接客户 %d。", pendingTasks+pendingEvents+handoverCustomers, pendingTasks, pendingEvents, handoverCustomers))
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "当前核心链路无明显积压，可继续观察门店入池和触达覆盖。")
	}
	return suggestions
}
