package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (api *API) syncHandoverDB(w http.ResponseWriter, r *http.Request, storeID string) error {
	if r.Method != http.MethodPost {
		return errNotFound
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if _, err := api.storeDB(ctx, storeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO store_handover_syncs (store_id, synced, synced_at, source, updated_at)
		VALUES ($1, true, now(), 'api', now())
		ON CONFLICT (store_id) DO UPDATE SET
			synced = true,
			synced_at = now(),
			source = EXCLUDED.source,
			updated_at = now()
	`, storeID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE store_handover_items
		SET status = '待处理',
			updated_at = now()
		WHERE store_id = $1 AND status = '待同步'
	`, storeID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO store_handover_items (
			id, store_id, customer_id, customer_name, reason,
			current_owner, required_action, status
		)
		SELECT
			'handover-' || c.id,
			$1,
			c.id,
			c.name,
			COALESCE(NULLIF(g.name, ''), c.owner_staff_name) || '离职/调店',
			c.owner_staff_name,
			'强制指派',
			'待处理'
		FROM customers c
		LEFT JOIN guides g ON g.id = c.owner_staff_id
		WHERE c.lifecycle_stage = '待交接'
			AND COALESCE(c.source_store_id, '') = $1
			AND NOT EXISTS (
				SELECT 1
				FROM store_handover_items existing
				WHERE existing.store_id = $1
					AND existing.customer_id = c.id
					AND existing.status NOT IN ('已完成', '已忽略')
			)
		ON CONFLICT (id) DO UPDATE SET
			customer_name = EXCLUDED.customer_name,
			current_owner = EXCLUDED.current_owner,
			status = CASE
				WHEN store_handover_items.status = '已完成' THEN store_handover_items.status
				ELSE EXCLUDED.status
			END,
			updated_at = now()
	`, storeID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	api.invalidateTodayMetricSnapshotsDB(ctx)
	api.invalidateCache("summary:", "customers:")
	items, err := api.handoverItemsDB(ctx, storeID, false)
	if err != nil {
		return err
	}
	return writeJSON(w, items)
}

func (api *API) submitHandoverDB(w http.ResponseWriter, r *http.Request, storeID string) error {
	if r.Method != http.MethodPost && r.Method != http.MethodPatch {
		return errNotFound
	}
	var payload struct {
		Items []HandoverItem `json:"items"`
	}
	if err := decode(r, &payload); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var synced bool
	err := api.db.QueryRowContext(ctx, `SELECT synced FROM store_handover_syncs WHERE store_id = $1`, storeID).Scan(&synced)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: sync wecom leave data first", errBadRequest)
		}
		return err
	}
	if !synced {
		return fmt.Errorf("%w: sync wecom leave data first", errBadRequest)
	}

	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, item := range payload.Items {
		if item.Receiver == "" && strings.Contains(item.RequiredAction, "强制") {
			return fmt.Errorf("%w: receiver is required for forced handover", errBadRequest)
		}
		stored, err := handoverItemDBTx(ctx, tx, storeID, item)
		if err != nil {
			return err
		}
		receiver := strings.TrimSpace(item.Receiver)
		if receiver == "" {
			if _, err := tx.ExecContext(ctx, `
				UPDATE store_handover_items
				SET status = '已忽略',
					updated_at = now(),
					submitted_at = now()
				WHERE id = $1 AND store_id = $2
			`, stored.ID, storeID); err != nil {
				return err
			}
			continue
		}
		guideID, guideName, err := resolveGuideForHandover(ctx, tx, storeID, receiver)
		if err != nil {
			return err
		}
		customerName, updated, err := applyCustomerHandoverDB(ctx, tx, storeID, stored, guideID, guideName)
		if err != nil {
			return err
		}
		status := "已完成"
		if !updated {
			status = "客户未入库"
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE store_handover_items
			SET receiver = $3,
				status = $4,
				updated_at = now(),
				submitted_at = now()
			WHERE id = $1 AND store_id = $2
		`, stored.ID, storeID, guideName, status); err != nil {
			return err
		}
		if updated {
			if err := insertCustomerEventDB(ctx, tx, stored.CustomerID, "handover_submit", "api", stored.ID, map[string]any{
				"operator": "店长",
				"people":   "客户：" + customerName + "；接收人：" + guideName,
				"detail":   stored.Reason,
			}); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	api.invalidateTodayMetricSnapshotsDB(ctx)
	api.invalidateCache("summary:", "customers:", "tag-customers:")
	items, err := api.handoverItemsDB(ctx, storeID, true)
	if err != nil {
		return err
	}
	return writeJSON(w, map[string]any{"status": "completed", "handover": items})
}

func (api *API) handoverItemsDB(ctx context.Context, storeID string, openOnly bool) ([]HandoverItem, error) {
	where := "WHERE store_id = $1"
	if openOnly {
		where += " AND status NOT IN ('已完成', '已忽略', '客户未入库')"
	}
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, customer_id, customer_name, reason, current_owner,
			required_action, receiver, status
		FROM store_handover_items
		`+where+`
		ORDER BY updated_at DESC, id DESC
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []HandoverItem{}
	for rows.Next() {
		var item HandoverItem
		if err := rows.Scan(&item.ID, &item.CustomerID, &item.Customer, &item.Reason, &item.CurrentOwner, &item.RequiredAction, &item.Receiver, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func handoverItemDBTx(ctx context.Context, tx *sql.Tx, storeID string, item HandoverItem) (HandoverItem, error) {
	where := "id = $1"
	arg := item.ID
	if arg == "" {
		where = "customer_id = $1"
		arg = item.CustomerID
	}
	if arg == "" {
		return HandoverItem{}, fmt.Errorf("%w: handover item id is required", errBadRequest)
	}
	var stored HandoverItem
	err := tx.QueryRowContext(ctx, `
		SELECT id, customer_id, customer_name, reason, current_owner,
			required_action, receiver, status
		FROM store_handover_items
		WHERE store_id = $2 AND `+where+`
		FOR UPDATE
	`, arg, storeID).Scan(&stored.ID, &stored.CustomerID, &stored.Customer, &stored.Reason, &stored.CurrentOwner, &stored.RequiredAction, &stored.Receiver, &stored.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HandoverItem{}, errNotFound
		}
		return HandoverItem{}, err
	}
	if item.RequiredAction != "" {
		stored.RequiredAction = item.RequiredAction
	}
	if item.Reason != "" {
		stored.Reason = item.Reason
	}
	if item.CurrentOwner != "" {
		stored.CurrentOwner = item.CurrentOwner
	}
	if item.Customer != "" {
		stored.Customer = item.Customer
	}
	if item.Receiver != "" {
		stored.Receiver = item.Receiver
	}
	return stored, nil
}

func resolveGuideForHandover(ctx context.Context, tx *sql.Tx, storeID, receiver string) (string, string, error) {
	var guideID, guideName string
	err := tx.QueryRowContext(ctx, `
		SELECT id, name
		FROM guides
		WHERE store_id = $1 AND (id = $2 OR name = $2)
		ORDER BY updated_at DESC
		LIMIT 1
	`, storeID, receiver).Scan(&guideID, &guideName)
	if err == nil {
		return guideID, guideName, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", "", err
	}
	return receiver, receiver, nil
}

func applyCustomerHandoverDB(ctx context.Context, tx *sql.Tx, storeID string, item HandoverItem, guideID, guideName string) (string, bool, error) {
	if item.CustomerID == "" {
		return item.Customer, false, nil
	}
	var customerName string
	err := tx.QueryRowContext(ctx, `
		UPDATE customers
		SET owner_staff_id = $2,
			owner_staff_name = $3,
			lifecycle_stage = '新客待转化',
			updated_at = now()
		WHERE id = $1
		RETURNING name
	`, item.CustomerID, guideID, guideName).Scan(&customerName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return item.Customer, false, nil
		}
		return "", false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE customer_staff_relations
		SET status = 'ended',
			ended_at = COALESCE(ended_at, now()),
			is_main_follow = false
		WHERE customer_id = $1 AND status = 'active' AND is_main_follow = true
	`, item.CustomerID); err != nil {
		return "", false, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO customer_staff_relations (
			id, customer_id, staff_id, staff_name, via_code_id, store_id, is_main_follow, status
		) VALUES ($1, $2, $3, $4, $5, $6, true, 'active')
	`, newID("r"), item.CustomerID, guideID, guideName, "handover:"+item.ID, storeID); err != nil {
		return "", false, err
	}
	return customerName, true, nil
}

func (api *API) invalidateTodayMetricSnapshotsDB(ctx context.Context) {
	if api.db == nil {
		return
	}
	_, _ = api.db.ExecContext(ctx, `
		DELETE FROM metric_snapshots
		WHERE metric_date = CURRENT_DATE
			AND scope_type = 'global'
			AND scope_id = 'all'
	`)
}

func (api *API) syncHandover(w http.ResponseWriter, r *http.Request, storeID string) error {
	if api.db != nil {
		return api.syncHandoverDB(w, r, storeID)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	api.wecomSynced = true
	for i := range api.handover {
		if api.handover[i].Status == "待同步" {
			api.handover[i].Status = "待处理"
		}
	}
	return writeJSON(w, api.handover)
}

func (api *API) submitHandover(w http.ResponseWriter, r *http.Request, storeID string) error {
	if api.db != nil {
		return api.submitHandoverDB(w, r, storeID)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if !api.wecomSynced {
		return fmt.Errorf("%w: sync wecom leave data first", errBadRequest)
	}
	var payload struct {
		Items []HandoverItem `json:"items"`
	}
	if err := decode(r, &payload); err != nil {
		return err
	}
	for _, item := range payload.Items {
		if item.Receiver == "" && strings.Contains(item.RequiredAction, "强制") {
			return fmt.Errorf("%w: receiver is required for forced handover", errBadRequest)
		}
		for i := range api.customers {
			if api.customers[i].ID == item.CustomerID && item.Receiver != "" {
				api.customers[i].Owner = item.Receiver
				api.customers[i].Stage = "新客待转化"
				api.customers[i].Timeline = prependAudit(api.customers[i].Timeline, audit(stamp(), "提交导购交接", "主跟进导购已更新", "店长", "客户："+api.customers[i].Name+"；接收人："+item.Receiver, item.Reason))
			}
		}
	}
	api.handover = []HandoverItem{}
	return writeJSON(w, map[string]any{"status": "completed", "handover": api.handover})
}
