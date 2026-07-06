package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type monobasePage struct {
	Items []json.RawMessage `json:"items"`
	Total int               `json:"total"`
	Skip  int               `json:"skip"`
	Limit int               `json:"limit"`
}

type monobaseEnvelope struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    monobasePage `json:"data"`
}

type monobaseDepartment struct {
	SCRMDepartmentID       any             `json:"scrm_department_id"`
	SCRMAccountID          any             `json:"scrm_account_id"`
	SCRMDepartmentName     string          `json:"scrm_department_name"`
	SCRMDepartmentParentID any             `json:"scrm_department_parent_id"`
	SCRMDepartmentSort     any             `json:"scrm_department_sort"`
	WeComDepartmentID      any             `json:"wecom_department_id"`
	SCRMSyncedAt           string          `json:"scrm_synced_at"`
	SCRMUpdatedAt          string          `json:"scrm_updated_at"`
	SCRMDeletedAt          *string         `json:"scrm_deleted_at"`
	Raw                    json.RawMessage `json:"-"`
}

type monobaseTag struct {
	SCRMTagID        any             `json:"scrm_tag_id"`
	SCRMTagName      string          `json:"scrm_tag_name"`
	SCRMTagColor     string          `json:"scrm_tag_color"`
	SCRMTagGroupName string          `json:"scrm_tag_group_name"`
	WeComTagID       *string         `json:"wecom_tag_id"`
	SCRMUpdatedAt    string          `json:"scrm_updated_at"`
	SCRMDeletedAt    *string         `json:"scrm_deleted_at"`
	Raw              json.RawMessage `json:"-"`
}

type monobaseCustomer struct {
	SCRMCustomerID      any             `json:"scrm_customer_id"`
	SCRMCustomerName    string          `json:"scrm_customer_name"`
	SCRMCustomerAvatar  string          `json:"scrm_customer_avatar"`
	SCRMCustomerType    string          `json:"scrm_customer_type"`
	SCRMCustomerSource  string          `json:"scrm_customer_source"`
	SCRMCustomerTags    []string        `json:"scrm_customer_tags"`
	SCRMCustomerAddTime string          `json:"scrm_customer_add_time"`
	SCRMCustomerRemark  string          `json:"scrm_customer_remark"`
	SCRMAccountID       any             `json:"scrm_account_id"`
	SCRMAccountName     string          `json:"scrm_account_name"`
	WeComExternalUserID string          `json:"wecom_external_user_id"`
	SCRMUpdatedAt       string          `json:"scrm_updated_at"`
	SCRMDeletedAt       *string         `json:"scrm_deleted_at"`
	Raw                 json.RawMessage `json:"-"`
}

type monobaseSyncStats struct {
	Resource string `json:"resource"`
	Total    int    `json:"total"`
	Added    int    `json:"added"`
	Updated  int    `json:"updated"`
	Skipped  int    `json:"skipped"`
	Failed   int    `json:"failed"`
}

func (api *API) monobaseStatusHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	if api.db == nil {
		return fmt.Errorf("%w: monobase sync requires postgres mode", errBadRequest)
	}
	if strings.TrimSpace(api.config.MonobaseBaseURL) == "" || strings.TrimSpace(api.config.MonobaseToken) == "" {
		return writeJSON(w, map[string]any{"configured": false, "states": []any{}})
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	rows, err := api.db.QueryContext(ctx, `
		SELECT resource,
			COALESCE(to_char(last_success_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			COALESCE(to_char(last_started_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			last_error, last_total, last_added, last_updated
		FROM monobase_sync_state
		ORDER BY resource
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	states := []map[string]any{}
	for rows.Next() {
		row := map[string]any{}
		var resource, successAt, startedAt, lastError string
		var total, added, updated int
		if err := rows.Scan(&resource, &successAt, &startedAt, &lastError, &total, &added, &updated); err != nil {
			return err
		}
		row["resource"] = resource
		row["lastSuccessAt"] = successAt
		row["lastStartedAt"] = startedAt
		row["lastError"] = lastError
		row["lastTotal"] = total
		row["lastAdded"] = added
		row["lastUpdated"] = updated
		states = append(states, row)
	}
	return writeJSON(w, map[string]any{
		"configured":    true,
		"baseUrl":       strings.TrimRight(api.config.MonobaseBaseURL, "/"),
		"corpAccountId": api.config.MonobaseCorpID,
		"syncPageSize":  api.monobasePageSize(),
		"states":        states,
	})
}

func (api *API) monobaseSyncHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	if api.db == nil {
		return fmt.Errorf("%w: monobase sync requires postgres mode", errBadRequest)
	}
	if strings.TrimSpace(api.config.MonobaseBaseURL) == "" || strings.TrimSpace(api.config.MonobaseToken) == "" {
		return fmt.Errorf("%w: MONOBASE_BASE_URL and MONOBASE_INTEGRATION_TOKEN are required", errBadRequest)
	}
	var payload struct {
		Resources []string `json:"resources"`
		FullSync  bool     `json:"fullSync"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	resources := normalizeMonobaseResources(payload.Resources)
	result := map[string]monobaseSyncStats{}
	for _, resource := range resources {
		stats, err := api.syncMonobaseResource(r.Context(), resource, payload.FullSync)
		result[resource] = stats
		if err != nil {
			_ = api.saveMonobaseSyncState(context.Background(), resource, stats, err)
			return fmt.Errorf("%w: sync %s failed: %v", errBadRequest, resource, err)
		}
		_ = api.saveMonobaseSyncState(context.Background(), resource, stats, nil)
	}
	api.invalidateCache("summary:", "customers:", "tags:", "tag-groups:", "tag-customers:")
	return writeJSON(w, map[string]any{"status": "ok", "resources": result})
}

func normalizeMonobaseResources(resources []string) []string {
	if len(resources) == 0 {
		return []string{"departments", "tags", "customers"}
	}
	allowed := map[string]bool{"departments": true, "tags": true, "customers": true}
	out := []string{}
	seen := map[string]bool{}
	for _, item := range resources {
		item = strings.TrimSpace(strings.ToLower(item))
		if allowed[item] && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return []string{"departments", "tags", "customers"}
	}
	return out
}

func (api *API) syncMonobaseResource(ctx context.Context, resource string, fullSync bool) (monobaseSyncStats, error) {
	stats := monobaseSyncStats{Resource: resource}
	if err := api.markMonobaseSyncStarted(ctx, resource); err != nil {
		return stats, err
	}
	pageSize := api.monobasePageSize()
	for skip := 0; ; skip += pageSize {
		page, err := api.fetchMonobasePage(ctx, resource, skip, pageSize, fullSync)
		if err != nil {
			return stats, err
		}
		for _, raw := range page.Items {
			changed, err := api.importMonobaseItem(ctx, resource, raw)
			if err != nil {
				stats.Failed++
				log.Printf("monobase import item failed resource=%s err=%v payload=%s\n", resource, err, string(raw))
				continue
			}
			stats.Total++
			switch changed {
			case "added":
				stats.Added++
			case "updated":
				stats.Updated++
			default:
				stats.Skipped++
			}
		}
		if len(page.Items) == 0 || skip+len(page.Items) >= page.Total || len(page.Items) < pageSize {
			break
		}
	}
	return stats, nil
}

func (api *API) fetchMonobasePage(ctx context.Context, resource string, skip, limit int, fullSync bool) (monobasePage, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(api.config.MonobaseBaseURL), "/")
	endpoint, err := url.Parse(baseURL + "/api/integration/scrm/" + resource)
	if err != nil {
		return monobasePage{}, err
	}
	q := endpoint.Query()
	q.Set("skip", strconv.Itoa(skip))
	q.Set("limit", strconv.Itoa(limit))
	if api.config.MonobaseCorpID != "" {
		q.Set("corp_account_id", api.config.MonobaseCorpID)
	}
	if !fullSync {
		if since, ok := api.monobaseLastSuccess(ctx, resource); ok {
			q.Set("updated_since", since.Format(time.RFC3339))
			q.Set("include_deleted", "true")
		}
	}
	endpoint.RawQuery = q.Encode()

	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return monobasePage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(api.config.MonobaseToken))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return monobasePage{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return monobasePage{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return monobasePage{}, fmt.Errorf("monobase HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var envelope monobaseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return monobasePage{}, err
	}
	if envelope.Code != 0 {
		return monobasePage{}, fmt.Errorf("monobase code=%d message=%s", envelope.Code, envelope.Message)
	}
	return envelope.Data, nil
}

func (api *API) importMonobaseItem(ctx context.Context, resource string, raw json.RawMessage) (string, error) {
	switch resource {
	case "departments":
		var item monobaseDepartment
		if err := json.Unmarshal(raw, &item); err != nil {
			return "", err
		}
		item.Raw = raw
		return api.importMonobaseDepartment(ctx, item)
	case "tags":
		var item monobaseTag
		if err := json.Unmarshal(raw, &item); err != nil {
			return "", err
		}
		item.Raw = raw
		return api.importMonobaseTag(ctx, item)
	case "customers":
		var item monobaseCustomer
		if err := json.Unmarshal(raw, &item); err != nil {
			return "", err
		}
		item.Raw = raw
		return api.importMonobaseCustomer(ctx, item)
	default:
		return "", fmt.Errorf("unsupported resource %s", resource)
	}
}

func (api *API) importMonobaseDepartment(ctx context.Context, item monobaseDepartment) (string, error) {
	id := monobaseString(item.SCRMDepartmentID)
	if id == "" {
		return "skipped", nil
	}
	existed := monobaseRowExists(ctx, api.db, `SELECT 1 FROM monobase_scrm_departments WHERE scrm_department_id = $1`, id)
	raw := string(item.Raw)
	_, err := api.db.ExecContext(ctx, `
		INSERT INTO monobase_scrm_departments (
			scrm_department_id, scrm_account_id, scrm_department_name, scrm_department_parent_id,
			wecom_department_id, scrm_department_sort, scrm_synced_at, scrm_updated_at, scrm_deleted_at, raw_payload
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)
		ON CONFLICT (scrm_department_id) DO UPDATE SET
			scrm_account_id = EXCLUDED.scrm_account_id,
			scrm_department_name = EXCLUDED.scrm_department_name,
			scrm_department_parent_id = EXCLUDED.scrm_department_parent_id,
			wecom_department_id = EXCLUDED.wecom_department_id,
			scrm_department_sort = EXCLUDED.scrm_department_sort,
			scrm_synced_at = EXCLUDED.scrm_synced_at,
			scrm_updated_at = EXCLUDED.scrm_updated_at,
			scrm_deleted_at = EXCLUDED.scrm_deleted_at,
			raw_payload = EXCLUDED.raw_payload,
			updated_at = now()
	`, id, monobaseString(item.SCRMAccountID), item.SCRMDepartmentName, monobaseString(item.SCRMDepartmentParentID), monobaseString(item.WeComDepartmentID), monobaseInt(item.SCRMDepartmentSort), nullTime(item.SCRMSyncedAt), nullTime(item.SCRMUpdatedAt), nullableStringTime(item.SCRMDeletedAt), raw)
	if err != nil {
		return "", err
	}
	return monobaseChangeLabel(existed), nil
}

func (api *API) importMonobaseTag(ctx context.Context, item monobaseTag) (string, error) {
	name := strings.TrimSpace(item.SCRMTagName)
	if name == "" || item.SCRMDeletedAt != nil {
		return "skipped", nil
	}
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	groupID, err := ensureDBTagGroup(ctx, tx, firstNonEmpty(item.SCRMTagGroupName, "客户状态"), "monobase-v2", 0)
	if err != nil {
		return "", err
	}
	tagID := "mbtag-" + monobaseString(item.SCRMTagID)
	existed := monobaseRowExists(ctx, tx, `SELECT 1 FROM tags WHERE tag_group_id = $1 AND name = $2`, groupID, name)
	wecomTagID := "monobase:" + monobaseString(item.SCRMTagID)
	if item.WeComTagID != nil && strings.TrimSpace(*item.WeComTagID) != "" {
		wecomTagID = strings.TrimSpace(*item.WeComTagID)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tags (id, tag_group_id, name, status, wecom_tag_id)
		VALUES ($1,$2,$3,'active',$4)
		ON CONFLICT (tag_group_id, name) DO UPDATE SET
			status = 'active',
			wecom_tag_id = EXCLUDED.wecom_tag_id,
			updated_at = now()
	`, tagID, groupID, name, wecomTagID)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return monobaseChangeLabel(existed), nil
}

func (api *API) importMonobaseCustomer(ctx context.Context, item monobaseCustomer) (string, error) {
	externalID := strings.TrimSpace(item.WeComExternalUserID)
	name := strings.TrimSpace(item.SCRMCustomerName)
	if externalID == "" || name == "" || item.SCRMDeletedAt != nil {
		return "skipped", nil
	}
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	customerID := "mbc-" + monobaseString(item.SCRMCustomerID)
	var existingID string
	existed := false
	err = tx.QueryRowContext(ctx, `SELECT customer_id FROM customer_identities WHERE identity_type = 'wecom_external_user_id' AND identity_value = $1`, externalID).Scan(&existingID)
	if err == nil && existingID != "" {
		customerID = existingID
		existed = true
	} else if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	if !existed {
		existed = monobaseRowExists(ctx, tx, `SELECT 1 FROM customers WHERE id = $1`, customerID)
	}

	stage := "企微客户"
	if strings.Contains(item.SCRMCustomerType, "corp") {
		stage = "企业客户"
	}
	sourceName := firstNonEmpty(item.SCRMAccountName, "monobase-v2")
	_, err = tx.ExecContext(ctx, `
		INSERT INTO customers (
			id, name, wecom_name, mobile_masked, lifecycle_stage, sales_stage,
			owner_staff_name, source_store_name, intent_score, updated_at, created_at
		) VALUES ($1,$2,$3,'',$4,'待识别','企微同步',$5,0,now(),COALESCE($6, now()))
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			wecom_name = EXCLUDED.wecom_name,
			lifecycle_stage = EXCLUDED.lifecycle_stage,
			owner_staff_name = EXCLUDED.owner_staff_name,
			source_store_name = EXCLUDED.source_store_name,
			updated_at = now()
	`, customerID, name, firstNonEmpty(item.SCRMCustomerRemark, externalID), stage, sourceName, nullTime(item.SCRMCustomerAddTime))
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO customer_identities (id, customer_id, identity_type, identity_value, is_primary)
		VALUES ($1,$2,'wecom_external_user_id',$3,true)
		ON CONFLICT (identity_type, identity_value) DO UPDATE SET
			customer_id = EXCLUDED.customer_id,
			is_primary = true,
			merged_at = now()
	`, newID("cid"), customerID, externalID); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO customer_attributions (
			customer_id, source_store_id, source_brand_id, first_staff_id, source_code_id,
			entry_mode, entry_at, current_service_store_id, main_follow_staff_id
		) VALUES ($1,'monobase-v2','','','monobase-integration',$2,COALESCE($3, now()),'monobase-v2','')
		ON CONFLICT (customer_id) DO UPDATE SET
			entry_mode = EXCLUDED.entry_mode,
			current_service_store_id = EXCLUDED.current_service_store_id
	`, customerID, firstNonEmpty(item.SCRMCustomerSource, "wecom_external"), nullTime(item.SCRMCustomerAddTime)); err != nil {
		return "", err
	}
	for _, tagName := range item.SCRMCustomerTags {
		tagName = strings.TrimSpace(tagName)
		if tagName == "" {
			continue
		}
		groupID, err := ensureDBTagGroup(ctx, tx, "monobase客户标签", "monobase-v2", 99)
		if err != nil {
			return "", err
		}
		var tagID string
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO tags (id, tag_group_id, name, status, wecom_tag_id)
			VALUES ($1,$2,$3,'active','')
			ON CONFLICT (tag_group_id, name) DO UPDATE SET status='active', updated_at=now()
			RETURNING id
		`, newID("tag"), groupID, tagName).Scan(&tagID); err != nil {
			return "", err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO customer_tags (customer_id, tag_id, source)
			VALUES ($1,$2,'monobase')
			ON CONFLICT (customer_id, tag_id) DO UPDATE SET source = EXCLUDED.source
		`, customerID, tagID); err != nil {
			return "", err
		}
	}
	if !existed {
		payload := map[string]any{
			"monobaseCustomerId":  monobaseString(item.SCRMCustomerID),
			"wecomExternalUserId": externalID,
			"source":              item.SCRMCustomerSource,
		}
		if err := insertCustomerEventDB(ctx, tx, customerID, "monobase_sync", "monobase", monobaseString(item.SCRMCustomerID), payload); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return monobaseChangeLabel(existed), nil
}

func (api *API) monobasePageSize() int {
	if api.config.MonobasePageSize <= 0 || api.config.MonobasePageSize > 200 {
		return 100
	}
	return api.config.MonobasePageSize
}

func (api *API) markMonobaseSyncStarted(ctx context.Context, resource string) error {
	_, err := api.db.ExecContext(ctx, `
		INSERT INTO monobase_sync_state (resource, last_started_at, last_error, updated_at)
		VALUES ($1, now(), '', now())
		ON CONFLICT (resource) DO UPDATE SET last_started_at = now(), last_error = '', updated_at = now()
	`, resource)
	return err
}

func (api *API) saveMonobaseSyncState(ctx context.Context, resource string, stats monobaseSyncStats, syncErr error) error {
	lastError := ""
	if syncErr != nil {
		lastError = syncErr.Error()
	}
	_, err := api.db.ExecContext(ctx, `
		INSERT INTO monobase_sync_state (resource, last_success_at, last_started_at, last_error, last_total, last_added, last_updated, updated_at)
		VALUES ($1, CASE WHEN $2 = '' THEN now() ELSE NULL END, now(), $2, $3, $4, $5, now())
		ON CONFLICT (resource) DO UPDATE SET
			last_success_at = CASE WHEN EXCLUDED.last_error = '' THEN now() ELSE monobase_sync_state.last_success_at END,
			last_error = EXCLUDED.last_error,
			last_total = EXCLUDED.last_total,
			last_added = EXCLUDED.last_added,
			last_updated = EXCLUDED.last_updated,
			updated_at = now()
	`, resource, lastError, stats.Total, stats.Added, stats.Updated)
	return err
}

func (api *API) monobaseLastSuccess(ctx context.Context, resource string) (time.Time, bool) {
	var t time.Time
	if err := api.db.QueryRowContext(ctx, `SELECT last_success_at FROM monobase_sync_state WHERE resource = $1 AND last_success_at IS NOT NULL`, resource).Scan(&t); err != nil {
		return time.Time{}, false
	}
	return t, true
}

func monobaseString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case json.Number:
		return typed.String()
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func monobaseInt(value any) int {
	n, _ := strconv.Atoi(monobaseString(value))
	return n
}

func nullTime(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	return nil
}

func nullableStringTime(raw *string) any {
	if raw == nil {
		return nil
	}
	return nullTime(*raw)
}

type monobaseQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func monobaseRowExists(ctx context.Context, q monobaseQueryer, query string, args ...any) bool {
	var marker int
	return q.QueryRowContext(ctx, query, args...).Scan(&marker) == nil
}

func monobaseChangeLabel(existed bool) string {
	if existed {
		return "updated"
	}
	return "added"
}
