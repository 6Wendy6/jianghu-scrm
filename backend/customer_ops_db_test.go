package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func newOpsDBTestServer(t *testing.T) (*API, http.Handler) {
	t.Helper()
	dsn := os.Getenv("SCRM_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SCRM_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", postgresURLWithTimeZone(dsn, "Asia/Shanghai"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
	if err := runMigrations(ctx, db, "migrations"); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if err := runSQLFile(ctx, db, "seeds/customer_ops_demo_seed.sql"); err != nil {
		t.Fatalf("run customer ops seed: %v", err)
	}
	api := newAPI()
	api.db = db
	api.storageMode = "postgres"
	mux := http.NewServeMux()
	api.register(mux)
	return api, mux
}

func opsDBRequest(t *testing.T, handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code < 200 || rr.Code >= 300 {
		t.Fatalf("%s %s status=%d body=%s", method, path, rr.Code, rr.Body.String())
	}
	return rr
}

func opsDBDecode[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var payload T
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}
	return payload
}

func TestOpsDBTagIsPersistentAndUnique(t *testing.T) {
	api, handler := newOpsDBTestServer(t)
	tagName := "DB唯一标签" + newID("case")
	body := `{"name":"` + tagName + `","category":"手动标签","source":"manual"}`
	opsDBRequest(t, handler, http.MethodPost, "/api/customers/oc1/tags", body, nil)
	opsDBRequest(t, handler, http.MethodPost, "/api/customers/oc1/tags", body, nil)

	var count int
	err := api.db.QueryRow(`
		SELECT count(*)
		FROM customer_tags ct
		JOIN tags t ON t.id = ct.tag_id
		WHERE ct.customer_id = 'oc1' AND t.name = $1
	`, tagName).Scan(&count)
	if err != nil {
		t.Fatalf("count tag relation: %v", err)
	}
	if count != 1 {
		t.Fatalf("tag relation count=%d want 1", count)
	}
}

func TestOpsDBStageWritesLifecycleLogAndTimeline(t *testing.T) {
	api, handler := newOpsDBTestServer(t)
	stage := "following"
	opsDBRequest(t, handler, http.MethodPatch, "/api/customers/oc3/stage", `{"stage":"`+stage+`","reason":"DB测试流转"}`, nil)

	var logCount, timelineCount int
	if err := api.db.QueryRow(`SELECT count(*) FROM customer_lifecycle_logs WHERE customer_id = 'oc3' AND stage_after = $1`, stage).Scan(&logCount); err != nil {
		t.Fatalf("count lifecycle logs: %v", err)
	}
	if err := api.db.QueryRow(`SELECT count(*) FROM customer_operation_timeline WHERE customer_id = 'oc3' AND event_type = 'lifecycle' AND content LIKE '%DB测试流转%'`).Scan(&timelineCount); err != nil {
		t.Fatalf("count timeline: %v", err)
	}
	if logCount == 0 || timelineCount == 0 {
		t.Fatalf("stage change did not write log/timeline: logs=%d timeline=%d", logCount, timelineCount)
	}
}

func TestOpsDBFollowupUpdatesLastFollowAndStage(t *testing.T) {
	api, handler := newOpsDBTestServer(t)
	opsDBRequest(t, handler, http.MethodPost, "/api/customers/oc3/follow-ups", `{"followUpType":"首次沟通","content":"DB跟进","result":"需继续跟进","stageAfter":"high_intent","nextFollowUpTime":"2026-07-09 10:00"}`, nil)

	var stage string
	var hasLastFollow bool
	if err := api.db.QueryRow(`SELECT lifecycle_stage, last_follow_up_time IS NOT NULL FROM customers WHERE id = 'oc3'`).Scan(&stage, &hasLastFollow); err != nil {
		t.Fatalf("read customer: %v", err)
	}
	if stage != "high_intent" || !hasLastFollow {
		t.Fatalf("customer stage=%q hasLastFollow=%t", stage, hasLastFollow)
	}
}

func TestOpsDBTaskCompleteWritesLogsTimelineAndResolvesException(t *testing.T) {
	api, handler := newOpsDBTestServer(t)
	opsDBRequest(t, handler, http.MethodPost, "/api/sop-tasks/ot1/complete", `{"remark":"DB完成任务"}`, nil)

	var taskLogs, timelines, openExceptions int
	_ = api.db.QueryRow(`SELECT count(*) FROM sop_task_logs WHERE task_id = 'ot1' AND new_status = 'completed'`).Scan(&taskLogs)
	_ = api.db.QueryRow(`SELECT count(*) FROM customer_operation_timeline WHERE related_id = 'ot1' AND event_type = 'task'`).Scan(&timelines)
	_ = api.db.QueryRow(`SELECT count(*) FROM operation_exceptions WHERE task_id = 'ot1' AND status NOT IN ('resolved', 'ignored')`).Scan(&openExceptions)
	if taskLogs == 0 || timelines == 0 || openExceptions != 0 {
		t.Fatalf("task complete side effects logs=%d timelines=%d openExceptions=%d", taskLogs, timelines, openExceptions)
	}
}

func TestOpsDBScopeAndForbidden(t *testing.T) {
	_, handler := newOpsDBTestServer(t)
	storeHeaders := map[string]string{"X-SCRM-Role": "store_manager", "X-SCRM-Store-ID": "s1"}
	storePayload := opsDBDecode[OpsBootstrap](t, opsDBRequest(t, handler, http.MethodGet, "/api/customer-ops/bootstrap", "", storeHeaders))
	for _, customer := range storePayload.Customers {
		if customer.StoreID != "s1" {
			t.Fatalf("store scope leaked customer: %#v", customer)
		}
	}

	guideHeaders := map[string]string{"X-SCRM-Role": "guide", "X-SCRM-Guide-ID": "g1"}
	guidePayload := opsDBDecode[OpsBootstrap](t, opsDBRequest(t, handler, http.MethodGet, "/api/customer-ops/bootstrap", "", guideHeaders))
	if len(guidePayload.Customers) == 0 {
		t.Fatalf("guide should see at least one customer")
	}
	for _, customer := range guidePayload.Customers {
		if customer.OwnerGuideID != "g1" {
			t.Fatalf("guide scope leaked customer: %#v", customer)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/customers/oc3", nil)
	req.Header.Set("X-SCRM-Role", "guide")
	req.Header.Set("X-SCRM-Guide-ID", "g1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("unauthorized detail status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "forbidden") {
		t.Fatalf("forbidden response should mention forbidden: %s", rr.Body.String())
	}
}
