package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newOpsTestServer() (*API, http.Handler) {
	api := newAPI()
	mux := http.NewServeMux()
	api.register(mux)
	return api, mux
}

func opsRequest(t *testing.T, handler http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
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

func decodeOpsResponse[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var payload T
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}
	return payload
}

func TestOpsCustomerStageWritesTimelineAndLog(t *testing.T) {
	api, handler := newOpsTestServer()
	_ = api

	opsRequest(t, handler, http.MethodPatch, "/api/customers/oc3/stage", `{"stage":"following","reason":"测试流转","operatorName":"测试运营"}`, nil)
	detail := decodeOpsResponse[map[string]any](t, opsRequest(t, handler, http.MethodGet, "/api/customers/oc3", "", nil))
	customer := detail["customer"].(map[string]any)
	if got := customer["lifecycleStage"]; got != "following" {
		t.Fatalf("lifecycleStage=%v want following", got)
	}

	timeline := detail["timeline"].([]any)
	if len(timeline) == 0 || !strings.Contains(timeline[0].(map[string]any)["title"].(string), "生命周期") {
		t.Fatalf("latest timeline should be lifecycle change, got %#v", timeline)
	}
	if len(api.opsLifecycleLogs) == 0 || api.opsLifecycleLogs[0].StageAfter != "following" {
		t.Fatalf("lifecycle log not written: %#v", api.opsLifecycleLogs)
	}
}

func TestOpsCustomerTagAndFollowup(t *testing.T) {
	_, handler := newOpsTestServer()

	detail := decodeOpsResponse[map[string]any](t, opsRequest(t, handler, http.MethodPost, "/api/customers/oc3/tags", `{"name":"复购潜力","category":"手动标签","source":"manual"}`, nil))
	customer := detail["customer"].(map[string]any)
	tags := customer["tags"].([]any)
	foundTag := false
	for _, tag := range tags {
		foundTag = foundTag || tag == "复购潜力"
	}
	if !foundTag {
		t.Fatalf("tag not added, got %#v", tags)
	}

	opsRequest(t, handler, http.MethodPost, "/api/customers/oc3/follow-ups", `{"followUpType":"首次沟通","content":"已完成首触","result":"需继续跟进","nextFollowUpTime":"2026-07-07 10:00","stageAfter":"following","createdBy":"王敏"}`, nil)
	after := decodeOpsResponse[map[string]any](t, opsRequest(t, handler, http.MethodGet, "/api/customers/oc3", "", nil))
	afterCustomer := after["customer"].(map[string]any)
	if afterCustomer["lastFollowUpTime"] == "" {
		t.Fatalf("followup should update lastFollowUpTime")
	}
	if got := afterCustomer["lifecycleStage"]; got != "following" {
		t.Fatalf("followup stage=%v want following", got)
	}
	if len(after["followups"].([]any)) == 0 {
		t.Fatalf("followup record missing")
	}
	if len(after["tasks"].([]any)) < 2 {
		t.Fatalf("next followup should create reminder task, got %#v", after["tasks"])
	}
}

func TestOpsTaskCompleteResolvesOverdueException(t *testing.T) {
	_, handler := newOpsTestServer()

	opsRequest(t, handler, http.MethodPost, "/api/sop-tasks/ot1/complete", `{"remark":"已经完成首次跟进"}`, nil)
	task := decodeOpsResponse[OpsTask](t, opsRequest(t, handler, http.MethodGet, "/api/sop-tasks/ot1", "", nil))
	if task.Status != "completed" {
		t.Fatalf("task status=%q want completed", task.Status)
	}

	exceptions := decodeOpsResponse[[]OpsException](t, opsRequest(t, handler, http.MethodGet, "/api/operation-exceptions", "", nil))
	for _, item := range exceptions {
		if item.TaskID == "ot1" && item.Status != "resolved" {
			t.Fatalf("ot1 exception should be resolved, got %#v", item)
		}
	}
}

func TestOpsBootstrapScopesByRole(t *testing.T) {
	_, handler := newOpsTestServer()
	headers := map[string]string{"X-SCRM-Role": "guide", "X-SCRM-Guide-ID": "g1"}

	payload := decodeOpsResponse[OpsBootstrap](t, opsRequest(t, handler, http.MethodGet, "/api/customer-ops/bootstrap", "", headers))
	if len(payload.Customers) != 1 {
		t.Fatalf("guide g1 customers=%d want 1", len(payload.Customers))
	}
	if payload.Customers[0].OwnerGuideID != "g1" {
		t.Fatalf("guide scope leaked customer: %#v", payload.Customers)
	}
	for _, task := range payload.Tasks {
		if task.AssignedToUserID != "g1" {
			t.Fatalf("guide scope leaked task: %#v", task)
		}
	}
}

func TestOpsBootstrapIncludesOverdueExceptions(t *testing.T) {
	_, handler := newOpsTestServer()

	payload := decodeOpsResponse[OpsBootstrap](t, opsRequest(t, handler, http.MethodGet, "/api/customer-ops/bootstrap", "", nil))
	found := false
	for _, item := range payload.Exceptions {
		found = found || item.ExceptionType == "task_overdue" && item.TaskID == "ot1" && item.Status == "pending"
	}
	if !found {
		t.Fatalf("expected pending overdue exception for ot1, got %#v", payload.Exceptions)
	}
}
