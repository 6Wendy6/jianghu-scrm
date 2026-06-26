package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (api *API) batchTags(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.batchTagsDB(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	var payload BatchTagTaskPayload
	if err := decode(r, &payload); err != nil {
		return err
	}
	if len(payload.CustomerIDs) == 0 {
		return fmt.Errorf("%w: customerIds is required", errBadRequest)
	}
	updated := 0
	for i := range api.customers {
		if contains(payload.CustomerIDs, api.customers[i].ID) {
			api.customers[i].Tags = appendUnique(api.customers[i].Tags, payload.Tags...)
			api.customers[i].Timeline = prependAudit(api.customers[i].Timeline, audit(stamp(), "批量打标签", "系统标签已更新", "运营", "客户："+api.customers[i].Name, strings.Join(payload.Tags, "、")))
			updated++
		}
	}
	return writeJSON(w, map[string]any{"updated": updated})
}

func (api *API) batchTagsDB(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return errNotFound
	}
	var payload BatchTagTaskPayload
	if err := decode(r, &payload); err != nil {
		return err
	}
	if len(payload.CustomerIDs) == 0 {
		return fmt.Errorf("%w: customerIds is required", errBadRequest)
	}
	if len(payload.Tags) == 0 {
		return fmt.Errorf("%w: tags is required", errBadRequest)
	}
	if payload.Operator == "" {
		payload.Operator = "运营"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tagIDs := []string{}
	for _, tagName := range payload.Tags {
		tagName = strings.TrimSpace(tagName)
		if tagName == "" {
			continue
		}
		tagID, err := ensureDBTag(ctx, tx, tagName)
		if err != nil {
			return err
		}
		tagIDs = append(tagIDs, tagID)
	}
	if len(tagIDs) == 0 {
		return fmt.Errorf("%w: tags is required", errBadRequest)
	}

	updated := 0
	for _, customerID := range payload.CustomerIDs {
		var customerName string
		err := tx.QueryRowContext(ctx, `
			UPDATE customers
			SET updated_at = now()
			WHERE id = $1
			RETURNING name
		`, customerID).Scan(&customerName)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return err
		}
		for _, tagID := range tagIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO customer_tags (customer_id, tag_id, source)
				VALUES ($1, $2, 'sync_batch')
				ON CONFLICT (customer_id, tag_id) DO UPDATE SET source = EXCLUDED.source
			`, customerID, tagID); err != nil {
				return err
			}
		}
		if err := insertCustomerEventDB(ctx, tx, customerID, "batch_customer_tags", "api", "", map[string]any{
			"tags":     payload.Tags,
			"operator": payload.Operator,
			"people":   "客户：" + customerName,
		}); err != nil {
			return err
		}
		updated++
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	api.invalidateCache("summary:", "customers:", "tags:", "tag-groups:", "tag-customers:")
	return writeJSON(w, map[string]any{"updated": updated})
}

func (api *API) enqueueBatchTags(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.enqueueBatchTagsDB(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method != http.MethodPost {
		return errNotFound
	}
	var payload BatchTagTaskPayload
	if err := decode(r, &payload); err != nil {
		return err
	}
	if len(payload.CustomerIDs) == 0 {
		return fmt.Errorf("%w: customerIds is required", errBadRequest)
	}
	if len(payload.Tags) == 0 {
		return fmt.Errorf("%w: tags is required", errBadRequest)
	}
	if payload.Operator == "" {
		payload.Operator = "运营"
	}
	task, err := api.enqueueBatchTagTaskLocked(payload)
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusAccepted)
	return writeJSON(w, map[string]any{"status": "queued", "task": task})
}

func (api *API) enqueueBatchTagTaskLocked(payload BatchTagTaskPayload) (TaskRecord, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return TaskRecord{}, err
	}
	now := businessNow().Format(time.RFC3339)
	task := TaskRecord{
		ID:         newID("task"),
		Type:       "batch_customer_tags",
		Status:     "queued",
		TotalCount: len(payload.CustomerIDs),
		Payload:    body,
		CreatedBy:  payload.Operator,
		CreatedAt:  now,
	}
	api.tasks = append([]TaskRecord{task}, api.tasks...)
	for _, customerID := range payload.CustomerIDs {
		item := TaskItemRecord{
			ID:        newID("item"),
			TaskID:    task.ID,
			TargetID:  customerID,
			Status:    "queued",
			Result:    json.RawMessage(`{}`),
			UpdatedAt: now,
		}
		api.taskItems = append(api.taskItems, item)
	}
	return task, nil
}

func (api *API) enqueueBatchTagsDB(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return errNotFound
	}
	var payload BatchTagTaskPayload
	if err := decode(r, &payload); err != nil {
		return err
	}
	task, err := api.enqueueBatchTagTaskDB(r.Context(), payload)
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusAccepted)
	return writeJSON(w, map[string]any{"status": "queued", "task": task})
}

func (api *API) enqueueBatchTagTaskDB(ctx context.Context, payload BatchTagTaskPayload) (TaskRecord, error) {
	if len(payload.CustomerIDs) == 0 {
		return TaskRecord{}, fmt.Errorf("%w: customerIds is required", errBadRequest)
	}
	if len(payload.Tags) == 0 {
		return TaskRecord{}, fmt.Errorf("%w: tags is required", errBadRequest)
	}
	if payload.Operator == "" {
		payload.Operator = "运营"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return TaskRecord{}, err
	}
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return TaskRecord{}, err
	}
	defer tx.Rollback()

	now := businessNow().Format(time.RFC3339)
	task := TaskRecord{
		ID:         newID("task"),
		Type:       "batch_customer_tags",
		Status:     "queued",
		TotalCount: len(payload.CustomerIDs),
		Payload:    body,
		CreatedBy:  payload.Operator,
		CreatedAt:  now,
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tasks (id, task_type, status, total_count, payload, created_by)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)
	`, task.ID, task.Type, task.Status, task.TotalCount, string(task.Payload), task.CreatedBy); err != nil {
		return TaskRecord{}, err
	}
	for _, customerID := range payload.CustomerIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO task_items (id, task_id, target_id, status, result)
			VALUES ($1, $2, $3, 'queued', '{}'::jsonb)
		`, newID("item"), task.ID, customerID); err != nil {
			return TaskRecord{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return TaskRecord{}, err
	}
	api.invalidateCache("summary:")
	return task, nil
}
