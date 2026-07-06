package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type wecomErrorExplanation struct {
	Code       string `json:"code"`
	ErrCode    int    `json:"errcode"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
}

type wecomTokenStatus struct {
	Cached      bool   `json:"cached"`
	Valid       bool   `json:"valid"`
	ExpiresAt   string `json:"expiresAt,omitempty"`
	SecondsLeft int64  `json:"secondsLeft,omitempty"`
}

type wecomIntegrationStatus struct {
	Status           string                      `json:"status"`
	Mode             string                      `json:"mode"`
	DryRun           bool                        `json:"dryRun"`
	Configured       bool                        `json:"configured"`
	Config           map[string]any              `json:"config"`
	Token            wecomTokenStatus            `json:"token"`
	Tokens           map[string]wecomTokenStatus `json:"tokens"`
	PermissionChecks map[string]any              `json:"permissionChecks"`
	CheckSummary     map[string]any              `json:"checkSummary"`
	Assets           map[string]int              `json:"assets"`
	Retry            map[string]any              `json:"retry"`
	LatestCallback   any                         `json:"latestCallback,omitempty"`
	ErrorDictionary  []wecomErrorExplanation     `json:"errorDictionary"`
	NextSteps        []string                    `json:"nextSteps"`
}

func (api *API) scrmWeComStatusHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	status := api.scrmWeComIntegrationStatus(r.Context(), true)
	return writeJSON(w, status)
}

func (api *API) scrmWeComErrorDictionaryHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	return writeJSON(w, wecomErrorDictionary())
}

func (api *API) scrmWeComSystemSummary(ctx context.Context) map[string]any {
	status := api.scrmWeComIntegrationStatus(ctx, false)
	return map[string]any{
		"status":           status.Status,
		"mode":             status.Mode,
		"dryRun":           status.DryRun,
		"configured":       status.Configured,
		"token":            status.Token,
		"tokens":           status.Tokens,
		"permissionChecks": status.PermissionChecks,
		"assets":           status.Assets,
		"retry":            status.Retry,
	}
}

func (api *API) scrmWeComIntegrationStatus(ctx context.Context, includeDictionary bool) wecomIntegrationStatus {
	status := wecomIntegrationStatus{
		Status:           "attention",
		Mode:             defaultString(api.storageMode, "memory"),
		DryRun:           api.config.WeComContactWayDryRun || envBool("WECOM_CONTACT_WAY_DRY_RUN", false),
		Config:           map[string]any{},
		Token:            wecomTokenStatus{},
		Tokens:           map[string]wecomTokenStatus{},
		PermissionChecks: map[string]any{},
		Assets:           map[string]int{},
		Retry:            map[string]any{"pending": 0, "failed": 0, "resolved": 0},
		NextSteps:        []string{},
	}
	if includeDictionary {
		status.ErrorDictionary = wecomErrorDictionary()
	}
	queryCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()

	if cfgV2, err := api.getSCRMWeComConfig(queryCtx, false); err == nil {
		status.Configured = true
		status.Config = sanitizeSCRMWeComConfigMap(cfgV2)
		status.Tokens = api.latestSCRMWeComTokens(queryCtx)
		status.Token = status.Tokens[wecomTokenContact]
		status.PermissionChecks = api.latestSCRMWeComPermissionSummary(queryCtx)
		if api.db != nil {
			status.Assets = api.wecomAssetCounts(queryCtx, cfgV2.CorpID)
			status.Retry = api.wecomRetrySummary(queryCtx)
			status.LatestCallback = api.latestWeComCallbackSummary(queryCtx)
		}
		status.CheckSummary = map[string]any{"configured": true, "enabled": cfgV2.Enabled, "permissionChecks": status.PermissionChecks}
		status.NextSteps = wecomNextSteps(status)
		status.Status = wecomIntegrationOverall(status)
		return status
	} else if api.db == nil {
		status.Mode = "memory"
		status.Configured = false
		status.Config = map[string]any{"configured": false}
		status.NextSteps = append(status.NextSteps, "保存企业微信配置，或切换 PostgreSQL 模式后做真实接入检查")
		return status
	}

	cfg, err := api.getWeComConfig(queryCtx, false)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			status.Configured = false
			status.Config = map[string]any{"configured": false}
			status.NextSteps = append(status.NextSteps, "保存企业微信 corpId、secret、回调 token 和 EncodingAESKey")
			return status
		}
		status.Config = map[string]any{"error": err.Error()}
		status.NextSteps = append(status.NextSteps, "检查 wecom_corp_config 表和数据库连接")
		return status
	}
	status.Configured = true
	status.Config = sanitizeWeComConfig(cfg)
	status.Token = api.wecomCachedTokenStatus(queryCtx, cfg.CorpID)
	status.Assets = api.wecomAssetCounts(queryCtx, cfg.CorpID)
	status.Retry = api.wecomRetrySummary(queryCtx)
	status.LatestCallback = api.latestWeComCallbackSummary(queryCtx)
	status.CheckSummary = api.wecomCheckSummary(queryCtx, cfg)
	status.NextSteps = wecomNextSteps(status)
	status.Status = wecomIntegrationOverall(status)
	return status
}

func sanitizeWeComConfig(cfg WeComConfig) map[string]any {
	return map[string]any{
		"id":                       cfg.ID,
		"corpId":                   maskMiddle(cfg.CorpID),
		"agentId":                  maskMiddle(cfg.AgentID),
		"callbackUrl":              cfg.CallbackURL,
		"testDepartmentId":         cfg.TestDepartmentID,
		"status":                   cfg.Status,
		"secretConfigured":         cfg.SecretConfigured,
		"tokenConfigured":          cfg.TokenConfigured,
		"encodingAesKeyConfigured": cfg.AESKeyConfigured,
		"createdAt":                cfg.CreatedAt,
		"updatedAt":                cfg.UpdatedAt,
	}
}

func sanitizeSCRMWeComConfigMap(cfg WeComIntegrationConfig) map[string]any {
	return map[string]any{
		"id":                       cfg.ID,
		"corpId":                   cfg.CorpIDMasked,
		"agentId":                  cfg.AgentIDMasked,
		"contactSecretConfigured":  cfg.ContactSecretConfigured,
		"contactSecretMasked":      cfg.ContactSecretMasked,
		"customerSecretConfigured": cfg.CustomerSecretConfigured,
		"customerSecretMasked":     cfg.CustomerSecretMasked,
		"agentSecretConfigured":    cfg.AgentSecretConfigured,
		"agentSecretMasked":        cfg.AgentSecretMasked,
		"callbackTokenConfigured":  cfg.CallbackTokenConfigured,
		"encodingAesKeyConfigured": cfg.EncodingAESKeyConfigured,
		"enabled":                  cfg.Enabled,
		"createdAt":                cfg.CreatedAt,
		"updatedAt":                cfg.UpdatedAt,
	}
}

func (api *API) wecomCachedTokenStatus(ctx context.Context, corpID string) wecomTokenStatus {
	var expiresAt time.Time
	var token string
	err := api.db.QueryRowContext(ctx, `SELECT access_token, expires_at FROM wecom_access_tokens WHERE corp_id = $1`, corpID).Scan(&token, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return wecomTokenStatus{Cached: false, Valid: false}
	}
	if err != nil {
		return wecomTokenStatus{Cached: false, Valid: false}
	}
	secondsLeft := int64(time.Until(expiresAt).Seconds())
	return wecomTokenStatus{
		Cached:      token != "",
		Valid:       token != "" && secondsLeft > 300,
		ExpiresAt:   expiresAt.Format(time.RFC3339),
		SecondsLeft: secondsLeft,
	}
}

func (api *API) wecomAssetCounts(ctx context.Context, corpID string) map[string]int {
	return map[string]int{
		"users":          api.queryIntOrZero(ctx, `SELECT count(*) FROM wecom_users WHERE corp_id = $1`, corpID),
		"contactWays":    api.queryIntOrZero(ctx, `SELECT count(*) FROM wecom_contact_ways WHERE corp_id = $1`, corpID),
		"customerEvents": api.queryIntOrZero(ctx, `SELECT count(*) FROM wecom_customer_events WHERE corp_id = $1`, corpID),
		"storeBindings":  api.queryIntOrZero(ctx, `SELECT count(*) FROM scrm_wecom_contact_way_bindings`),
		"assignments":    api.queryIntOrZero(ctx, `SELECT count(*) FROM scrm_customer_assignments`),
	}
}

func (api *API) wecomRetrySummary(ctx context.Context) map[string]any {
	rows, err := api.db.QueryContext(ctx, `
		SELECT status, count(*)
		FROM scrm_wecom_contact_way_retry_tasks
		GROUP BY status
	`)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	defer rows.Close()
	summary := map[string]any{"pending": 0, "failed": 0, "resolved": 0}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err == nil {
			summary[status] = count
		}
	}
	return summary
}

func (api *API) latestWeComCallbackSummary(ctx context.Context) any {
	var raw []byte
	err := api.db.QueryRowContext(ctx, `
		SELECT jsonb_build_object(
			'id', id,
			'eventKey', event_key,
			'eventType', event_type,
			'externalUserid', external_userid,
			'userId', user_id,
			'state', state,
			'processed', processed,
			'createdAt', to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		)
		FROM scrm_wecom_callback_events
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&raw)
	if err != nil {
		return nil
	}
	return rawJSONMap(raw)
}

func (api *API) wecomCheckSummary(ctx context.Context, cfg WeComConfig) map[string]any {
	checks := []scrmWeComCheck{}
	add := func(name string, ok bool, message string) {
		status := "ok"
		if !ok {
			status = "fail"
		}
		checks = append(checks, scrmWeComCheck{Name: name, Status: status, Message: message})
	}
	add("corp_id", cfg.CorpID != "", configuredMessage(cfg.CorpID != ""))
	add("secret", cfg.SecretConfigured, configuredMessage(cfg.SecretConfigured))
	add("callback_token", cfg.TokenConfigured, configuredMessage(cfg.TokenConfigured))
	add("callback_aes_key", cfg.AESKeyConfigured, configuredMessage(cfg.AESKeyConfigured))
	add("callback_url", cfg.CallbackURL != "", firstNonEmpty(cfg.CallbackURL, "未配置回调地址"))
	tokenStatus := api.wecomCachedTokenStatus(ctx, cfg.CorpID)
	add("cached_access_token", tokenStatus.Valid, map[bool]string{true: "缓存 token 有效", false: "未缓存 token 或 token 即将过期"}[tokenStatus.Valid])
	failed := 0
	for _, check := range checks {
		if check.Status != "ok" {
			failed++
		}
	}
	return map[string]any{"total": len(checks), "failed": failed, "checks": checks}
}

func wecomIntegrationOverall(status wecomIntegrationStatus) string {
	if !status.Configured {
		return "attention"
	}
	if failed, ok := status.CheckSummary["failed"].(int); ok && failed > 0 {
		return "attention"
	}
	if pending, ok := status.Retry["pending"].(int); ok && pending > 0 {
		return "attention"
	}
	if failed, ok := status.PermissionChecks["failed"].(int); ok && failed > 0 {
		return "attention"
	}
	return "ok"
}

func wecomNextSteps(status wecomIntegrationStatus) []string {
	steps := []string{}
	if !status.Configured {
		return []string{"保存企业微信配置", "执行 /api/scrm/wecom/doctor 检查权限和 token"}
	}
	if !status.Token.Valid {
		steps = append(steps, "执行连接测试刷新 access_token，并检查 secret/IP 白名单/应用权限")
	}
	for _, tokenType := range allWeComTokenTypes() {
		if tokenStatus, ok := status.Tokens[tokenType]; ok && !tokenStatus.Valid {
			steps = append(steps, "测试并缓存 "+tokenType)
		}
	}
	if failed, ok := status.PermissionChecks["failed"].(int); ok && failed > 0 {
		steps = append(steps, "处理最近一次企业微信权限检测失败项")
	}
	if status.Assets["users"] == 0 {
		steps = append(steps, "同步或导入企业微信成员，用于门店导购映射")
	}
	if status.Assets["contactWays"] == 0 {
		steps = append(steps, "同步或导入已有企业微信活码资产 config_id")
	}
	if pending, ok := status.Retry["pending"].(int); ok && pending > 0 {
		steps = append(steps, "处理 update_contact_way 失败重试任务")
	}
	if len(steps) == 0 {
		steps = append(steps, "进入部门/成员/客户联系/活码真实同步阶段")
	}
	return steps
}

func maskMiddle(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 6 {
		return value[:1] + "***" + value[len(value)-1:]
	}
	return value[:3] + "***" + value[len(value)-3:]
}

func (api *API) queryIntOrZero(ctx context.Context, query string, args ...any) int {
	var count int
	if err := api.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0
	}
	return count
}

func wecomErrorDictionary() []wecomErrorExplanation {
	items := []wecomErrorExplanation{
		{Code: "WECOM_INVALID_CREDENTIAL", ErrCode: 40001, Message: "企业微信 corpId 或 secret 无效", Suggestion: "检查企业微信后台应用 Secret，确认当前应用属于该企业"},
		{Code: "WECOM_INVALID_CREDENTIAL", ErrCode: 40014, Message: "access_token 无效", Suggestion: "重新获取 access_token，确认没有跨企业或跨应用复用 token"},
		{Code: "WECOM_TOKEN_EXPIRED", ErrCode: 42001, Message: "access_token 已过期", Suggestion: "重新获取 access_token，并检查服务器时间和 token 缓存刷新逻辑"},
		{Code: "WECOM_IP_NOT_ALLOWED", ErrCode: 60020, Message: "服务器 IP 未加入企业微信可信 IP", Suggestion: "在企业微信后台应用配置中加入当前服务器出口 IP"},
		{Code: "WECOM_PERMISSION_DENIED", ErrCode: 60011, Message: "企业微信通讯录或客户联系权限不足", Suggestion: "在企业微信后台开启应用可见范围、通讯录读取或客户联系相关权限"},
		{Code: "WECOM_RATE_LIMITED", ErrCode: 45009, Message: "企业微信接口调用超过频率限制", Suggestion: "降低同步频率，使用任务队列、退避重试和缓存"},
		{Code: "WECOM_CONTACT_WAY_NOT_FOUND", ErrCode: 41042, Message: "企业微信活码 config_id 不存在或不可访问", Suggestion: "确认 config_id 来自当前企业和当前应用可见范围"},
		{Code: "WECOM_CALLBACK_VERIFY_FAILED", ErrCode: -90001, Message: "企业微信回调验签或解密失败", Suggestion: "检查 Token、EncodingAESKey、CorpID、URL 和回调消息体"},
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ErrCode < items[j].ErrCode })
	return items
}

func explainWeComError(errcode int) wecomErrorExplanation {
	for _, item := range wecomErrorDictionary() {
		if item.ErrCode == errcode {
			return item
		}
	}
	return wecomErrorExplanation{
		Code:       fmt.Sprintf("WECOM_ERR_%d", errcode),
		ErrCode:    errcode,
		Message:    "企业微信接口返回未登记错误码",
		Suggestion: "查看企业微信官方错误码文档，并将高频错误补充到本地错误字典",
	}
}
