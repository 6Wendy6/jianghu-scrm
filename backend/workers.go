package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

func (api *API) runWorkers(ctx context.Context, cfg Config) {
	interval := cfg.WorkerInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	batchSize := cfg.WorkerBatchSize
	if batchSize <= 0 {
		batchSize = 20
	}
	maxAttempts := cfg.WorkerMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	retryDelay := cfg.WorkerRetryDelay
	if retryDelay <= 0 {
		retryDelay = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Printf("AI SCRM workers started interval=%s batch_size=%d max_attempts=%d retry_delay=%s\n", interval, batchSize, maxAttempts, retryDelay)
	for {
		select {
		case <-ctx.Done():
			log.Printf("AI SCRM workers stopped\n")
			return
		case <-ticker.C:
			api.processQueuedWork(batchSize, maxAttempts, retryDelay)
		}
	}
}

func (api *API) processQueuedWork(batchSize, maxAttempts int, retryDelay time.Duration) {
	if api.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := api.processQueuedDBWork(ctx, batchSize, maxAttempts, retryDelay); err != nil {
			log.Printf("worker db error: %v\n", err)
		}
		return
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	api.processQueuedEventsLocked(batchSize)
	api.processQueuedTaskItemsLocked(batchSize)
}

func (api *API) processQueuedDBWork(ctx context.Context, batchSize, maxAttempts int, retryDelay time.Duration) error {
	if err := api.processQueuedDBEvents(ctx, batchSize); err != nil {
		return err
	}
	return api.processQueuedDBTaskItems(ctx, batchSize, maxAttempts, retryDelay)
}

func (api *API) processQueuedDBEvents(ctx context.Context, batchSize int) error {
	result, err := api.db.ExecContext(ctx, `
		WITH picked AS (
			SELECT id
			FROM event_inbox
			WHERE status = 'queued'
				AND next_attempt_at <= now()
			ORDER BY received_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE event_inbox e
		SET status = 'processed',
			attempts = e.attempts + 1,
			processed_at = now()
		FROM picked
		WHERE e.id = picked.id
	`, batchSize)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		api.invalidateCache("summary:")
	}
	return nil
}

func (api *API) processQueuedDBTaskItems(ctx context.Context, batchSize, maxAttempts int, retryDelay time.Duration) error {
	rows, err := api.db.QueryContext(ctx, `
		SELECT ti.id, ti.task_id, ti.target_id, t.task_type, t.payload::text, t.created_by
		FROM task_items ti
		JOIN tasks t ON t.id = ti.task_id
		WHERE ti.status = 'queued'
			AND ti.next_attempt_at <= now()
			AND t.status IN ('queued', 'running')
		ORDER BY t.created_at, ti.id
		LIMIT $1
	`, batchSize)
	if err != nil {
		return err
	}
	defer rows.Close()

	type queuedItem struct {
		ID        string
		TaskID    string
		TargetID  string
		TaskType  string
		Payload   json.RawMessage
		CreatedBy string
	}
	items := []queuedItem{}
	for rows.Next() {
		var item queuedItem
		var payload string
		if err := rows.Scan(&item.ID, &item.TaskID, &item.TargetID, &item.TaskType, &payload, &item.CreatedBy); err != nil {
			return err
		}
		item.Payload = json.RawMessage(payload)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, item := range items {
		if err := api.processQueuedDBTaskItem(ctx, item.ID, item.TaskID, item.TargetID, item.TaskType, item.Payload, item.CreatedBy, maxAttempts, retryDelay); err != nil {
			log.Printf("worker task item error task_id=%s item_id=%s error=%v\n", item.TaskID, item.ID, err)
		}
	}
	return nil
}

func (api *API) processQueuedDBTaskItem(ctx context.Context, itemID, taskID, targetID, taskType string, payload json.RawMessage, createdBy string, maxAttempts int, retryDelay time.Duration) error {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var attempts int
	err = tx.QueryRowContext(ctx, `
		UPDATE task_items
		SET status = 'running',
			attempts = attempts + 1,
			updated_at = now()
		WHERE id = $1 AND status = 'queued'
		RETURNING attempts
	`, itemID).Scan(&attempts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = 'running',
			started_at = COALESCE(started_at, now())
		WHERE id = $1 AND status = 'queued'
	`, taskID); err != nil {
		return err
	}

	var itemErr error
	switch taskType {
	case "batch_customer_tags":
		itemErr = processDBBatchTagItem(ctx, tx, taskID, targetID, payload, createdBy)
	default:
		itemErr = fmt.Errorf("unsupported task type")
	}

	if itemErr != nil {
		if attempts >= maxAttempts {
			if _, err := tx.ExecContext(ctx, `
				UPDATE task_items
				SET status = 'dead_letter',
					last_error = $2,
					updated_at = now()
				WHERE id = $1
			`, itemID, itemErr.Error()); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE tasks
				SET failed_count = failed_count + 1,
					last_error = $2
				WHERE id = $1
			`, taskID, itemErr.Error()); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
				UPDATE task_items
				SET status = 'queued',
					last_error = $2,
					next_attempt_at = now() + (($3)::double precision * interval '1 second'),
					updated_at = now()
				WHERE id = $1
			`, itemID, itemErr.Error(), retryDelay.Seconds()); err != nil {
				return err
			}
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE task_items
			SET status = 'succeeded',
				result = '{"updated": true}'::jsonb,
				updated_at = now()
			WHERE id = $1
		`, itemID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE tasks
			SET success_count = success_count + 1
			WHERE id = $1
		`, taskID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE tasks
		SET status = CASE
				WHEN success_count + failed_count >= total_count AND failed_count = 0 THEN 'completed'
				WHEN success_count + failed_count >= total_count AND success_count = 0 THEN 'failed'
				WHEN success_count + failed_count >= total_count THEN 'partial_failed'
				ELSE status
			END,
			finished_at = CASE
				WHEN success_count + failed_count >= total_count THEN COALESCE(finished_at, now())
				ELSE finished_at
			END
		WHERE id = $1
	`, taskID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if itemErr == nil && taskType == "batch_customer_tags" {
		api.invalidateCache("summary:", "customers:", "tags:", "tag-groups:", "tag-customers:")
	}
	return nil
}

func processDBBatchTagItem(ctx context.Context, tx *sql.Tx, taskID, customerID string, rawPayload json.RawMessage, createdBy string) error {
	var payload BatchTagTaskPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return fmt.Errorf("invalid task payload")
	}
	if len(payload.Tags) == 0 {
		return fmt.Errorf("tags is required")
	}
	var customerName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM customers WHERE id = $1`, customerID).Scan(&customerName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("customer not found")
		}
		return err
	}
	operator := defaultString(payload.Operator, createdBy)
	for _, tagName := range payload.Tags {
		tagName = strings.TrimSpace(tagName)
		if tagName == "" {
			continue
		}
		tagID, err := ensureDBTag(ctx, tx, tagName)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO customer_tags (customer_id, tag_id, source)
			VALUES ($1, $2, $3)
			ON CONFLICT (customer_id, tag_id) DO UPDATE SET source = EXCLUDED.source
		`, customerID, tagID, "task:"+taskID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE customers SET updated_at = now() WHERE id = $1`, customerID); err != nil {
		return err
	}
	eventPayload, _ := json.Marshal(map[string]any{"tags": payload.Tags, "operator": operator})
	_, err := tx.ExecContext(ctx, `
		INSERT INTO customer_events (id, customer_id, event_type, source, object_id, payload)
		VALUES ($1, $2, 'batch_customer_tags', 'task_worker', $3, $4::jsonb)
	`, newID("cev"), customerID, taskID, string(eventPayload))
	if err != nil {
		return err
	}
	_ = customerName
	return nil
}

func ensureDBTag(ctx context.Context, tx *sql.Tx, tagName string) (string, error) {
	var tagID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE name = $1 ORDER BY created_at LIMIT 1`, tagName).Scan(&tagID)
	if err == nil {
		return tagID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tag_groups (id, name, scope, display_order)
		VALUES ('tg-auto-task', '任务自动标签', '全部门店', 99)
		ON CONFLICT (id) DO NOTHING
	`); err != nil {
		return "", err
	}
	tagID = newID("tag")
	_, err = tx.ExecContext(ctx, `
		INSERT INTO tags (id, tag_group_id, name, status)
		VALUES ($1, 'tg-auto-task', $2, 'active')
		ON CONFLICT (tag_group_id, name) DO UPDATE SET updated_at = now()
	`, tagID, tagName)
	if err != nil {
		return "", err
	}
	if err := tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE name = $1 ORDER BY created_at LIMIT 1`, tagName).Scan(&tagID); err != nil {
		return "", err
	}
	return tagID, nil
}

func (api *API) processQueuedEventsLocked(limit int) {
	processed := 0
	now := businessNow().Format(time.RFC3339)
	for i := range api.events {
		if processed >= limit {
			return
		}
		if api.events[i].Status != "queued" {
			continue
		}
		api.events[i].Status = "processed"
		api.events[i].Attempts++
		api.events[i].ProcessedAt = now
		processed++
	}
}

func (api *API) processQueuedTaskItemsLocked(limit int) {
	processed := 0
	now := businessNow().Format(time.RFC3339)
	for taskIndex := range api.tasks {
		if processed >= limit {
			return
		}
		task := &api.tasks[taskIndex]
		if task.Status != "queued" && task.Status != "running" {
			continue
		}
		if task.Status == "queued" {
			task.Status = "running"
			task.StartedAt = now
		}
		for itemIndex := range api.taskItems {
			if processed >= limit {
				api.finishTaskIfDoneLocked(task)
				return
			}
			item := &api.taskItems[itemIndex]
			if item.TaskID != task.ID || item.Status != "queued" {
				continue
			}
			api.processTaskItemLocked(task, item)
			processed++
		}
		api.finishTaskIfDoneLocked(task)
	}
}

func (api *API) processTaskItemLocked(task *TaskRecord, item *TaskItemRecord) {
	item.Attempts++
	item.UpdatedAt = businessNow().Format(time.RFC3339)
	switch task.Type {
	case "batch_customer_tags":
		api.processBatchTagItemLocked(task, item)
	default:
		item.Status = "failed"
		item.LastError = "unsupported task type"
		task.FailedCount++
		task.LastError = item.LastError
	}
}

func (api *API) processBatchTagItemLocked(task *TaskRecord, item *TaskItemRecord) {
	var payload BatchTagTaskPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		item.Status = "failed"
		item.LastError = "invalid task payload"
		task.FailedCount++
		task.LastError = item.LastError
		return
	}
	customer, ok := api.findCustomer(item.TargetID)
	if !ok {
		item.Status = "failed"
		item.LastError = "customer not found"
		task.FailedCount++
		task.LastError = item.LastError
		return
	}
	operator := defaultString(payload.Operator, task.CreatedBy)
	customer.Tags = appendUnique(customer.Tags, payload.Tags...)
	customer.Timeline = prependAudit(customer.Timeline, audit(stamp(), "异步批量打标签", "系统标签已更新", operator, "客户："+customer.Name, strings.Join(payload.Tags, "、")))
	item.Status = "succeeded"
	item.Result = json.RawMessage(`{"updated":true}`)
	task.SuccessCount++
}

func (api *API) finishTaskIfDoneLocked(task *TaskRecord) {
	done := task.SuccessCount + task.FailedCount
	if done < task.TotalCount || task.Status == "completed" || task.Status == "failed" || task.Status == "partial_failed" {
		return
	}
	task.FinishedAt = businessNow().Format(time.RFC3339)
	switch {
	case task.FailedCount == 0:
		task.Status = "completed"
	case task.SuccessCount == 0:
		task.Status = "failed"
	default:
		task.Status = "partial_failed"
	}
}
