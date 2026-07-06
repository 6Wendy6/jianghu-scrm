package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type SCRMWeComRetryTask struct {
	ID                string `json:"id"`
	BindingID         string `json:"bindingId"`
	ConfigID          string `json:"configId"`
	TargetGuideUserID string `json:"targetGuideUserid"`
	TargetState       string `json:"targetState"`
	FailedReason      string `json:"failedReason"`
	RetryCount        int    `json:"retryCount"`
	Status            string `json:"status"`
	NextRetryAt       string `json:"nextRetryAt,omitempty"`
	LastAttemptAt     string `json:"lastAttemptAt,omitempty"`
	ResolvedAt        string `json:"resolvedAt,omitempty"`
	CreatedAt         string `json:"createdAt,omitempty"`
	UpdatedAt         string `json:"updatedAt,omitempty"`
}

type scrmWeComCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type scrmValidatedContactWay struct {
	ConfigID    string   `json:"configId"`
	QRCodeURL   string   `json:"qrCodeUrl"`
	State       string   `json:"state"`
	BoundUsers  []string `json:"boundUserids"`
	Party       string   `json:"party"`
	Remark      string   `json:"remark"`
	Source      string   `json:"source"`
	RawResponse any      `json:"rawResponse,omitempty"`
}

type scrmAccessScope struct {
	Role    string
	StoreID string
	UserID  string
}

func scrmScopeFromRequest(r *http.Request) scrmAccessScope {
	role := strings.TrimSpace(r.Header.Get("X-SCRM-Role"))
	if role == "" {
		role = "hq_admin"
	}
	return scrmAccessScope{
		Role:    role,
		StoreID: strings.TrimSpace(r.Header.Get("X-SCRM-Store-ID")),
		UserID:  strings.TrimSpace(r.Header.Get("X-SCRM-UserID")),
	}
}

func (scope scrmAccessScope) bindingWhere() (string, []any) {
	switch scope.Role {
	case "store_manager":
		if scope.StoreID == "" {
			return " WHERE false", nil
		}
		return " WHERE b.store_id = $1", []any{scope.StoreID}
	case "guide":
		if scope.UserID == "" {
			return " WHERE false", nil
		}
		return " WHERE EXISTS (SELECT 1 FROM scrm_store_guides g WHERE g.store_id = b.store_id AND g.guide_userid = $1)", []any{scope.UserID}
	default:
		return "", nil
	}
}

func appendAssignmentScope(conditions []string, args []any, scope scrmAccessScope) ([]string, []any) {
	switch scope.Role {
	case "store_manager":
		if scope.StoreID == "" {
			conditions = append(conditions, "false")
			return conditions, args
		}
		args = append(args, scope.StoreID)
		conditions = append(conditions, "a.store_id = $"+strconv.Itoa(len(args)))
	case "guide":
		if scope.UserID == "" {
			conditions = append(conditions, "false")
			return conditions, args
		}
		args = append(args, scope.UserID)
		conditions = append(conditions, "a.guide_userid = $"+strconv.Itoa(len(args)))
	}
	return conditions, args
}

func (api *API) scrmWeComDoctorHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: wecom doctor requires postgres mode", errBadRequest)
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	checks := []scrmWeComCheck{}
	addCheck := func(name string, ok bool, message string) {
		status := "ok"
		if !ok {
			status = "fail"
		}
		checks = append(checks, scrmWeComCheck{Name: name, Status: status, Message: message})
	}
	cfg, err := api.getWeComConfig(r.Context(), true)
	if err != nil {
		addCheck("wecom_config", false, "未找到企业微信配置，请先保存 corpId/secret/callback 配置")
	} else {
		addCheck("WECOM_CORP_ID", cfg.CorpID != "", configuredMessage(cfg.CorpID != ""))
		addCheck("WECOM_SECRET", cfg.SecretConfigured, configuredMessage(cfg.SecretConfigured))
		addCheck("WECOM_CALLBACK_TOKEN", cfg.TokenConfigured, configuredMessage(cfg.TokenConfigured))
		addCheck("WECOM_CALLBACK_AES_KEY", cfg.AESKeyConfigured, configuredMessage(cfg.AESKeyConfigured))
		addCheck("WECOM_CALLBACK_URL", cfg.CallbackURL != "", firstNonEmpty(cfg.CallbackURL, "未配置回调地址"))
	}
	dryRun := api.config.WeComContactWayDryRun || envBool("WECOM_CONTACT_WAY_DRY_RUN", false)
	addCheck("update_contact_way_mode", true, map[bool]string{true: "dryRun=true，不会修改生产 config_id", false: "dryRun=false，真实 update_contact_way 仅在显式请求 update=1 时执行"}[dryRun])
	if err == nil && cfg.SecretConfigured {
		if token, tokenErr := api.wecomAccessToken(r.Context(), false); tokenErr != nil {
			addCheck("access_token", false, tokenErr.Error())
		} else {
			addCheck("access_token", token != "", "access_token 可获取")
		}
	} else {
		addCheck("access_token", false, "缺少 corpId/secret，无法获取 access_token")
	}
	configID := strings.TrimSpace(r.URL.Query().Get("configId"))
	if configID == "" {
		configID = api.firstLocalWeComContactWayConfigID(r.Context())
	}
	var contactWay any
	if configID == "" {
		addCheck("get_contact_way", false, "未传 configId，且本地没有已同步的企业微信活码资产")
	} else {
		way, validateErr := api.validateSCRMExistingContactWay(r.Context(), configID)
		if validateErr != nil {
			addCheck("get_contact_way", false, validateErr.Error())
		} else {
			contactWay = way
			addCheck("get_contact_way", true, "config_id 存在，二维码和绑定成员可读取")
		}
	}
	updateRequested := r.URL.Query().Get("update") == "1"
	targetGuide := strings.TrimSpace(r.URL.Query().Get("targetGuideUserid"))
	if updateRequested {
		if dryRun {
			addCheck("update_contact_way", true, "dryRun=true，已跳过真实修改")
		} else if configID == "" || targetGuide == "" {
			addCheck("update_contact_way", false, "真实 update 测试需要同时传 configId 和 targetGuideUserid")
		} else {
			state := buildSCRMContactWayState("doctor", "doctor", targetGuide, "doctor")
			if updateErr := api.updateWeComContactWay(r.Context(), configID, "SCRM doctor update test", true, state, []string{targetGuide}); updateErr != nil {
				addCheck("update_contact_way", false, updateErr.Error())
			} else {
				addCheck("update_contact_way", true, "update_contact_way 调用成功")
			}
		}
	} else {
		addCheck("update_contact_way", true, "未执行真实 update；如需测试，请传 update=1&configId=...&targetGuideUserid=...")
	}
	return writeJSON(w, map[string]any{
		"status":     doctorOverallStatus(checks),
		"dryRun":     dryRun,
		"configId":   configID,
		"checks":     checks,
		"contactWay": contactWay,
		"tips": []string{
			"真实联调前确认自建应用具备客户联系权限、导购在应用可见范围、服务器 IP 已加入可信列表。",
			"企业微信回调必须使用公网 HTTPS，本地 127.0.0.1 只能做页面和接口 dry-run。",
		},
	})
}

func configuredMessage(ok bool) string {
	if ok {
		return "已配置"
	}
	return "未配置"
}

func doctorOverallStatus(checks []scrmWeComCheck) string {
	for _, check := range checks {
		if check.Status != "ok" {
			return "attention"
		}
	}
	return "ok"
}

func (api *API) firstLocalWeComContactWayConfigID(ctx context.Context) string {
	var configID string
	_ = api.db.QueryRowContext(ctx, `SELECT config_id FROM wecom_contact_ways ORDER BY created_at DESC LIMIT 1`).Scan(&configID)
	return configID
}

func (api *API) validateSCRMExistingContactWay(ctx context.Context, configID string) (scrmValidatedContactWay, error) {
	configID = strings.TrimSpace(configID)
	if configID == "" {
		return scrmValidatedContactWay{}, fmt.Errorf("%w: configId is required", errBadRequest)
	}
	if !(api.config.WeComContactWayDryRun || envBool("WECOM_CONTACT_WAY_DRY_RUN", false)) {
		raw, err := api.fetchWeComContactWay(ctx, configID)
		if err != nil {
			return scrmValidatedContactWay{}, err
		}
		errCode := intFromAny(raw["errcode"])
		if errCode != 0 {
			return scrmValidatedContactWay{}, fmt.Errorf("%w: get_contact_way errcode=%d errmsg=%v", errBadRequest, errCode, raw["errmsg"])
		}
		contactWay := mapFromAny(raw["contact_way"])
		if len(contactWay) == 0 {
			contactWay = raw
		}
		return scrmValidatedContactWay{
			ConfigID:    configID,
			QRCodeURL:   stringFromAny(firstAny(contactWay["qr_code"], contactWay["qr_code_url"])),
			State:       stringFromAny(contactWay["state"]),
			BoundUsers:  stringsFromAny(contactWay["user"]),
			Party:       strings.Join(stringsFromAny(contactWay["party"]), ","),
			Remark:      stringFromAny(contactWay["remark"]),
			Source:      "wecom_api",
			RawResponse: raw,
		}, nil
	}
	return api.localWeComContactWayByConfigID(ctx, configID)
}

func (api *API) localWeComContactWayByConfigID(ctx context.Context, configID string) (scrmValidatedContactWay, error) {
	var way scrmValidatedContactWay
	var bound string
	err := api.db.QueryRowContext(ctx, `
		SELECT config_id, qr_code_url, state, bound_userids::text, department_id, name
		FROM wecom_contact_ways
		WHERE config_id = $1
		LIMIT 1
	`, configID).Scan(&way.ConfigID, &way.QRCodeURL, &way.State, &bound, &way.Party, &way.Remark)
	if errors.Is(err, sql.ErrNoRows) {
		return scrmValidatedContactWay{}, fmt.Errorf("%w: config_id %s not found in local wecom_contact_ways; sync Enterprise WeChat assets first", errBadRequest, configID)
	}
	if err != nil {
		return scrmValidatedContactWay{}, err
	}
	way.BoundUsers = stringsFromJSONArray(bound)
	way.Source = "local_wecom_contact_ways"
	return way, nil
}

func (api *API) scrmWeComRetryTasksHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: wecom retry tasks require postgres mode", errBadRequest)
	}
	if r.URL.Path != "/api/scrm/wecom/retries" {
		return errNotFound
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	tasks, err := api.listSCRMWeComRetryTasks(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		return err
	}
	return writeJSON(w, tasks)
}

func (api *API) scrmWeComRetryTaskActionHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: wecom retry tasks require postgres mode", errBadRequest)
	}
	parts := pathParts(r.URL.Path, "/api/scrm/wecom/retries/")
	if len(parts) != 2 || parts[1] != "retry" || r.Method != http.MethodPost {
		return errNotFound
	}
	task, err := api.retrySCRMWeComContactWayTask(r.Context(), parts[0])
	if err != nil {
		return err
	}
	return writeJSON(w, task)
}

func (api *API) scrmWeComIntegrationLogHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: wecom integration log requires postgres mode", errBadRequest)
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	retries, _ := api.listSCRMWeComRetryTasks(r.Context(), "")
	var callback map[string]any
	var rawCallback []byte
	if err := api.db.QueryRowContext(r.Context(), `
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
	`).Scan(&rawCallback); err == nil {
		_ = json.Unmarshal(rawCallback, &callback)
	}
	return writeJSON(w, map[string]any{
		"latestRetry":    firstRetryTask(retries),
		"retryTasks":     retries,
		"latestCallback": callback,
	})
}

func (api *API) listSCRMWeComRetryTasks(ctx context.Context, status string) ([]SCRMWeComRetryTask, error) {
	args := []any{}
	where := ""
	status = strings.TrimSpace(status)
	if status != "" && status != "all" {
		args = append(args, status)
		where = "WHERE status = $1"
	}
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, binding_id, config_id, target_guide_userid, target_state, failed_reason,
			retry_count, status,
			COALESCE(to_char(next_retry_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			COALESCE(to_char(last_attempt_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			COALESCE(to_char(resolved_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'),
			to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM scrm_wecom_contact_way_retry_tasks
		`+where+`
		ORDER BY created_at DESC
		LIMIT 100
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []SCRMWeComRetryTask{}
	for rows.Next() {
		var task SCRMWeComRetryTask
		if err := rows.Scan(&task.ID, &task.BindingID, &task.ConfigID, &task.TargetGuideUserID, &task.TargetState, &task.FailedReason, &task.RetryCount, &task.Status, &task.NextRetryAt, &task.LastAttemptAt, &task.ResolvedAt, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (api *API) insertSCRMWeComRetryTask(ctx context.Context, binding SCRMContactWayBinding, targetGuideUserid, targetState, reason string) {
	_, err := api.db.ExecContext(ctx, `
		INSERT INTO scrm_wecom_contact_way_retry_tasks (
			id, binding_id, config_id, target_guide_userid, target_state, failed_reason, status, next_retry_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'pending', now() + interval '1 minute')
	`, newID("swrt"), binding.ID, binding.ConfigID, targetGuideUserid, targetState, truncateText(reason, 500))
	if err != nil {
		// Retry logging must never hide the original update_contact_way failure.
		_ = err
	}
}

func (api *API) retrySCRMWeComContactWayTask(ctx context.Context, retryID string) (SCRMWeComRetryTask, error) {
	var task SCRMWeComRetryTask
	err := api.db.QueryRowContext(ctx, `
		SELECT id, binding_id, config_id, target_guide_userid, target_state, failed_reason, retry_count, status,
			COALESCE(to_char(next_retry_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			COALESCE(to_char(last_attempt_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			COALESCE(to_char(resolved_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'),
			to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM scrm_wecom_contact_way_retry_tasks
		WHERE id = $1
	`, retryID).Scan(&task.ID, &task.BindingID, &task.ConfigID, &task.TargetGuideUserID, &task.TargetState, &task.FailedReason, &task.RetryCount, &task.Status, &task.NextRetryAt, &task.LastAttemptAt, &task.ResolvedAt, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SCRMWeComRetryTask{}, errNotFound
	}
	if err != nil {
		return SCRMWeComRetryTask{}, err
	}
	if task.Status == "resolved" {
		return task, nil
	}
	if err := api.updateWeComContactWay(ctx, task.ConfigID, "SCRM retry update_contact_way", true, task.TargetState, []string{task.TargetGuideUserID}); err != nil {
		_, _ = api.db.ExecContext(ctx, `
			UPDATE scrm_wecom_contact_way_retry_tasks
			SET retry_count = retry_count + 1,
				failed_reason = $2,
				last_attempt_at = now(),
				next_retry_at = now() + interval '5 minutes',
				updated_at = now()
			WHERE id = $1
		`, retryID, truncateText(err.Error(), 500))
		return SCRMWeComRetryTask{}, err
	}
	_, err = api.db.ExecContext(ctx, `
		UPDATE scrm_wecom_contact_way_retry_tasks
		SET retry_count = retry_count + 1,
			status = 'resolved',
			last_attempt_at = now(),
			resolved_at = now(),
			updated_at = now()
		WHERE id = $1
	`, retryID)
	if err != nil {
		return SCRMWeComRetryTask{}, err
	}
	tasks, err := api.listSCRMWeComRetryTasks(ctx, "resolved")
	if err != nil {
		return SCRMWeComRetryTask{}, err
	}
	for _, item := range tasks {
		if item.ID == retryID {
			return item, nil
		}
	}
	return task, nil
}

func firstRetryTask(tasks []SCRMWeComRetryTask) any {
	if len(tasks) == 0 {
		return nil
	}
	return tasks[0]
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := strconv.Atoi(v.String())
		return i
	case string:
		i, _ := strconv.Atoi(v)
		return i
	default:
		return 0
	}
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	default:
		if value == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return nil
}

func stringsFromAny(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		rows := []string{}
		for _, item := range v {
			if text := stringFromAny(item); text != "" {
				rows = append(rows, text)
			}
		}
		return rows
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{strings.TrimSpace(v)}
	default:
		return nil
	}
}

func firstAny(values ...any) any {
	for _, value := range values {
		if stringFromAny(value) != "" {
			return value
		}
	}
	return nil
}
