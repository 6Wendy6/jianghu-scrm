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

func (api *API) eventInboxHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.eventInboxDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method == http.MethodPost {
		var payload struct {
			ID             string          `json:"id"`
			EventID        string          `json:"eventId"`
			Source         string          `json:"source"`
			Type           string          `json:"type"`
			ObjectID       string          `json:"objectId"`
			IdempotencyKey string          `json:"idempotencyKey"`
			Payload        json.RawMessage `json:"payload"`
		}
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.Source == "" || payload.Type == "" {
			return fmt.Errorf("%w: source and type are required", errBadRequest)
		}
		eventID := firstNonEmpty(payload.EventID, payload.ID, newID("evt"))
		key := firstNonEmpty(payload.IdempotencyKey, payload.Source+":"+payload.Type+":"+eventID)
		for _, event := range api.events {
			if event.IdempotencyKey == key {
				return writeJSON(w, map[string]any{"status": "duplicate", "event": event})
			}
		}
		event := EventEnvelope{
			ID:             eventID,
			Source:         payload.Source,
			Type:           payload.Type,
			ObjectID:       payload.ObjectID,
			IdempotencyKey: key,
			Status:         "queued",
			ReceivedAt:     businessNow().Format(time.RFC3339),
			Payload:        payload.Payload,
		}
		if len(event.Payload) == 0 {
			event.Payload = json.RawMessage(`{}`)
		}
		api.events = append([]EventEnvelope{event}, api.events...)
		w.WriteHeader(http.StatusAccepted)
		return writeJSON(w, map[string]any{"status": "queued", "event": event})
	}

	status := r.URL.Query().Get("status")
	source := r.URL.Query().Get("source")
	eventType := r.URL.Query().Get("type")
	rows := filter(api.events, func(event EventEnvelope) bool {
		return matchOption(status, "全部状态", event.Status) && matchOption(source, "全部来源", event.Source) && matchOption(eventType, "全部类型", event.Type)
	})
	return writePaginatedOrList(w, r, rows)
}

func (api *API) eventInboxDBHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method == http.MethodPost {
		var payload struct {
			ID             string          `json:"id"`
			EventID        string          `json:"eventId"`
			Source         string          `json:"source"`
			Type           string          `json:"type"`
			ObjectID       string          `json:"objectId"`
			IdempotencyKey string          `json:"idempotencyKey"`
			Payload        json.RawMessage `json:"payload"`
		}
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.Source == "" || payload.Type == "" {
			return fmt.Errorf("%w: source and type are required", errBadRequest)
		}
		eventID := firstNonEmpty(payload.EventID, payload.ID, newID("evt"))
		key := firstNonEmpty(payload.IdempotencyKey, payload.Source+":"+payload.Type+":"+eventID)
		rawPayload := payload.Payload
		if len(rawPayload) == 0 {
			rawPayload = json.RawMessage(`{}`)
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		_, err := api.db.ExecContext(ctx, `
			INSERT INTO event_inbox (id, source, event_type, object_id, idempotency_key, status, payload)
			VALUES ($1, $2, $3, $4, $5, 'queued', $6::jsonb)
		`, eventID, payload.Source, payload.Type, payload.ObjectID, key, string(rawPayload))
		if err != nil {
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "23505" {
				event, findErr := api.eventDBByKey(ctx, key)
				if findErr != nil {
					return findErr
				}
				return writeJSON(w, map[string]any{"status": "duplicate", "event": event})
			}
			return err
		}
		api.invalidateCache("summary:")
		event, err := api.eventDBByKey(ctx, key)
		if err != nil {
			return err
		}
		w.WriteHeader(http.StatusAccepted)
		return writeJSON(w, map[string]any{"status": "queued", "event": event})
	}

	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		page, pageSize = 1, 200
	}
	where, args := eventDBFilters(r)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	var total int
	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM event_inbox `+where, args...).Scan(&total); err != nil {
		return err
	}
	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, source, event_type, object_id, idempotency_key, status, attempts,
			to_char(received_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'),
			COALESCE(to_char(processed_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			payload::text
		FROM event_inbox
		`+where+`
		ORDER BY received_at DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		queryArgs...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	events, err := scanEventRows(rows)
	if err != nil {
		return err
	}
	if !paged {
		return writeJSON(w, events)
	}
	return writeJSON(w, PageResult[EventEnvelope]{Data: events, Page: makePageMeta(page, pageSize, total)})
}

func eventDBFilters(r *http.Request) (string, []any) {
	query := r.URL.Query()
	args := []any{}
	conditions := []string{}
	if status := strings.TrimSpace(query.Get("status")); status != "" && status != "全部状态" {
		args = append(args, status)
		conditions = append(conditions, "status = $"+strconv.Itoa(len(args)))
	}
	if source := strings.TrimSpace(query.Get("source")); source != "" && source != "全部来源" {
		args = append(args, source)
		conditions = append(conditions, "source = $"+strconv.Itoa(len(args)))
	}
	if eventType := strings.TrimSpace(query.Get("type")); eventType != "" && eventType != "全部类型" {
		args = append(args, eventType)
		conditions = append(conditions, "event_type = $"+strconv.Itoa(len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (api *API) eventDBByKey(ctx context.Context, key string) (EventEnvelope, error) {
	row := api.db.QueryRowContext(ctx, `
		SELECT id, source, event_type, object_id, idempotency_key, status, attempts,
			to_char(received_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'),
			COALESCE(to_char(processed_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			payload::text
		FROM event_inbox
		WHERE idempotency_key = $1
	`, key)
	return scanEventRow(row)
}

func (api *API) tasksHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.tasksDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method == http.MethodPost {
		var payload struct {
			Type      string              `json:"type"`
			BatchTags BatchTagTaskPayload `json:"batchTags"`
		}
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.Type != "batch_customer_tags" {
			return fmt.Errorf("%w: unsupported task type", errBadRequest)
		}
		task, err := api.enqueueBatchTagTaskLocked(payload.BatchTags)
		if err != nil {
			return err
		}
		w.WriteHeader(http.StatusAccepted)
		return writeJSON(w, map[string]any{"status": "queued", "task": task})
	}

	status := r.URL.Query().Get("status")
	taskType := r.URL.Query().Get("type")
	rows := filter(api.tasks, func(task TaskRecord) bool {
		return matchOption(status, "全部状态", task.Status) && matchOption(taskType, "全部类型", task.Type)
	})
	return writePaginatedOrList(w, r, rows)
}

func (api *API) tasksDBHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method == http.MethodPost {
		var payload struct {
			Type      string              `json:"type"`
			BatchTags BatchTagTaskPayload `json:"batchTags"`
		}
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.Type != "batch_customer_tags" {
			return fmt.Errorf("%w: unsupported task type", errBadRequest)
		}
		task, err := api.enqueueBatchTagTaskDB(r.Context(), payload.BatchTags)
		if err != nil {
			return err
		}
		api.invalidateCache("summary:")
		w.WriteHeader(http.StatusAccepted)
		return writeJSON(w, map[string]any{"status": "queued", "task": task})
	}

	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		page, pageSize = 1, 200
	}
	where, args := taskDBFilters(r)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	var total int
	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM tasks `+where, args...).Scan(&total); err != nil {
		return err
	}
	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, task_type, status, total_count, success_count, failed_count,
			payload::text, created_by, last_error,
			to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'),
			COALESCE(to_char(started_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			COALESCE(to_char(finished_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), '')
		FROM tasks
		`+where+`
		ORDER BY created_at DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		queryArgs...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	tasks, err := scanTaskRows(rows)
	if err != nil {
		return err
	}
	if !paged {
		return writeJSON(w, tasks)
	}
	return writeJSON(w, PageResult[TaskRecord]{Data: tasks, Page: makePageMeta(page, pageSize, total)})
}

func (api *API) taskActionHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.taskActionDBHandler(w, r)
	}
	api.mu.RLock()
	defer api.mu.RUnlock()
	parts := pathParts(r.URL.Path, "/api/tasks/")
	if len(parts) == 0 {
		return errNotFound
	}
	task, ok := api.findTask(parts[0])
	if !ok {
		return errNotFound
	}
	if len(parts) == 1 {
		items := filter(api.taskItems, func(item TaskItemRecord) bool { return item.TaskID == task.ID })
		return writeJSON(w, map[string]any{"task": task, "items": items})
	}
	if len(parts) == 2 && parts[1] == "items" {
		items := filter(api.taskItems, func(item TaskItemRecord) bool { return item.TaskID == task.ID })
		return writePaginatedOrList(w, r, items)
	}
	return errNotFound
}

func (api *API) taskActionDBHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/tasks/")
	if len(parts) == 0 {
		return errNotFound
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	task, err := api.taskDB(ctx, parts[0])
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}
	if len(parts) == 1 {
		items, err := api.taskItemsDB(ctx, task.ID, 1, 200)
		if err != nil {
			return err
		}
		return writeJSON(w, map[string]any{"task": task, "items": items})
	}
	if len(parts) == 2 && parts[1] == "items" {
		page, pageSize, paged, err := paginationParams(r)
		if err != nil {
			return err
		}
		if !paged {
			page, pageSize = 1, 200
		}
		total, err := api.taskItemsCountDB(ctx, task.ID)
		if err != nil {
			return err
		}
		items, err := api.taskItemsDB(ctx, task.ID, page, pageSize)
		if err != nil {
			return err
		}
		if !paged {
			return writeJSON(w, items)
		}
		return writeJSON(w, PageResult[TaskItemRecord]{Data: items, Page: makePageMeta(page, pageSize, total)})
	}
	return errNotFound
}

func taskDBFilters(r *http.Request) (string, []any) {
	query := r.URL.Query()
	args := []any{}
	conditions := []string{}
	if status := strings.TrimSpace(query.Get("status")); status != "" && status != "全部状态" {
		args = append(args, status)
		conditions = append(conditions, "status = $"+strconv.Itoa(len(args)))
	}
	if taskType := strings.TrimSpace(query.Get("type")); taskType != "" && taskType != "全部类型" {
		args = append(args, taskType)
		conditions = append(conditions, "task_type = $"+strconv.Itoa(len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func (api *API) findStore(id string) (*Store, bool) {
	for i := range api.stores {
		if api.stores[i].ID == id {
			return &api.stores[i], true
		}
	}
	return nil, false
}

func (api *API) findGuide(id string) (*Guide, bool) {
	for i := range api.guides {
		if api.guides[i].ID == id || api.guides[i].Name == id {
			return &api.guides[i], true
		}
	}
	return nil, false
}

func (api *API) findCustomer(id string) (*Customer, bool) {
	for i := range api.customers {
		if api.customers[i].ID == id {
			return &api.customers[i], true
		}
	}
	return nil, false
}

func (api *API) findTask(id string) (TaskRecord, bool) {
	for _, task := range api.tasks {
		if task.ID == id {
			return task, true
		}
	}
	return TaskRecord{}, false
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEventRow(row rowScanner) (EventEnvelope, error) {
	var event EventEnvelope
	var payload string
	if err := row.Scan(&event.ID, &event.Source, &event.Type, &event.ObjectID, &event.IdempotencyKey, &event.Status, &event.Attempts, &event.ReceivedAt, &event.ProcessedAt, &payload); err != nil {
		return EventEnvelope{}, err
	}
	event.Payload = json.RawMessage(payload)
	return event, nil
}

func scanEventRows(rows *sql.Rows) ([]EventEnvelope, error) {
	events := []EventEnvelope{}
	for rows.Next() {
		event, err := scanEventRow(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func scanTaskRow(row rowScanner) (TaskRecord, error) {
	var task TaskRecord
	var payload string
	if err := row.Scan(&task.ID, &task.Type, &task.Status, &task.TotalCount, &task.SuccessCount, &task.FailedCount, &payload, &task.CreatedBy, &task.LastError, &task.CreatedAt, &task.StartedAt, &task.FinishedAt); err != nil {
		return TaskRecord{}, err
	}
	task.Payload = json.RawMessage(payload)
	return task, nil
}

func scanTaskRows(rows *sql.Rows) ([]TaskRecord, error) {
	tasks := []TaskRecord{}
	for rows.Next() {
		task, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (api *API) taskDB(ctx context.Context, id string) (TaskRecord, error) {
	row := api.db.QueryRowContext(ctx, `
		SELECT id, task_type, status, total_count, success_count, failed_count,
			payload::text, created_by, last_error,
			to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'),
			COALESCE(to_char(started_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			COALESCE(to_char(finished_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), '')
		FROM tasks
		WHERE id = $1
	`, id)
	return scanTaskRow(row)
}

func (api *API) taskItemsCountDB(ctx context.Context, taskID string) (int, error) {
	var total int
	err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM task_items WHERE task_id = $1`, taskID).Scan(&total)
	return total, err
}

func (api *API) taskItemsDB(ctx context.Context, taskID string, page, pageSize int) ([]TaskItemRecord, error) {
	offset := (page - 1) * pageSize
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, task_id, target_id, status, attempts, result::text, last_error,
			to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM task_items
		WHERE task_id = $1
		ORDER BY updated_at DESC, id
		LIMIT $2 OFFSET $3
	`, taskID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []TaskItemRecord{}
	for rows.Next() {
		var item TaskItemRecord
		var result string
		if err := rows.Scan(&item.ID, &item.TaskID, &item.TargetID, &item.Status, &item.Attempts, &result, &item.LastError, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Result = json.RawMessage(result)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
