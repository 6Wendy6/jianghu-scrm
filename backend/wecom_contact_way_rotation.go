package main

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SCRMStoreRecord struct {
	ID            string `json:"id"`
	StoreCode     string `json:"storeCode"`
	StoreName     string `json:"storeName"`
	ManagerUserID string `json:"managerUserid"`
	Status        string `json:"status"`
}

type SCRMStoreGuide struct {
	ID             string `json:"id"`
	StoreID        string `json:"storeId"`
	GuideUserID    string `json:"guideUserid"`
	GuideName      string `json:"guideName"`
	SortOrder      int    `json:"sortOrder"`
	Status         string `json:"status"`
	DailyLimit     int    `json:"dailyLimit"`
	AssignedToday  int    `json:"assignedToday"`
	LastAssignedAt string `json:"lastAssignedAt,omitempty"`
}

type SCRMContactWayBinding struct {
	ID                 string `json:"id"`
	StoreID            string `json:"storeId"`
	StoreCode          string `json:"storeCode"`
	StoreName          string `json:"storeName"`
	ManagerUserID      string `json:"managerUserid"`
	ConfigID           string `json:"configId"`
	QRCodeURL          string `json:"qrCodeUrl"`
	CurrentGuideUserID string `json:"currentGuideUserid"`
	NextIndex          int    `json:"nextIndex"`
	StatePrefix        string `json:"statePrefix"`
	Remark             string `json:"remark"`
	Status             string `json:"status"`
	LastSwitchedAt     string `json:"lastSwitchedAt,omitempty"`
	LastError          string `json:"lastError,omitempty"`
	CustomerCount      int    `json:"customerCount"`
}

type SCRMCustomerAssignment struct {
	ID             string          `json:"id"`
	StoreID        string          `json:"storeId"`
	StoreCode      string          `json:"storeCode,omitempty"`
	StoreName      string          `json:"storeName,omitempty"`
	BindingID      string          `json:"bindingId"`
	ConfigID       string          `json:"configId"`
	ManagerUserID  string          `json:"managerUserid"`
	GuideUserID    string          `json:"guideUserid"`
	ExternalUserID string          `json:"externalUserid"`
	State          string          `json:"state"`
	EventType      string          `json:"eventType"`
	AddTime        string          `json:"addTime,omitempty"`
	Status         string          `json:"status"`
	RawEvent       json.RawMessage `json:"rawEvent,omitempty"`
	CreatedAt      string          `json:"createdAt,omitempty"`
}

type wecomCustomerAddEvent struct {
	ToUserName     string `json:"toUserName" xml:"ToUserName"`
	FromUserName   string `json:"fromUserName" xml:"FromUserName"`
	CreateTime     int64  `json:"createTime" xml:"CreateTime"`
	MsgType        string `json:"msgType" xml:"MsgType"`
	Event          string `json:"event" xml:"Event"`
	ChangeType     string `json:"changeType" xml:"ChangeType"`
	UserID         string `json:"userId" xml:"UserID"`
	ExternalUserID string `json:"externalUserid" xml:"ExternalUserID"`
	State          string `json:"state" xml:"State"`
	WelcomeCode    string `json:"welcomeCode" xml:"WelcomeCode"`
}

func (api *API) scrmContactWayBindingsHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: contact-way rotation requires postgres mode", errBadRequest)
	}
	if r.URL.Path != "/api/scrm/contact-way-bindings" {
		return errNotFound
	}
	switch r.Method {
	case http.MethodGet:
		return api.listSCRMContactWayBindings(w, r)
	case http.MethodPost:
		return api.bindExistingSCRMContactWay(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
}

func (api *API) scrmContactWayBindingActionHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: contact-way rotation requires postgres mode", errBadRequest)
	}
	parts := pathParts(r.URL.Path, "/api/scrm/contact-way-bindings/")
	if len(parts) != 2 || r.Method != http.MethodPost {
		return errNotFound
	}
	switch parts[1] {
	case "activate":
		binding, err := api.activateSCRMContactWayBinding(r.Context(), parts[0])
		if err != nil {
			return err
		}
		return writeJSON(w, binding)
	case "switch-next":
		binding, err := api.switchSCRMContactWayBindingToNextGuide(r.Context(), parts[0])
		if err != nil {
			return err
		}
		return writeJSON(w, binding)
	case "pause":
		binding, err := api.setSCRMContactWayBindingStatus(r.Context(), parts[0], "paused")
		if err != nil {
			return err
		}
		return writeJSON(w, binding)
	case "enable":
		binding, err := api.setSCRMContactWayBindingStatus(r.Context(), parts[0], "active")
		if err != nil {
			return err
		}
		return writeJSON(w, binding)
	default:
		return errNotFound
	}
}

func (api *API) scrmStoreGuidesHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: contact-way rotation requires postgres mode", errBadRequest)
	}
	parts := pathParts(r.URL.Path, "/api/scrm/stores/")
	if len(parts) != 2 || parts[1] != "guides" {
		return errNotFound
	}
	switch r.Method {
	case http.MethodGet:
		guides, err := api.listSCRMStoreGuides(r.Context(), parts[0])
		if err != nil {
			return err
		}
		return writeJSON(w, guides)
	case http.MethodPost:
		return api.upsertSCRMStoreGuides(w, r, parts[0])
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
}

func (api *API) scrmCustomerAssignmentsHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: contact-way rotation requires postgres mode", errBadRequest)
	}
	if r.URL.Path != "/api/scrm/customer-assignments" {
		return errNotFound
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		page, pageSize = 1, 200
	}
	where, args := scrmCustomerAssignmentFilters(r)
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	var total int
	if err := api.db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM scrm_customer_assignments a
		JOIN scrm_stores s ON s.id = a.store_id
		`+where, args...).Scan(&total); err != nil {
		return err
	}
	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := api.db.QueryContext(ctx, `
		SELECT a.id, a.store_id, s.store_code, s.store_name, a.binding_id, a.config_id,
			a.manager_userid, a.guide_userid, a.external_userid, a.state, a.event_type,
			COALESCE(to_char(a.add_time, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			a.status,
			COALESCE(a.raw_event::text, '{}'::text),
			to_char(a.created_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM scrm_customer_assignments a
		JOIN scrm_stores s ON s.id = a.store_id
		`+where+`
		ORDER BY COALESCE(a.add_time, a.created_at) DESC, a.id DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		queryArgs...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	assignments := []SCRMCustomerAssignment{}
	for rows.Next() {
		var item SCRMCustomerAssignment
		var raw string
		if err := rows.Scan(&item.ID, &item.StoreID, &item.StoreCode, &item.StoreName, &item.BindingID, &item.ConfigID, &item.ManagerUserID, &item.GuideUserID, &item.ExternalUserID, &item.State, &item.EventType, &item.AddTime, &item.Status, &raw, &item.CreatedAt); err != nil {
			return err
		}
		item.RawEvent = json.RawMessage(raw)
		assignments = append(assignments, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !paged {
		return writeJSON(w, assignments)
	}
	return writeJSON(w, PageResult[SCRMCustomerAssignment]{Data: assignments, Page: makePageMeta(page, pageSize, total)})
}

func scrmCustomerAssignmentFilters(r *http.Request) (string, []any) {
	query := r.URL.Query()
	args := []any{}
	conditions := []string{}
	addFilter := func(column, value string) {
		value = strings.TrimSpace(value)
		if value == "" || value == "全部" || strings.EqualFold(value, "all") {
			return
		}
		args = append(args, value)
		conditions = append(conditions, column+" = $"+strconv.Itoa(len(args)))
	}
	addFilter("a.store_id", query.Get("storeId"))
	addFilter("s.store_code", query.Get("storeCode"))
	addFilter("a.manager_userid", query.Get("managerUserid"))
	addFilter("a.guide_userid", query.Get("guideUserid"))
	addFilter("a.config_id", query.Get("configId"))
	addFilter("a.status", query.Get("status"))
	if start := strings.TrimSpace(query.Get("start")); start != "" {
		args = append(args, start)
		conditions = append(conditions, "COALESCE(a.add_time, a.created_at) >= $"+strconv.Itoa(len(args))+"::timestamptz")
	}
	if end := strings.TrimSpace(query.Get("end")); end != "" {
		args = append(args, end)
		conditions = append(conditions, "COALESCE(a.add_time, a.created_at) <= $"+strconv.Itoa(len(args))+"::timestamptz")
	}
	conditions, args = appendAssignmentScope(conditions, args, scrmScopeFromRequest(r))
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (api *API) bindExistingSCRMContactWay(w http.ResponseWriter, r *http.Request) error {
	var payload struct {
		StoreID       string `json:"storeId"`
		StoreCode     string `json:"storeCode"`
		StoreName     string `json:"storeName"`
		ManagerUserID string `json:"managerUserid"`
		ConfigID      string `json:"configId"`
		QRCodeURL     string `json:"qrCodeUrl"`
		Remark        string `json:"remark"`
		StatePrefix   string `json:"statePrefix"`
	}
	if err := decode(r, &payload); err != nil {
		return err
	}
	payload.StoreCode = strings.TrimSpace(payload.StoreCode)
	payload.StoreName = strings.TrimSpace(payload.StoreName)
	payload.ManagerUserID = strings.TrimSpace(payload.ManagerUserID)
	payload.ConfigID = strings.TrimSpace(payload.ConfigID)
	if payload.StoreCode == "" || payload.StoreName == "" || payload.ConfigID == "" {
		return fmt.Errorf("%w: storeCode, storeName and configId are required", errBadRequest)
	}
	var existingBindingID, existingStoreID string
	err := api.db.QueryRowContext(r.Context(), `
		SELECT id, store_id
		FROM scrm_wecom_contact_way_bindings
		WHERE config_id = $1
	`, payload.ConfigID).Scan(&existingBindingID, &existingStoreID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if existingBindingID != "" {
		return fmt.Errorf("%w: config_id %s already bound to store %s as binding %s", errBadRequest, payload.ConfigID, existingStoreID, existingBindingID)
	}
	validatedWay, err := api.validateSCRMExistingContactWay(r.Context(), payload.ConfigID)
	if err != nil {
		return err
	}
	if validatedWay.QRCodeURL == "" {
		return fmt.Errorf("%w: config_id %s has no qr_code_url", errBadRequest, payload.ConfigID)
	}
	payload.QRCodeURL = validatedWay.QRCodeURL
	if strings.TrimSpace(payload.Remark) == "" {
		payload.Remark = firstNonEmpty(validatedWay.Remark, "门店店长码")
	}
	storeID := strings.TrimSpace(payload.StoreID)
	if storeID == "" {
		storeID = payload.StoreCode
	}
	statePrefix := strings.TrimSpace(payload.StatePrefix)
	if statePrefix == "" {
		statePrefix = buildSCRMStatePrefix(payload.StoreCode, payload.ManagerUserID)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO scrm_stores (id, store_code, store_name, manager_userid, status)
		VALUES ($1, $2, $3, $4, 'active')
		ON CONFLICT (store_code) DO UPDATE SET
			store_name = EXCLUDED.store_name,
			manager_userid = EXCLUDED.manager_userid,
			status = 'active',
			updated_at = now()
	`, storeID, payload.StoreCode, payload.StoreName, payload.ManagerUserID); err != nil {
		return err
	}
	var bindingID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO scrm_wecom_contact_way_bindings (
			id, store_id, manager_userid, config_id, qr_code_url, state_prefix, remark, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 'active')
		ON CONFLICT (config_id) DO UPDATE SET
			store_id = EXCLUDED.store_id,
			manager_userid = EXCLUDED.manager_userid,
			qr_code_url = EXCLUDED.qr_code_url,
			state_prefix = EXCLUDED.state_prefix,
			remark = EXCLUDED.remark,
			status = 'active',
			last_error = '',
			updated_at = now()
		RETURNING id
	`, newID("scwb"), storeID, payload.ManagerUserID, payload.ConfigID, strings.TrimSpace(payload.QRCodeURL), statePrefix, strings.TrimSpace(payload.Remark)).Scan(&bindingID)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	binding, err := api.getSCRMContactWayBinding(ctx, bindingID)
	if err != nil {
		return err
	}
	log.Printf("scrm contact-way binding saved config_id=%s store_code=%s binding_id=%s\n", binding.ConfigID, binding.StoreCode, binding.ID)
	return writeJSON(w, binding)
}

func (api *API) listSCRMContactWayBindings(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	where, args := scrmScopeFromRequest(r).bindingWhere()
	rows, err := api.db.QueryContext(ctx, `
		SELECT b.id, b.store_id, s.store_code, s.store_name, b.manager_userid, b.config_id, b.qr_code_url,
			b.current_guide_userid, b.next_index, b.state_prefix, b.remark, b.status,
			COALESCE(to_char(b.last_switched_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			b.last_error,
			count(a.id)::int
		FROM scrm_wecom_contact_way_bindings b
		JOIN scrm_stores s ON s.id = b.store_id
		LEFT JOIN scrm_customer_assignments a ON a.binding_id = b.id
		`+where+`
		GROUP BY b.id, s.store_code, s.store_name
		ORDER BY b.updated_at DESC, b.id
	`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	bindings := []SCRMContactWayBinding{}
	for rows.Next() {
		binding, err := scanSCRMContactWayBinding(rows)
		if err != nil {
			return err
		}
		bindings = append(bindings, binding)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writeJSON(w, bindings)
}

func (api *API) upsertSCRMStoreGuides(w http.ResponseWriter, r *http.Request, storeID string) error {
	var payload struct {
		Guides []struct {
			GuideUserID string `json:"guideUserid"`
			GuideName   string `json:"guideName"`
			SortOrder   int    `json:"sortOrder"`
			Status      string `json:"status"`
			DailyLimit  int    `json:"dailyLimit"`
		} `json:"guides"`
	}
	if err := decode(r, &payload); err != nil {
		return err
	}
	if len(payload.Guides) == 0 {
		return fmt.Errorf("%w: guides are required", errBadRequest)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if !scrmStoreExists(ctx, tx, storeID) {
		return errNotFound
	}
	for index, guide := range payload.Guides {
		userID := strings.TrimSpace(guide.GuideUserID)
		if userID == "" {
			return fmt.Errorf("%w: guideUserid is required", errBadRequest)
		}
		sortOrder := guide.SortOrder
		if sortOrder == 0 {
			sortOrder = index + 1
		}
		status := strings.TrimSpace(guide.Status)
		if status == "" {
			status = "active"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO scrm_store_guides (
				id, store_id, guide_userid, guide_name, sort_order, status, daily_limit
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (store_id, guide_userid) DO UPDATE SET
				guide_name = EXCLUDED.guide_name,
				sort_order = EXCLUDED.sort_order,
				status = EXCLUDED.status,
				daily_limit = EXCLUDED.daily_limit,
				updated_at = now()
		`, newID("scg"), storeID, userID, strings.TrimSpace(guide.GuideName), sortOrder, status, max(0, guide.DailyLimit)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	guides, err := api.listSCRMStoreGuides(ctx, storeID)
	if err != nil {
		return err
	}
	return writeJSON(w, guides)
}

func (api *API) activateSCRMContactWayBinding(ctx context.Context, bindingID string) (SCRMContactWayBinding, error) {
	return api.switchSCRMContactWayBinding(ctx, bindingID, 0)
}

func (api *API) switchSCRMContactWayBindingToNextGuide(ctx context.Context, bindingID string) (SCRMContactWayBinding, error) {
	return api.switchSCRMContactWayBinding(ctx, bindingID, -1)
}

func (api *API) switchSCRMContactWayBinding(ctx context.Context, bindingID string, forcedIndex int) (SCRMContactWayBinding, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return SCRMContactWayBinding{}, err
	}
	defer tx.Rollback()
	binding, err := api.getSCRMContactWayBindingForUpdate(ctx, tx, bindingID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SCRMContactWayBinding{}, errNotFound
		}
		return SCRMContactWayBinding{}, err
	}
	if binding.Status != "active" && forcedIndex < 0 {
		return SCRMContactWayBinding{}, fmt.Errorf("%w: contact-way binding is %s; enable or activate it before switching", errBadRequest, binding.Status)
	}
	guides, err := listAvailableSCRMStoreGuidesForUpdate(ctx, tx, binding.StoreID)
	if err != nil {
		return SCRMContactWayBinding{}, err
	}
	if len(guides) == 0 {
		message := "no active guide available for contact-way binding"
		_, _ = tx.ExecContext(ctx, `UPDATE scrm_wecom_contact_way_bindings SET last_error = $2, updated_at = now() WHERE id = $1`, binding.ID, message)
		if err := tx.Commit(); err != nil {
			return SCRMContactWayBinding{}, err
		}
		return SCRMContactWayBinding{}, fmt.Errorf("%w: %s", errBadRequest, message)
	}
	nextIndex := binding.NextIndex
	if forcedIndex >= 0 {
		nextIndex = forcedIndex
	}
	slot := positiveModulo(nextIndex, len(guides))
	nextGuide := guides[slot]
	nextState := buildSCRMContactWayState(binding.StoreCode, binding.ManagerUserID, nextGuide.GuideUserID, binding.ID)
	if err := api.updateWeComContactWay(ctx, binding.ConfigID, binding.Remark, true, nextState, []string{nextGuide.GuideUserID}); err != nil {
		message := truncateText(err.Error(), 500)
		_, _ = tx.ExecContext(ctx, `UPDATE scrm_wecom_contact_way_bindings SET last_error = $2, updated_at = now() WHERE id = $1`, binding.ID, message)
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO scrm_wecom_contact_way_retry_tasks (
				id, binding_id, config_id, target_guide_userid, target_state, failed_reason, status, next_retry_at
			) VALUES ($1, $2, $3, $4, $5, $6, 'pending', now() + interval '1 minute')
		`, newID("swrt"), binding.ID, binding.ConfigID, nextGuide.GuideUserID, nextState, message)
		_ = tx.Commit()
		return SCRMContactWayBinding{}, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE scrm_wecom_contact_way_bindings
		SET current_guide_userid = $2,
			next_index = $3,
			status = CASE WHEN $4 THEN 'active' ELSE status END,
			last_switched_at = now(),
			last_error = '',
			updated_at = now()
		WHERE id = $1
	`, binding.ID, nextGuide.GuideUserID, nextIndex+1, forcedIndex >= 0)
	if err != nil {
		return SCRMContactWayBinding{}, err
	}
	if err := tx.Commit(); err != nil {
		return SCRMContactWayBinding{}, err
	}
	log.Printf("scrm contact-way switched binding_id=%s config_id=%s guide_userid=%s state=%s\n", binding.ID, binding.ConfigID, nextGuide.GuideUserID, nextState)
	return api.getSCRMContactWayBinding(ctx, binding.ID)
}

func (api *API) setSCRMContactWayBindingStatus(ctx context.Context, bindingID, status string) (SCRMContactWayBinding, error) {
	status = strings.TrimSpace(status)
	if status != "active" && status != "paused" {
		return SCRMContactWayBinding{}, fmt.Errorf("%w: invalid contact-way binding status", errBadRequest)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	res, err := api.db.ExecContext(ctx, `
		UPDATE scrm_wecom_contact_way_bindings
		SET status = $2,
			last_error = '',
			updated_at = now()
		WHERE id = $1
	`, bindingID, status)
	if err != nil {
		return SCRMContactWayBinding{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return SCRMContactWayBinding{}, err
	}
	if affected == 0 {
		return SCRMContactWayBinding{}, errNotFound
	}
	return api.getSCRMContactWayBinding(ctx, bindingID)
}

func (api *API) updateWeComContactWay(ctx context.Context, configID, remark string, skipVerify bool, state string, users []string) error {
	if api.config.WeComContactWayDryRun || envBool("WECOM_CONTACT_WAY_DRY_RUN", false) {
		log.Printf("wecom update_contact_way dry_run config_id=%s state=%s users=%s\n", configID, state, strings.Join(users, ","))
		return nil
	}
	token, err := api.wecomAccessToken(ctx, false)
	if err != nil {
		return fmt.Errorf("get wecom access_token: %w", err)
	}
	reqBody := map[string]any{
		"config_id":   configID,
		"remark":      remark,
		"skip_verify": skipVerify,
		"state":       state,
		"user":        users,
	}
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := postWeComJSON(ctx, wecomAPIBase+"/cgi-bin/externalcontact/update_contact_way?access_token="+url.QueryEscape(token), reqBody, &resp); err != nil {
		return err
	}
	if resp.ErrCode != 0 {
		return wecomAPIError("update_contact_way", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}

func (api *API) fetchWeComContactWay(ctx context.Context, configID string) (map[string]any, error) {
	token, err := api.wecomAccessToken(ctx, false)
	if err != nil {
		return nil, err
	}
	reqBody := map[string]any{"config_id": configID}
	var resp map[string]any
	if err := postWeComJSON(ctx, wecomAPIBase+"/cgi-bin/externalcontact/get_contact_way?access_token="+url.QueryEscape(token), reqBody, &resp); err != nil {
		return nil, err
	}
	if errCode := intFromAny(resp["errcode"]); errCode != 0 {
		return nil, wecomAPIError("get_contact_way", errCode, stringFromAny(resp["errmsg"]))
	}
	return resp, nil
}

func (api *API) handleWeComCustomerEventXML(ctx context.Context, corpID string, plainXML []byte) error {
	var event wecomCustomerAddEvent
	if err := xml.Unmarshal(plainXML, &event); err != nil {
		return api.saveUnknownWeComCallbackEvent(ctx, "xml_parse_failed", plainXML, err)
	}
	rawJSON, _ := json.Marshal(map[string]any{"xml": string(plainXML)})
	return api.handleWeComCustomerEvent(ctx, corpID, event, rawJSON)
}

func (api *API) handleWeComCustomerEventJSON(ctx context.Context, corpID string, body []byte) error {
	var event wecomCustomerAddEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return api.saveUnknownWeComCallbackEvent(ctx, "json_parse_failed", body, err)
	}
	return api.handleWeComCustomerEvent(ctx, corpID, event, body)
}

func (api *API) handleWeComCustomerEvent(ctx context.Context, corpID string, event wecomCustomerAddEvent, raw []byte) error {
	event.Event = strings.TrimSpace(event.Event)
	event.ChangeType = strings.TrimSpace(event.ChangeType)
	event.UserID = strings.TrimSpace(event.UserID)
	event.ExternalUserID = strings.TrimSpace(event.ExternalUserID)
	event.State = strings.TrimSpace(event.State)
	eventKey := buildWeComCallbackEventKey(event)
	eventType := strings.Trim(strings.Join([]string{event.Event, event.ChangeType}, "/"), "/")
	if eventType == "" {
		eventType = "unknown"
	}
	inserted, err := api.insertSCRMCallbackEvent(ctx, eventKey, eventType, event.ExternalUserID, event.UserID, event.State, false, raw)
	if err != nil {
		return err
	}
	if !inserted {
		log.Printf("wecom callback duplicate skipped event_key=%s event_type=%s\n", eventKey, eventType)
		return nil
	}
	if event.Event != "change_external_contact" || (event.ChangeType != "add_external_contact" && event.ChangeType != "add_half_external_contact") {
		log.Printf("wecom callback stored but ignored event=%s change_type=%s\n", event.Event, event.ChangeType)
		return nil
	}
	bindingID := parseBindingIDFromSCRMState(event.State)
	if bindingID == "" {
		log.Printf("wecom callback stored without binding state=%s external_userid=%s\n", event.State, event.ExternalUserID)
		return nil
	}
	binding, err := api.getSCRMContactWayBinding(ctx, bindingID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, errNotFound) {
			log.Printf("wecom callback binding not found binding_id=%s state=%s\n", bindingID, event.State)
			return nil
		}
		return err
	}
	if err := api.saveLegacyWeComCustomerEvent(ctx, corpID, binding.ID, event, raw); err != nil {
		return err
	}
	if err := api.saveSCRMCustomerAssignment(ctx, binding, event, raw); err != nil {
		return err
	}
	if _, err := api.switchSCRMContactWayBindingToNextGuide(ctx, binding.ID); err != nil {
		log.Printf("scrm contact-way switch after callback failed binding_id=%s err=%v\n", binding.ID, err)
	}
	if err := api.markSCRMCallbackEventProcessed(ctx, eventKey); err != nil {
		return err
	}
	return nil
}

func (api *API) saveLegacyWeComCustomerEvent(ctx context.Context, corpID, contactWayID string, event wecomCustomerAddEvent, raw []byte) error {
	var addTime any
	if event.CreateTime > 0 {
		addTime = time.Unix(event.CreateTime, 0)
	}
	_, err := api.db.ExecContext(ctx, `
		INSERT INTO wecom_customer_events (id, corp_id, external_userid, follow_userid, state, contact_way_id, event_type, add_time, raw_payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
	`, newID("wce"), corpID, event.ExternalUserID, event.UserID, event.State, contactWayID, event.ChangeType, addTime, string(raw))
	return err
}

func (api *API) saveSCRMCustomerAssignment(ctx context.Context, binding SCRMContactWayBinding, event wecomCustomerAddEvent, raw []byte) error {
	var addTime any
	if event.CreateTime > 0 {
		addTime = time.Unix(event.CreateTime, 0)
	}
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO scrm_customer_assignments (
			id, store_id, binding_id, config_id, manager_userid, guide_userid,
			external_userid, state, event_type, add_time, status, raw_event
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'added', $11::jsonb)
	`, newID("sca"), binding.StoreID, binding.ID, binding.ConfigID, binding.ManagerUserID, event.UserID, event.ExternalUserID, event.State, event.ChangeType, addTime, string(raw))
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE scrm_store_guides
		SET assigned_today = assigned_today + 1,
			last_assigned_at = now(),
			updated_at = now()
		WHERE store_id = $1 AND guide_userid = $2
	`, binding.StoreID, event.UserID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (api *API) insertSCRMCallbackEvent(ctx context.Context, eventKey, eventType, externalUserID, userID, state string, processed bool, raw []byte) (bool, error) {
	var id string
	err := api.db.QueryRowContext(ctx, `
		INSERT INTO scrm_wecom_callback_events (
			id, event_key, event_type, external_userid, user_id, state, processed, raw_event
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
		ON CONFLICT (event_key) DO NOTHING
		RETURNING id
	`, newID("swce"), eventKey, eventType, externalUserID, userID, state, processed, string(raw)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (api *API) markSCRMCallbackEventProcessed(ctx context.Context, eventKey string) error {
	_, err := api.db.ExecContext(ctx, `UPDATE scrm_wecom_callback_events SET processed = true WHERE event_key = $1`, eventKey)
	return err
}

func (api *API) saveUnknownWeComCallbackEvent(ctx context.Context, eventType string, raw []byte, parseErr error) error {
	body, _ := json.Marshal(map[string]any{
		"raw":   string(raw),
		"error": parseErr.Error(),
	})
	key := "unknown:" + sha1Hex(raw)
	inserted, err := api.insertSCRMCallbackEvent(ctx, key, eventType, "", "", "", false, body)
	if err == nil && inserted {
		log.Printf("wecom callback unknown event saved type=%s err=%v\n", eventType, parseErr)
	}
	return err
}

func (api *API) getSCRMContactWayBinding(ctx context.Context, id string) (SCRMContactWayBinding, error) {
	row := api.db.QueryRowContext(ctx, `
		SELECT b.id, b.store_id, s.store_code, s.store_name, b.manager_userid, b.config_id, b.qr_code_url,
			b.current_guide_userid, b.next_index, b.state_prefix, b.remark, b.status,
			COALESCE(to_char(b.last_switched_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			b.last_error,
			(SELECT count(*)::int FROM scrm_customer_assignments a WHERE a.binding_id = b.id)
		FROM scrm_wecom_contact_way_bindings b
		JOIN scrm_stores s ON s.id = b.store_id
		WHERE b.id = $1
	`, id)
	return scanSCRMContactWayBinding(row)
}

func (api *API) getSCRMContactWayBindingForUpdate(ctx context.Context, tx *sql.Tx, id string) (SCRMContactWayBinding, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT b.id, b.store_id, s.store_code, s.store_name, b.manager_userid, b.config_id, b.qr_code_url,
			b.current_guide_userid, b.next_index, b.state_prefix, b.remark, b.status,
			COALESCE(to_char(b.last_switched_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			b.last_error,
			(SELECT count(*)::int FROM scrm_customer_assignments a WHERE a.binding_id = b.id)
		FROM scrm_wecom_contact_way_bindings b
		JOIN scrm_stores s ON s.id = b.store_id
		WHERE b.id = $1
		FOR UPDATE OF b
	`, id)
	return scanSCRMContactWayBinding(row)
}

func scanSCRMContactWayBinding(row scanner) (SCRMContactWayBinding, error) {
	var binding SCRMContactWayBinding
	err := row.Scan(
		&binding.ID, &binding.StoreID, &binding.StoreCode, &binding.StoreName, &binding.ManagerUserID,
		&binding.ConfigID, &binding.QRCodeURL, &binding.CurrentGuideUserID, &binding.NextIndex,
		&binding.StatePrefix, &binding.Remark, &binding.Status, &binding.LastSwitchedAt, &binding.LastError,
		&binding.CustomerCount,
	)
	return binding, err
}

func (api *API) listSCRMStoreGuides(ctx context.Context, storeID string) ([]SCRMStoreGuide, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rows, err := api.db.QueryContext(queryCtx, `
		SELECT id, store_id, guide_userid, guide_name, sort_order, status, daily_limit, assigned_today,
			COALESCE(to_char(last_assigned_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), '')
		FROM scrm_store_guides
		WHERE store_id = $1
		ORDER BY sort_order, id
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	guides := []SCRMStoreGuide{}
	for rows.Next() {
		var guide SCRMStoreGuide
		if err := rows.Scan(&guide.ID, &guide.StoreID, &guide.GuideUserID, &guide.GuideName, &guide.SortOrder, &guide.Status, &guide.DailyLimit, &guide.AssignedToday, &guide.LastAssignedAt); err != nil {
			return nil, err
		}
		guides = append(guides, guide)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return guides, nil
}

func listAvailableSCRMStoreGuidesForUpdate(ctx context.Context, tx *sql.Tx, storeID string) ([]SCRMStoreGuide, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, store_id, guide_userid, guide_name, sort_order, status, daily_limit, assigned_today,
			COALESCE(to_char(last_assigned_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), '')
		FROM scrm_store_guides
		WHERE store_id = $1
			AND status = 'active'
			AND (daily_limit <= 0 OR assigned_today < daily_limit)
		ORDER BY sort_order, id
		FOR UPDATE
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	guides := []SCRMStoreGuide{}
	for rows.Next() {
		var guide SCRMStoreGuide
		if err := rows.Scan(&guide.ID, &guide.StoreID, &guide.GuideUserID, &guide.GuideName, &guide.SortOrder, &guide.Status, &guide.DailyLimit, &guide.AssignedToday, &guide.LastAssignedAt); err != nil {
			return nil, err
		}
		guides = append(guides, guide)
	}
	return guides, rows.Err()
}

func scrmStoreExists(ctx context.Context, tx *sql.Tx, storeID string) bool {
	var marker int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM scrm_stores WHERE id = $1`, storeID).Scan(&marker)
	return err == nil
}

func buildSCRMStatePrefix(storeCode, managerUserID string) string {
	return "store:" + sanitizeSCRMStatePart(storeCode) + ":manager:" + sanitizeSCRMStatePart(managerUserID)
}

func buildSCRMContactWayState(storeCode, managerUserID, guideUserID, bindingID string) string {
	return buildSCRMStatePrefix(storeCode, managerUserID) + ":guide:" + sanitizeSCRMStatePart(guideUserID) + ":bind:" + sanitizeSCRMStatePart(bindingID) + ":v1"
}

func parseBindingIDFromSCRMState(state string) string {
	parts := strings.Split(state, ":")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "bind" {
			return strings.TrimSpace(parts[i+1])
		}
	}
	return ""
}

func buildWeComCallbackEventKey(event wecomCustomerAddEvent) string {
	parts := []string{
		event.Event,
		event.ChangeType,
		event.UserID,
		event.ExternalUserID,
		event.State,
		event.WelcomeCode,
		strconv.FormatInt(event.CreateTime, 10),
	}
	return "wecom:" + sha1Hex([]byte(strings.Join(parts, "|")))
}

func sanitizeSCRMStatePart(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, ":", "_")
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func positiveModulo(value, mod int) int {
	if mod <= 0 {
		return 0
	}
	next := value % mod
	if next < 0 {
		return next + mod
	}
	return next
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func sha1Hex(value []byte) string {
	sum := sha1.Sum(value)
	return hex.EncodeToString(sum[:])
}
