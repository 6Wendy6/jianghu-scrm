package main

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"
)

func (api *API) scrmWeComPermissionCheckHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	checks := api.runSCRMWeComPermissionCheck(r.Context(), requestIDFromContext(r.Context()))
	return writeJSON(w, map[string]any{"checks": checks})
}

func (api *API) scrmWeComPermissionChecksHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	checks, err := api.listSCRMWeComPermissionChecks(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, checks)
}

func (api *API) runSCRMWeComPermissionCheck(ctx context.Context, requestID string) []WeComPermissionCheck {
	now := businessNow().Format(time.RFC3339)
	client := api.activeWeComClient()
	checks := []WeComPermissionCheck{}
	add := func(checkType, status string, err error) {
		check := WeComPermissionCheck{ID: newID("swpc"), CheckType: checkType, Status: status, CheckedAt: now}
		if err != nil {
			explained := explainError(err)
			if status != "not_checked" && status != "planned" && status != "unsupported" {
				check.Status = "failed"
			}
			check.ErrCode = explained.ErrCode
			check.ErrMsg = firstNonEmpty(explained.ErrMsg, explained.Message)
			check.LocalCode = explained.Code
			check.Suggestion = explained.Suggestion
			if check.Suggestion == "" {
				check.Suggestion = suggestionForLocalCode(check.LocalCode)
			}
		}
		_ = api.recordSCRMWeComPermissionCheck(ctx, check)
		if check.Status == "failed" {
			_ = api.upsertWeComException(ctx, check, requestID)
		}
		checks = append(checks, check)
	}

	contactToken, err := api.tokenAccessValue(ctx, wecomTokenContact)
	add("contact_token", mapBoolStatus(err == nil), err)

	customerToken, err := api.tokenAccessValue(ctx, wecomTokenCustomer)
	add("customer_token", mapBoolStatus(err == nil), err)

	appToken, err := api.tokenAccessValue(ctx, wecomTokenApp)
	add("app_token", mapBoolStatus(err == nil), err)

	if contactToken != "" {
		err = client.GetDepartmentList(ctx, contactToken)
		add("department_read", mapBoolStatus(err == nil), err)
		err = client.GetUserList(ctx, contactToken, "1")
		add("user_read", mapBoolStatus(err == nil), err)
	} else {
		add("department_read", "not_checked", newCodedError("WECOM_TOKEN_UNAVAILABLE", "通讯录 token 不可用，跳过部门读取检测", errBadRequest, map[string]any{"suggestion": "先修复 contact_token"}))
		add("user_read", "not_checked", newCodedError("WECOM_TOKEN_UNAVAILABLE", "通讯录 token 不可用，跳过成员读取检测", errBadRequest, map[string]any{"suggestion": "先修复 contact_token"}))
	}

	if customerToken != "" {
		err = client.GetFollowUserList(ctx, customerToken)
		add("customer_contact_basic", mapBoolStatus(err == nil), err)
	} else {
		add("customer_contact_basic", "not_checked", newCodedError("WECOM_TOKEN_UNAVAILABLE", "客户联系 token 不可用，跳过客户联系基础权限检测", errBadRequest, map[string]any{"suggestion": "先修复 customer_token"}))
	}

	_ = appToken
	for _, planned := range []string{"customer_group_sync", "contact_way_sync"} {
		check := WeComPermissionCheck{ID: newID("swpc"), CheckType: planned, Status: "planned", LocalCode: "WECOM_CHECK_PLANNED", Suggestion: "后续同步阶段再启用该检测", CheckedAt: now}
		_ = api.recordSCRMWeComPermissionCheck(ctx, check)
		checks = append(checks, check)
	}
	return checks
}

func mapBoolStatus(ok bool) string {
	if ok {
		return "passed"
	}
	return "failed"
}

func (api *API) recordSCRMWeComPermissionCheck(ctx context.Context, check WeComPermissionCheck) error {
	if api.db != nil {
		_, err := api.db.ExecContext(ctx, `
			INSERT INTO wecom_permission_checks (
				id, check_type, status, errcode, errmsg, local_code, suggestion, checked_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,now())
		`, check.ID, check.CheckType, check.Status, check.ErrCode, check.ErrMsg, check.LocalCode, check.Suggestion)
		return err
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	api.wecomChecks = append([]WeComPermissionCheck{check}, api.wecomChecks...)
	return nil
}

func (api *API) listSCRMWeComPermissionChecks(ctx context.Context) ([]WeComPermissionCheck, error) {
	if api.db != nil {
		rows, err := api.db.QueryContext(ctx, `
			SELECT id, check_type, status, errcode, errmsg, local_code, suggestion,
				to_char(checked_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
			FROM wecom_permission_checks
			ORDER BY checked_at DESC, id DESC
			LIMIT 100
		`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		checks := []WeComPermissionCheck{}
		for rows.Next() {
			var check WeComPermissionCheck
			if err := rows.Scan(&check.ID, &check.CheckType, &check.Status, &check.ErrCode, &check.ErrMsg, &check.LocalCode, &check.Suggestion, &check.CheckedAt); err != nil {
				return nil, err
			}
			checks = append(checks, check)
		}
		return checks, rows.Err()
	}
	api.mu.RLock()
	defer api.mu.RUnlock()
	out := append([]WeComPermissionCheck{}, api.wecomChecks...)
	if len(out) > 100 {
		out = out[:100]
	}
	return out, nil
}

func (api *API) latestSCRMWeComPermissionSummary(ctx context.Context) map[string]any {
	checks, err := api.listSCRMWeComPermissionChecks(ctx)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	latest := map[string]WeComPermissionCheck{}
	for _, check := range checks {
		if _, ok := latest[check.CheckType]; !ok {
			latest[check.CheckType] = check
		}
	}
	failed := 0
	for _, check := range latest {
		if check.Status == "failed" {
			failed++
		}
	}
	return map[string]any{"failed": failed, "latest": latest}
}

func (api *API) upsertWeComException(ctx context.Context, check WeComPermissionCheck, requestID string) error {
	exceptionType := exceptionTypeForWeComCheck(check)
	title := "企业微信接入异常：" + check.CheckType
	description := strings.TrimSpace(check.LocalCode + " " + check.ErrMsg)
	if requestID != "" {
		description += " requestId=" + requestID
	}
	suggestion := firstNonEmpty(check.Suggestion, suggestionForLocalCode(check.LocalCode))
	if api.db != nil {
		var existingID string
		err := api.db.QueryRowContext(ctx, `
			SELECT id
			FROM operation_exceptions
			WHERE exception_type = $1 AND status NOT IN ('resolved', 'ignored')
			ORDER BY created_at DESC
			LIMIT 1
		`, exceptionType).Scan(&existingID)
		if err == nil && existingID != "" {
			_, err = api.db.ExecContext(ctx, `
				UPDATE operation_exceptions
				SET title = $2,
					description = $3,
					severity = 'P0',
					suggestion = $4,
					created_at = now()
				WHERE id = $1
			`, existingID, title, description, suggestion)
			return err
		}
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		_, err = api.db.ExecContext(ctx, `
			INSERT INTO operation_exceptions (
				id, exception_type, title, description, severity, status, suggestion
			) VALUES ($1,$2,$3,$4,'P0','pending',$5)
		`, newID("oe"), exceptionType, title, description, suggestion)
		return err
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	for i := range api.opsExceptions {
		if api.opsExceptions[i].ExceptionType == exceptionType && api.opsExceptions[i].Status != "resolved" && api.opsExceptions[i].Status != "ignored" {
			api.opsExceptions[i].Title = title
			api.opsExceptions[i].Description = description
			api.opsExceptions[i].Suggestion = suggestion
			api.opsExceptions[i].CreatedAt = stamp()
			return nil
		}
	}
	api.opsExceptions = append([]OpsException{{
		ID:            newID("oe"),
		ExceptionType: exceptionType,
		Title:         title,
		Description:   description,
		Severity:      "P0",
		Status:        "pending",
		Suggestion:    suggestion,
		CreatedAt:     stamp(),
	}}, api.opsExceptions...)
	return nil
}

func exceptionTypeForWeComCheck(check WeComPermissionCheck) string {
	switch check.LocalCode {
	case "WECOM_IP_NOT_ALLOWED":
		return "wecom_ip_whitelist_error"
	case "WECOM_RATE_LIMITED":
		return "wecom_rate_limit_error"
	case "WECOM_PERMISSION_DENIED":
		return "wecom_permission_error"
	case "WECOM_TOKEN_EXPIRED", "WECOM_INVALID_CREDENTIAL", "WECOM_SECRET_MISSING", "WECOM_TOKEN_UNAVAILABLE":
		return "wecom_token_error"
	default:
		if strings.Contains(check.CheckType, "token") {
			return "wecom_token_error"
		}
		return "wecom_config_error"
	}
}

func suggestionForLocalCode(code string) string {
	switch code {
	case "WECOM_IP_NOT_ALLOWED":
		return "在企业微信后台应用配置中加入当前服务器出口 IP"
	case "WECOM_RATE_LIMITED":
		return "降低检测/同步频率，使用队列和退避重试"
	case "WECOM_PERMISSION_DENIED":
		return "检查应用可见范围、通讯录读取权限和客户联系权限"
	case "WECOM_TOKEN_UNAVAILABLE", "WECOM_TOKEN_EXPIRED", "WECOM_INVALID_CREDENTIAL":
		return "检查 corpId、secret、服务器时间和 token 缓存"
	case "WECOM_CONFIG_MISSING":
		return "先保存企业微信配置"
	default:
		return "查看企业微信错误码并按建议修复配置或权限"
	}
}
