package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (api *API) customersHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.customersDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method == http.MethodPost {
		var payload Customer
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.Name == "" {
			return fmt.Errorf("%w: customer name is required", errBadRequest)
		}
		payload.ID = newID("c")
		payload.Timeline = []AuditEvent{audit(stamp(), "后台新增客户", "已入客户池", "运营", "客户："+payload.Name, "用于补录或测试，来源归属需要显式填写。")}
		api.customers = append([]Customer{payload}, api.customers...)
		return writeJSON(w, payload)
	}
	keyword := r.URL.Query().Get("keyword")
	guide := r.URL.Query().Get("guide")
	stage := r.URL.Query().Get("stage")
	rows := filter(api.customers, func(c Customer) bool {
		return match(keyword, c.Name+c.Mobile+c.Wecom+c.Source) && matchOption(guide, "全部导购", c.Owner) && matchOption(stage, "全部阶段", c.Stage)
	})
	return writePaginatedOrList(w, r, rows)
}

func (api *API) customersDBHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method == http.MethodPost {
		var payload Customer
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.Name == "" {
			return fmt.Errorf("%w: customer name is required", errBadRequest)
		}
		if payload.ID == "" {
			payload.ID = newID("c")
		}
		if payload.Owner == "" {
			payload.Owner = "运营"
		}
		if payload.Stage == "" {
			payload.Stage = "新客待转化"
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		_, err := api.db.ExecContext(ctx, `
			INSERT INTO customers (
				id, name, wecom_name, mobile_masked, lifecycle_stage, sales_stage,
				owner_staff_name, source_store_name, intent_score, updated_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,now())
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				wecom_name = EXCLUDED.wecom_name,
				mobile_masked = EXCLUDED.mobile_masked,
				lifecycle_stage = EXCLUDED.lifecycle_stage,
				sales_stage = EXCLUDED.sales_stage,
				owner_staff_name = EXCLUDED.owner_staff_name,
				source_store_name = EXCLUDED.source_store_name,
				intent_score = EXCLUDED.intent_score,
				updated_at = now()
		`, payload.ID, payload.Name, payload.Wecom, payload.Mobile, payload.Stage, payload.SalesStage, payload.Owner, payload.SourceStore, payload.IntentScore)
		if err != nil {
			return err
		}
		api.invalidateCache("summary:", "customers:", "tag-customers:")
		payload.Timeline = []AuditEvent{audit(stamp(), "后台新增客户", "已写入数据库客户池", "运营", "客户："+payload.Name, "PostgreSQL 模式")}
		return writeJSON(w, payload)
	}

	cacheKey := requestCacheKey("customers", r)
	if api.serveCachedJSON(w, r, cacheKey) {
		return nil
	}
	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		page, pageSize = 1, 200
	}
	where, args := customerDBFilters(r)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var total int
	countSQL := `SELECT count(*) FROM customers ` + where
	if err := api.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return err
	}

	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := api.db.QueryContext(ctx, `
		SELECT
			id, name, wecom_name, mobile_masked, owner_staff_name, source_store_name,
			lifecycle_stage, sales_stage, intent_score, deal_amount_cents,
			COALESCE(to_char(next_follow_up_at, 'YYYY-MM-DD HH24:MI'), ''),
			updated_at
		FROM customers
		`+where+`
		ORDER BY updated_at DESC, id DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		queryArgs...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	customers := []Customer{}
	ids := []string{}
	for rows.Next() {
		var customer Customer
		var updatedAt time.Time
		var dealAmountCents int64
		if err := rows.Scan(&customer.ID, &customer.Name, &customer.Wecom, &customer.Mobile, &customer.Owner, &customer.SourceStore, &customer.Stage, &customer.SalesStage, &customer.IntentScore, &dealAmountCents, &customer.NextFollowUp, &updatedAt); err != nil {
			return err
		}
		customer.Source = customer.SourceStore
		customer.TagGroup = "行为标签 / 生命周期标签"
		customer.LastActive = formatBusinessTime(updatedAt, "2006-01-02 15:04")
		customer.DealAmount = formatCNYCents(dealAmountCents)
		customers = append(customers, customer)
		ids = append(ids, customer.ID)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	tags, err := api.customerTagsDB(ctx, ids)
	if err != nil {
		return err
	}
	for i := range customers {
		customers[i].Tags = tags[customers[i].ID]
	}
	if !paged {
		return api.writeCachedJSON(w, r, cacheKey, customers)
	}
	return api.writeCachedJSON(w, r, cacheKey, PageResult[Customer]{
		Data: customers,
		Page: makePageMeta(page, pageSize, total),
	})
}

func (api *API) customerActionDBHandler(w http.ResponseWriter, r *http.Request, parts []string) error {
	customerID := parts[0]
	if len(parts) == 1 {
		cacheKey := requestCacheKey("customers", r)
		if api.serveCachedJSON(w, r, cacheKey) {
			return nil
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		customer, err := api.customerDetailDB(ctx, customerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errNotFound
			}
			return err
		}
		return api.writeCachedJSON(w, r, cacheKey, customer)
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPatch {
		return errNotFound
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	customer, err := api.customerDB(ctx, customerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}

	switch parts[1] {
	case "touch":
		var payload struct {
			Content string `json:"content"`
			Method  string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.Method == "" {
			payload.Method = "企微单聊待办"
		}
		if err := api.customerTouchDB(ctx, customer, payload.Method, payload.Content); err != nil {
			return err
		}
	case "followups":
		var payload struct {
			Detail       string `json:"detail"`
			NextFollowUp string `json:"nextFollowUp"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if err := api.customerFollowupDB(ctx, customer, payload.Detail, payload.NextFollowUp); err != nil {
			return err
		}
	case "relations":
		if len(parts) == 2 {
			var payload struct {
				GuideID string `json:"guideId"`
				Guide   string `json:"guide"`
				Code    string `json:"code"`
				Store   string `json:"store"`
			}
			if err := decode(r, &payload); err != nil {
				return err
			}
			if err := api.customerRelationAddDB(ctx, customer, payload.GuideID, payload.Guide, payload.Code, payload.Store); err != nil {
				return err
			}
			break
		}
		if len(parts) < 4 {
			return errNotFound
		}
		if err := api.customerRelationActionDB(ctx, customer, parts[2], parts[3]); err != nil {
			return err
		}
	case "lead":
		if err := api.customerLeadDB(ctx, customer); err != nil {
			return err
		}
	case "order":
		var payload struct {
			Amount string `json:"amount"`
			Source string `json:"source"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.Amount == "" {
			payload.Amount = "¥1,280"
		}
		if err := api.customerOrderDB(ctx, customer, payload.Amount, payload.Source); err != nil {
			return err
		}
	case "lost":
		var payload struct {
			Reason string `json:"reason"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if err := api.customerLostDB(ctx, customer, payload.Reason); err != nil {
			return err
		}
	default:
		return errNotFound
	}

	api.invalidateCache("summary:", "customers:", "tags:", "tag-groups:", "tag-customers:")
	updated, err := api.customerDetailDB(ctx, customerID)
	if err != nil {
		return err
	}
	return writeJSON(w, updated)
}

func customerDBFilters(r *http.Request) (string, []any) {
	query := r.URL.Query()
	args := []any{}
	conditions := []string{}
	if keyword := strings.TrimSpace(query.Get("keyword")); keyword != "" {
		args = append(args, "%"+keyword+"%")
		placeholder := "$" + strconv.Itoa(len(args))
		conditions = append(conditions, "(name ILIKE "+placeholder+" OR wecom_name ILIKE "+placeholder+" OR mobile_masked ILIKE "+placeholder+" OR source_store_name ILIKE "+placeholder+")")
	}
	if guide := strings.TrimSpace(query.Get("guide")); guide != "" && guide != "全部导购" {
		args = append(args, guide)
		conditions = append(conditions, "owner_staff_name = $"+strconv.Itoa(len(args)))
	}
	if stage := strings.TrimSpace(query.Get("stage")); stage != "" && stage != "全部阶段" {
		args = append(args, stage)
		conditions = append(conditions, "lifecycle_stage = $"+strconv.Itoa(len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (api *API) customerDB(ctx context.Context, id string) (Customer, error) {
	var customer Customer
	var updatedAt time.Time
	var dealAmountCents int64
	err := api.db.QueryRowContext(ctx, `
		SELECT
			id, name, wecom_name, mobile_masked, owner_staff_name, source_store_name,
			lifecycle_stage, sales_stage, intent_score, deal_amount_cents,
			COALESCE(to_char(next_follow_up_at, 'YYYY-MM-DD HH24:MI'), ''),
			updated_at
		FROM customers
		WHERE id = $1
	`, id).Scan(&customer.ID, &customer.Name, &customer.Wecom, &customer.Mobile, &customer.Owner, &customer.SourceStore, &customer.Stage, &customer.SalesStage, &customer.IntentScore, &dealAmountCents, &customer.NextFollowUp, &updatedAt)
	if err != nil {
		return Customer{}, err
	}
	customer.Source = customer.SourceStore
	customer.TagGroup = "行为标签 / 生命周期标签"
	customer.LastActive = formatBusinessTime(updatedAt, "2006-01-02 15:04")
	customer.DealAmount = formatCNYCents(dealAmountCents)
	return customer, nil
}

func (api *API) customerDetailDB(ctx context.Context, customerID string) (Customer, error) {
	customer, err := api.customerDB(ctx, customerID)
	if err != nil {
		return Customer{}, err
	}
	relations, err := api.customerRelationsDB(ctx, customer.ID)
	if err != nil {
		return Customer{}, err
	}
	tags, err := api.customerTagsDB(ctx, []string{customer.ID})
	if err != nil {
		return Customer{}, err
	}
	timeline, err := api.customerEventsDB(ctx, customer.ID, 50)
	if err != nil {
		return Customer{}, err
	}
	customer.Relations = relations
	customer.Tags = tags[customer.ID]
	customer.Timeline = timeline
	if len(customer.Timeline) == 0 {
		customer.Timeline = []AuditEvent{audit(customer.LastActive, "数据库读取客户详情", "已加载客户主档、标签和导购关系", "系统", "客户："+customer.Name, "PostgreSQL 模式")}
	}
	return customer, nil
}

func (api *API) customerTagsDB(ctx context.Context, ids []string) (map[string][]string, error) {
	result := map[string][]string{}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := api.db.QueryContext(ctx, `
		SELECT ct.customer_id, t.name
		FROM customer_tags ct
		JOIN tags t ON t.id = ct.tag_id
		WHERE ct.customer_id = ANY($1)
		ORDER BY ct.customer_id, t.name
	`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var customerID, tag string
		if err := rows.Scan(&customerID, &tag); err != nil {
			return nil, err
		}
		result[customerID] = append(result[customerID], tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (api *API) customerRelationsDB(ctx context.Context, customerID string) ([]Relation, error) {
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, staff_id, staff_name, via_code_id, store_id,
			to_char(linked_at, 'YYYY-MM-DD'),
			COALESCE(to_char(ended_at, 'YYYY-MM-DD'), '-'),
			is_main_follow
		FROM customer_staff_relations
		WHERE customer_id = $1
		ORDER BY is_main_follow DESC, linked_at DESC
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	relations := []Relation{}
	for rows.Next() {
		var relation Relation
		if err := rows.Scan(&relation.ID, &relation.GuideID, &relation.Guide, &relation.Code, &relation.Store, &relation.LinkedAt, &relation.EndedAt, &relation.Main); err != nil {
			return nil, err
		}
		relations = append(relations, relation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return relations, nil
}

func (api *API) customerEventsDB(ctx context.Context, customerID string, limit int) ([]AuditEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, event_type, source, object_id, payload::text,
			to_char(occurred_at, 'YYYY-MM-DD HH24:MI')
		FROM customer_events
		WHERE customer_id = $1
		ORDER BY occurred_at DESC, created_at DESC
		LIMIT $2
	`, customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []AuditEvent{}
	for rows.Next() {
		var id, eventType, source, objectID, payloadText, occurredAt string
		if err := rows.Scan(&id, &eventType, &source, &objectID, &payloadText, &occurredAt); err != nil {
			return nil, err
		}
		payload := map[string]any{}
		_ = json.Unmarshal([]byte(payloadText), &payload)
		event := audit(occurredAt, customerEventAction(eventType), customerEventResult(eventType), stringPayload(payload, "operator"), stringPayload(payload, "people"), customerEventDetail(eventType, source, objectID, payload))
		event.ID = id
		if event.Operator == "" {
			event.Operator = "系统"
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (api *API) customerTouchDB(ctx context.Context, customer Customer, method, content string) error {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE customers SET updated_at = now() WHERE id = $1`, customer.ID); err != nil {
		return err
	}
	payload := map[string]any{
		"method":   method,
		"content":  content,
		"operator": "运营",
		"people":   "客户：" + customer.Name + "；导购：" + customer.Owner,
	}
	if err := insertCustomerEventDB(ctx, tx, customer.ID, "touch_task", "api", "", payload); err != nil {
		return err
	}
	return tx.Commit()
}

func (api *API) customerFollowupDB(ctx context.Context, customer Customer, detail, nextFollowUp string) error {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if nextFollowUp != "" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE customers
			SET next_follow_up_at = $2::timestamptz,
				updated_at = now()
			WHERE id = $1
		`, customer.ID, nextFollowUp); err != nil {
			return err
		}
	} else if _, err := tx.ExecContext(ctx, `UPDATE customers SET updated_at = now() WHERE id = $1`, customer.ID); err != nil {
		return err
	}
	payload := map[string]any{
		"detail":       detail,
		"nextFollowUp": nextFollowUp,
		"operator":     defaultString(customer.Owner, "运营"),
		"people":       "客户：" + customer.Name,
	}
	if err := insertCustomerEventDB(ctx, tx, customer.ID, "followup", "api", "", payload); err != nil {
		return err
	}
	return tx.Commit()
}

func (api *API) customerRelationAddDB(ctx context.Context, customer Customer, guideID, guideName, code, store string) error {
	guideName = strings.TrimSpace(guideName)
	if guideName == "" {
		return fmt.Errorf("%w: guide is required", errBadRequest)
	}
	if guideID == "" {
		guideID = guideName
	}
	if store == "" {
		store = customer.SourceStore
	}
	if code == "" {
		code = relationCodePrefix(store) + "-" + guideName
	}
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	relationID := newID("r")
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO customer_staff_relations (id, customer_id, staff_id, staff_name, via_code_id, store_id, is_main_follow)
		VALUES ($1, $2, $3, $4, $5, $6, false)
	`, relationID, customer.ID, guideID, guideName, code, store); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE customers SET updated_at = now() WHERE id = $1`, customer.ID); err != nil {
		return err
	}
	payload := map[string]any{
		"operator": "运营",
		"people":   "客户：" + customer.Name + "；导购：" + guideName,
		"detail":   "历史来源不回改。",
	}
	if err := insertCustomerEventDB(ctx, tx, customer.ID, "relation_add", "api", relationID, payload); err != nil {
		return err
	}
	return tx.Commit()
}

func (api *API) customerRelationActionDB(ctx context.Context, customer Customer, relationID, action string) error {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var guideID string
	var guideName string
	var isMain bool
	var status string
	err = tx.QueryRowContext(ctx, `
		SELECT staff_id, staff_name, is_main_follow, status
		FROM customer_staff_relations
		WHERE id = $1 AND customer_id = $2
	`, relationID, customer.ID).Scan(&guideID, &guideName, &isMain, &status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}

	eventType := ""
	detail := ""
	switch action {
	case "set-main":
		if status != "active" {
			return fmt.Errorf("%w: ended relation cannot be main guide", errBadRequest)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE customer_staff_relations SET is_main_follow = false WHERE customer_id = $1`, customer.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE customer_staff_relations SET is_main_follow = true WHERE id = $1`, relationID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE customers
			SET owner_staff_id = $2,
				owner_staff_name = $3,
				updated_at = now()
			WHERE id = $1
		`, customer.ID, guideID, guideName); err != nil {
			return err
		}
		eventType = "relation_set_main"
		detail = "之前的主跟进自动变为否。"
	case "end":
		if isMain {
			return fmt.Errorf("%w: main guide relation cannot be ended", errBadRequest)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE customer_staff_relations
			SET status = 'ended',
				ended_at = now()
			WHERE id = $1 AND customer_id = $2
		`, relationID, customer.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE customers SET updated_at = now() WHERE id = $1`, customer.ID); err != nil {
			return err
		}
		eventType = "relation_end"
		detail = "系统留痕，历史来源不回改。"
	default:
		return errNotFound
	}

	payload := map[string]any{
		"operator": "运营",
		"people":   "客户：" + customer.Name + "；导购：" + guideName,
		"detail":   detail,
	}
	if err := insertCustomerEventDB(ctx, tx, customer.ID, eventType, "api", relationID, payload); err != nil {
		return err
	}
	return tx.Commit()
}

func (api *API) customerLeadDB(ctx context.Context, customer Customer) error {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE customers
		SET sales_stage = '已建线索',
			lifecycle_stage = '新客待转化',
			updated_at = now()
		WHERE id = $1
	`, customer.ID); err != nil {
		return err
	}
	payload := map[string]any{
		"operator": "运营",
		"people":   "客户：" + customer.Name + "；负责人：" + customer.Owner,
		"detail":   "来源行为：" + first(customer.Signals),
	}
	if err := insertCustomerEventDB(ctx, tx, customer.ID, "lead", "api", "", payload); err != nil {
		return err
	}
	return tx.Commit()
}

func (api *API) customerOrderDB(ctx context.Context, customer Customer, amount, source string) error {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	amountCents := parseCNYCents(amount)
	if _, err := tx.ExecContext(ctx, `
		UPDATE customers
		SET deal_amount_cents = $2,
			sales_stage = '已成交',
			lifecycle_stage = '复购培育',
			updated_at = now()
		WHERE id = $1
	`, customer.ID, amountCents); err != nil {
		return err
	}
	tagID, err := ensureDBTag(ctx, tx, "已成交")
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO customer_tags (customer_id, tag_id, source)
		VALUES ($1, $2, 'customer_order')
		ON CONFLICT (customer_id, tag_id) DO UPDATE SET source = EXCLUDED.source
	`, customer.ID, tagID); err != nil {
		return err
	}
	payload := map[string]any{
		"amount":   amount,
		"source":   source,
		"operator": "运营",
		"people":   "客户：" + customer.Name + "；导购：" + customer.Owner,
		"detail":   "成交来源：" + source + "；金额：" + amount,
	}
	if err := insertCustomerEventDB(ctx, tx, customer.ID, "order", "api", tagID, payload); err != nil {
		return err
	}
	return tx.Commit()
}

func (api *API) customerLostDB(ctx context.Context, customer Customer, reason string) error {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE customers
		SET lifecycle_stage = '已流失',
			updated_at = now()
		WHERE id = $1
	`, customer.ID); err != nil {
		return err
	}
	tagID, err := ensureDBTag(ctx, tx, "已流失")
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO customer_tags (customer_id, tag_id, source)
		VALUES ($1, $2, 'customer_lost')
		ON CONFLICT (customer_id, tag_id) DO UPDATE SET source = EXCLUDED.source
	`, customer.ID, tagID); err != nil {
		return err
	}
	payload := map[string]any{
		"operator": "运营",
		"people":   "客户：" + customer.Name,
		"detail":   reason,
	}
	if err := insertCustomerEventDB(ctx, tx, customer.ID, "lost", "api", tagID, payload); err != nil {
		return err
	}
	return tx.Commit()
}

func insertCustomerEventDB(ctx context.Context, tx *sql.Tx, customerID, eventType, source, objectID string, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO customer_events (id, customer_id, event_type, source, object_id, payload)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
	`, newID("cev"), customerID, eventType, source, objectID, string(body))
	return err
}
