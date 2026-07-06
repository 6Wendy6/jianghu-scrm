package main

import (
	"context"
	"database/sql"
	"errors"
	"github.com/lib/pq"
	"net/http"
	"time"
)

func (api *API) customerOpsBootstrapDBHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return errNotFound
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := api.refreshOpsExceptionsDB(ctx); err != nil {
		return err
	}
	customers, _, err := api.listOpsCustomersDB(ctx, r)
	if err != nil {
		return err
	}
	tasks, _, err := api.listOpsTasksDB(ctx, r)
	if err != nil {
		return err
	}
	materials, err := api.listOpsMaterialsDB(ctx)
	if err != nil {
		return err
	}
	exceptions, _, err := api.listOpsExceptionsDB(ctx, r)
	if err != nil {
		return err
	}
	tags, err := api.listOpsTagsDB(ctx)
	if err != nil {
		return err
	}
	return writeJSON(w, OpsBootstrap{Customers: customers, Tasks: tasks, Materials: materials, Exceptions: exceptions, Tags: tags})
}

func (api *API) opsCustomersDBHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return errNotFound
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	rows, total, err := api.listOpsCustomersDB(ctx, r)
	if err != nil {
		return err
	}
	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		return writeJSON(w, rows)
	}
	return writeJSON(w, PageResult[OpsCustomer]{Data: rows, Page: makePageMeta(page, pageSize, total)})
}

func (api *API) opsCustomerActionDBHandler(w http.ResponseWriter, r *http.Request, customerID string, parts []string) error {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	customer, err := api.opsCustomerScopedDB(ctx, r, customerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}
	if len(parts) == 0 {
		if r.Method != http.MethodGet {
			return errNotFound
		}
		detail, err := api.opsCustomerDetailFromCustomerDB(ctx, customer)
		if err != nil {
			return err
		}
		return writeJSON(w, detail)
	}
	switch parts[0] {
	case "stage":
		if r.Method != http.MethodPatch {
			return errNotFound
		}
		var payload struct {
			Stage        string `json:"stage"`
			Reason       string `json:"reason"`
			OperatorID   string `json:"operatorId"`
			OperatorName string `json:"operatorName"`
		}
		if err := decode(r, &payload); err != nil {
			return err
		}
		if err := api.changeOpsCustomerStageDB(ctx, customer, payload.Stage, payload.Reason, payload.OperatorID, payload.OperatorName); err != nil {
			return err
		}
	case "tags":
		if len(parts) == 1 && r.Method == http.MethodPost {
			var payload struct {
				TagID        string `json:"tagId"`
				Name         string `json:"name"`
				Category     string `json:"category"`
				Source       string `json:"source"`
				OperatorID   string `json:"operatorId"`
				OperatorName string `json:"operatorName"`
			}
			if err := decode(r, &payload); err != nil {
				return err
			}
			if err := api.addOpsCustomerTagDB(ctx, customer, payload.TagID, payload.Name, payload.Category, payload.Source, payload.OperatorID, payload.OperatorName); err != nil {
				return err
			}
		} else if len(parts) == 2 && r.Method == http.MethodDelete {
			if err := api.removeOpsCustomerTagDB(ctx, customer, parts[1], "", ""); err != nil {
				return err
			}
		} else {
			return errNotFound
		}
	case "follow-ups":
		if r.Method != http.MethodPost {
			return errNotFound
		}
		var payload struct {
			FollowUpType     string `json:"followUpType"`
			Content          string `json:"content"`
			Result           string `json:"result"`
			NextFollowUpTime string `json:"nextFollowUpTime"`
			StageAfter       string `json:"stageAfter"`
			CreatedBy        string `json:"createdBy"`
		}
		if err := decode(r, &payload); err != nil {
			return err
		}
		if err := api.addOpsFollowupDB(ctx, customer, payload.FollowUpType, payload.Content, payload.Result, payload.NextFollowUpTime, payload.StageAfter, payload.CreatedBy); err != nil {
			return err
		}
	case "timeline":
		if r.Method != http.MethodGet {
			return errNotFound
		}
		timeline, err := api.opsTimelineForCustomerDB(ctx, customer.ID)
		if err != nil {
			return err
		}
		return writeJSON(w, timeline)
	default:
		return errNotFound
	}
	api.invalidateCache("summary:", "customers:", "tags:", "tag-groups:", "tag-customers:")
	detail, err := api.opsCustomerDetailDB(ctx, r, customerID)
	if err != nil {
		return err
	}
	return writeJSON(w, detail)
}

func (api *API) opsTasksDBHandler(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		if err := api.refreshOpsExceptionsDB(ctx); err != nil {
			return err
		}
		rows, total, err := api.listOpsTasksDB(ctx, r)
		if err != nil {
			return err
		}
		page, pageSize, paged, err := paginationParams(r)
		if err != nil {
			return err
		}
		if !paged {
			return writeJSON(w, rows)
		}
		return writeJSON(w, PageResult[OpsTask]{Data: rows, Page: makePageMeta(page, pageSize, total)})
	case http.MethodPost:
		var payload OpsTask
		if err := decode(r, &payload); err != nil {
			return err
		}
		task, err := api.createOpsTaskDB(ctx, payload)
		if err != nil {
			return err
		}
		return writeJSON(w, task)
	default:
		return errNotFound
	}
}

func (api *API) opsTaskActionDBHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/sop-tasks/")
	if len(parts) == 0 {
		return errNotFound
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	task, err := api.opsTaskScopedDB(ctx, r, parts[0])
	if err != nil {
		return err
	}
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			return errNotFound
		}
		return writeJSON(w, task)
	}
	var payload struct {
		Remark           string `json:"remark"`
		AssignedToUserID string `json:"assignedToUserId"`
		AssignedToName   string `json:"assignedToName"`
	}
	_ = decode(r, &payload)
	switch parts[1] {
	case "process":
		if err := api.setOpsTaskStatusDB(ctx, task, "processing", "开始处理任务", payload.Remark); err != nil {
			return err
		}
	case "complete":
		if err := api.setOpsTaskStatusDB(ctx, task, "completed", "完成任务", payload.Remark); err != nil {
			return err
		}
	case "assign":
		if err := api.assignOpsTaskDB(ctx, task, payload.AssignedToUserID, payload.AssignedToName); err != nil {
			return err
		}
	case "ignore":
		if err := api.setOpsTaskStatusDB(ctx, task, "ignored", "忽略任务", payload.Remark); err != nil {
			return err
		}
	default:
		return errNotFound
	}
	updated, err := api.opsTaskScopedDB(ctx, r, task.ID)
	if err != nil {
		return err
	}
	return writeJSON(w, updated)
}

func (api *API) opsMaterialsDBHandler(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		rows, err := api.listOpsMaterialsDB(ctx)
		if err != nil {
			return err
		}
		return writePaginatedOrList(w, r, rows)
	case http.MethodPost:
		var payload OpsMaterial
		if err := decode(r, &payload); err != nil {
			return err
		}
		item, err := api.saveOpsMaterialDB(ctx, payload)
		if err != nil {
			return err
		}
		return writeJSON(w, item)
	default:
		return errNotFound
	}
}

func (api *API) opsMaterialActionDBHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/materials/")
	if len(parts) == 0 {
		return errNotFound
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	switch r.Method {
	case http.MethodPatch:
		var payload OpsMaterial
		if err := decode(r, &payload); err != nil {
			return err
		}
		_, err := api.db.ExecContext(ctx, `
			UPDATE materials
			SET title = COALESCE(NULLIF($2, ''), title),
				content = COALESCE(NULLIF($3, ''), content),
				material_type = COALESCE(NULLIF($4, ''), material_type),
				applicable_stage = COALESCE(NULLIF($5, ''), applicable_stage),
				applicable_tags = CASE WHEN $6::text[] IS NULL THEN applicable_tags ELSE $6::text[] END,
				status = COALESCE(NULLIF($7, ''), status),
				updated_at = now()
			WHERE id = $1
		`, parts[0], payload.Title, payload.Content, payload.MaterialType, payload.ApplicableStage, pq.Array(payload.ApplicableTags), payload.Status)
		if err != nil {
			return err
		}
	case http.MethodDelete:
		result, err := api.db.ExecContext(ctx, `DELETE FROM materials WHERE id = $1`, parts[0])
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n == 0 {
			return errNotFound
		}
		return writeJSON(w, map[string]string{"id": parts[0], "status": "deleted"})
	default:
		return errNotFound
	}
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, title, content, material_type, applicable_stage, applicable_tags, status, created_by,
			to_char(created_at, 'YYYY-MM-DD HH24:MI'), to_char(updated_at, 'YYYY-MM-DD HH24:MI')
		FROM materials
		WHERE id = $1
	`, parts[0])
	if err != nil {
		return err
	}
	items, err := scanOpsMaterialRows(rows)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return errNotFound
	}
	return writeJSON(w, items[0])
}

func (api *API) opsExceptionsDBHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return errNotFound
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := api.refreshOpsExceptionsDB(ctx); err != nil {
		return err
	}
	rows, total, err := api.listOpsExceptionsDB(ctx, r)
	if err != nil {
		return err
	}
	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		return writeJSON(w, rows)
	}
	return writeJSON(w, PageResult[OpsException]{Data: rows, Page: makePageMeta(page, pageSize, total)})
}

func (api *API) opsExceptionActionDBHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/operation-exceptions/")
	if len(parts) < 2 {
		return errNotFound
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	status := ""
	switch parts[1] {
	case "process":
		status = "processing"
	case "resolve":
		status = "resolved"
	case "ignore":
		status = "ignored"
	default:
		return errNotFound
	}
	resolvedSQL := "resolved_at"
	if status == "resolved" {
		resolvedSQL = "now()"
	}
	result, err := api.db.ExecContext(ctx, `
		UPDATE operation_exceptions
		SET status = $2,
			resolved_at = `+resolvedSQL+`
		WHERE id = $1
	`, parts[0], status)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return errNotFound
	}
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, exception_type, title, description, severity, region_id, region_name, store_id, store_name,
			customer_id, customer_name, task_id, assigned_to, status, suggestion,
			to_char(created_at, 'YYYY-MM-DD HH24:MI'),
			COALESCE(to_char(resolved_at, 'YYYY-MM-DD HH24:MI'), '')
		FROM operation_exceptions
		WHERE id = $1
	`, parts[0])
	if err != nil {
		return err
	}
	items, err := scanOpsExceptionRows(rows)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return errNotFound
	}
	return writeJSON(w, items[0])
}
