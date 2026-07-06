package main

import (
	rand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func customer(id, name, wecom, mobile, owner, source, sourceStore, stage string, intent int, salesStage, conversionSource, dealAmount, nextFollowUp string, tags, wecomTags []string, relations []Relation, signals []string) Customer {
	return Customer{
		ID: id, Name: name, Wecom: wecom, Mobile: mobile, Owner: owner, Source: source, SourceStore: sourceStore, Stage: stage,
		TagGroup: "行为标签 / 生命周期标签", Tags: tags, WecomTags: wecomTags, LastActive: "今天 15:10", IntentScore: intent,
		SalesStage: salesStage, ConversionSource: conversionSource, DealAmount: dealAmount, NextFollowUp: nextFollowUp, Relations: relations, Signals: signals,
		Timeline: []AuditEvent{audit("2026-06-25 15:10", "扫码入池", "已锁定来源并进入客户池", "系统", "客户："+name+"；导购："+owner, "来源活码："+source)},
	}
}

func customerEventAction(eventType string) string {
	switch eventType {
	case "batch_customer_tags":
		return "批量打标签"
	case "touch_task":
		return "创建触达任务"
	case "followup":
		return "新增跟进"
	case "relation_add":
		return "新增关联导购"
	case "relation_set_main":
		return "设置主跟进导购"
	case "relation_end":
		return "结束关联导购"
	case "handover_submit":
		return "提交导购交接"
	case "lead":
		return "建线索 / 商机"
	case "order":
		return "记录成交 / 回款"
	case "lost":
		return "标记流失"
	default:
		return eventType
	}
}

func customerEventResult(eventType string) string {
	switch eventType {
	case "batch_customer_tags":
		return "系统标签已更新"
	case "touch_task":
		return "待导购确认发送"
	case "followup":
		return "已记录客户沟通结果"
	case "relation_add":
		return "已建立服务关系"
	case "relation_set_main":
		return "主跟进已更新"
	case "relation_end":
		return "关系已结束"
	case "handover_submit":
		return "主跟进导购已更新"
	case "lead":
		return "已进入销售承接"
	case "order":
		return "已回写标签和转化阶段"
	case "lost":
		return "客户阶段已更新"
	default:
		return "已记录"
	}
}

func customerEventDetail(eventType, source, objectID string, payload map[string]any) string {
	if detail := stringPayload(payload, "detail"); detail != "" {
		return detail
	}
	switch eventType {
	case "batch_customer_tags":
		return strings.Join(stringSlicePayload(payload, "tags"), "、")
	case "touch_task":
		return strings.Trim(strings.Join([]string{stringPayload(payload, "method"), stringPayload(payload, "content")}, "｜"), "｜")
	case "followup":
		return stringPayload(payload, "nextFollowUp")
	case "order":
		return strings.Trim(strings.Join([]string{"成交来源：" + stringPayload(payload, "source"), "金额：" + stringPayload(payload, "amount")}, "；"), "；")
	default:
		if objectID != "" {
			return source + "：" + objectID
		}
		return source
	}
}

func stringPayload(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

func stringSlicePayload(payload map[string]any, key string) []string {
	value, ok := payload[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		rows := []string{}
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				rows = append(rows, text)
			}
		}
		return rows
	default:
		text := strings.TrimSpace(fmt.Sprint(typed))
		if text == "" {
			return nil
		}
		return []string{text}
	}
}

func jsonStringArray(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	body, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func stringsFromJSONArray(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	rows := []string{}
	if err := json.Unmarshal([]byte(value), &rows); err == nil {
		return rows
	}
	return []string{value}
}

func parseCNYCents(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	var builder strings.Builder
	for _, r := range value {
		if (r >= '0' && r <= '9') || r == '.' {
			builder.WriteRune(r)
		}
	}
	amount, err := strconv.ParseFloat(builder.String(), 64)
	if err != nil {
		return 0
	}
	return int64(amount*100 + 0.5)
}

func formatCNYCents(value int64) string {
	if value <= 0 {
		return ""
	}
	yuan := value / 100
	cents := value % 100
	if cents == 0 {
		return "¥" + formatIntWithComma(yuan)
	}
	return fmt.Sprintf("¥%s.%02d", formatIntWithComma(yuan), cents)
}

func formatIntWithComma(value int64) string {
	text := strconv.FormatInt(value, 10)
	if len(text) <= 3 {
		return text
	}
	parts := []string{}
	for len(text) > 3 {
		parts = append([]string{text[len(text)-3:]}, parts...)
		text = text[:len(text)-3]
	}
	parts = append([]string{text}, parts...)
	return strings.Join(parts, ",")
}

func writeJSON(w http.ResponseWriter, v any) error {
	return json.NewEncoder(w).Encode(v)
}

func (api *API) serveCachedJSON(w http.ResponseWriter, r *http.Request, key string) bool {
	if r.Method != http.MethodGet || api.cache == nil || key == "" {
		return false
	}
	body, ok := api.cache.Get(key)
	if !ok {
		return false
	}
	w.Header().Set("X-SCRM-Cache", "HIT")
	_, _ = w.Write(body)
	return true
}

func (api *API) writeCachedJSON(w http.ResponseWriter, r *http.Request, key string, v any) error {
	if r.Method != http.MethodGet || api.cache == nil || key == "" {
		return writeJSON(w, v)
	}
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	api.cache.Set(key, body)
	w.Header().Set("X-SCRM-Cache", "MISS")
	_, err = w.Write(body)
	return err
}

func (api *API) invalidateCache(prefixes ...string) {
	if api.cache == nil {
		return
	}
	for _, prefix := range prefixes {
		if prefix != "" {
			api.cache.DeletePrefix(prefix)
		}
	}
}

func requestCacheKey(namespace string, r *http.Request) string {
	return namespace + ":" + r.URL.RequestURI()
}

func cacheStats(cache cacheStore) map[string]any {
	if cache == nil {
		return map[string]any{"enabled": false}
	}
	return cache.Stats()
}

func promLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return value
}

func promMetricName(value string) string {
	var builder strings.Builder
	for i, r := range value {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				builder.WriteByte('_')
			}
			builder.WriteRune(r + ('a' - 'A'))
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
	}
	return builder.String()
}

func writePaginatedOrList[T any](w http.ResponseWriter, r *http.Request, rows []T) error {
	page, pageSize, ok, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !ok {
		return writeJSON(w, rows)
	}
	total := len(rows)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	result := PageResult[T]{
		Data: rows[start:end],
		Page: makePageMeta(page, pageSize, total),
	}
	return writeJSON(w, result)
}

func makePageMeta(page, pageSize, total int) PageMeta {
	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	return PageMeta{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page*pageSize < total,
	}
}

func paginationParams(r *http.Request) (int, int, bool, error) {
	query := r.URL.Query()
	hasPage := query.Has("page") || query.Has("pageSize")
	hasLimit := query.Has("limit") || query.Has("offset")
	if !hasPage && !hasLimit {
		return 0, 0, false, nil
	}
	if hasLimit && !hasPage {
		limit := intQuery(query.Get("limit"), 20)
		offset := intQuery(query.Get("offset"), 0)
		if limit <= 0 || limit > 200 || offset < 0 {
			return 0, 0, true, fmt.Errorf("%w: invalid limit or offset", errBadRequest)
		}
		return offset/limit + 1, limit, true, nil
	}
	page := intQuery(query.Get("page"), 1)
	pageSize := intQuery(query.Get("pageSize"), 20)
	if page <= 0 || pageSize <= 0 || pageSize > 200 {
		return 0, 0, true, fmt.Errorf("%w: invalid page or pageSize", errBadRequest)
	}
	return page, pageSize, true, nil
}

func intQuery(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func decode(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("%w: empty body", errBadRequest)
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("%w: %v", errBadRequest, err)
	}
	return nil
}

func pathParts(path, prefix string) []string {
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	for i := range parts {
		parts[i], _ = url.PathUnescape(parts[i])
	}
	return parts
}

func createOrList[T any](w http.ResponseWriter, r *http.Request, items *[]T, prepare func(*T)) error {
	if r.Method == http.MethodPost {
		var item T
		if err := decode(r, &item); err != nil {
			return err
		}
		prepare(&item)
		*items = append([]T{item}, (*items)...)
		return writeJSON(w, item)
	}
	return writePaginatedOrList(w, r, *items)
}

func filter[T any](rows []T, keep func(T) bool) []T {
	result := make([]T, 0, len(rows))
	for _, row := range rows {
		if keep(row) {
			result = append(result, row)
		}
	}
	return result
}

func match(keyword, text string) bool {
	return keyword == "" || strings.Contains(strings.ToLower(text), strings.ToLower(keyword))
}

func matchOption(value, all, text string) bool {
	return value == "" || value == all || strings.Contains(text, value)
}

func contains[T comparable](rows []T, value T) bool {
	for _, row := range rows {
		if row == value {
			return true
		}
	}
	return false
}

func appendUnique(rows []string, values ...string) []string {
	seen := map[string]bool{}
	for _, row := range rows {
		seen[row] = true
	}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		rows = append(rows, value)
		seen[value] = true
	}
	return rows
}

func prependAudit(rows []AuditEvent, event AuditEvent) []AuditEvent {
	return append([]AuditEvent{event}, rows...)
}

func audit(timeText, action, result, operator, people, detail string) AuditEvent {
	return AuditEvent{ID: newID("ev"), Time: timeText, Action: action, Result: result, Operator: operator, People: people, Detail: detail}
}

func makeQRDownload(name, kind string) QRDownload {
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="360" height="420" viewBox="0 0 360 420"><rect width="360" height="420" fill="#fff"/><text x="180" y="34" text-anchor="middle" font-size="20" font-family="Arial" fill="#1f2937">%s</text><rect x="70" y="60" width="220" height="220" fill="#f8fafc" stroke="#1f2937" stroke-width="6"/><g fill="#111827"><rect x="90" y="80" width="48" height="48"/><rect x="222" y="80" width="48" height="48"/><rect x="90" y="212" width="48" height="48"/><rect x="154" y="94" width="16" height="16"/><rect x="184" y="94" width="22" height="22"/><rect x="150" y="138" width="26" height="26"/><rect x="194" y="142" width="18" height="18"/><rect x="226" y="154" width="22" height="22"/></g><text x="180" y="330" text-anchor="middle" font-size="16" font-family="Arial" fill="#475467">线下门店私域 %s</text></svg>`, name, kind)
	return QRDownload{FileName: name + "-" + kind + ".svg", MimeType: "image/svg+xml", Content: svg}
}

var businessLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

func setBusinessLocation(location *time.Location) {
	if location != nil {
		businessLocation = location
	}
}

func businessNow() time.Time {
	return time.Now().In(businessLocation)
}

func formatBusinessTime(value time.Time, layout string) string {
	if value.IsZero() {
		return ""
	}
	return value.In(businessLocation).Format(layout)
}

func stamp() string {
	return businessNow().Format("2006-01-02 15:04")
}

func today() string {
	return businessNow().Format("2006-01-02")
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func envString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("invalid %s=%q, using %d\n", key, value, fallback)
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		log.Printf("invalid %s=%q, using %t\n", key, value, fallback)
		return fallback
	}
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err == nil {
		return duration
	}
	seconds, err := strconv.Atoi(value)
	if err == nil {
		return time.Duration(seconds) * time.Second
	}
	log.Printf("invalid %s=%q, using %s\n", key, value, fallback)
	return fallback
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func writeTooManyRequests(w http.ResponseWriter, requestID string) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":     "rate limit exceeded",
		"requestId": requestID,
	})
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func newID(prefix string) string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return prefix + hex.EncodeToString(b[:])
}
