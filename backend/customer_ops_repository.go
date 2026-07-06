package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/lib/pq"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type opsSQLExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (api *API) listOpsCustomersDB(ctx context.Context, r *http.Request) ([]OpsCustomer, int, error) {
	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return nil, 0, err
	}
	if !paged {
		page, pageSize = 1, 500
	}
	scope := opsScopeFromRequest(r)
	conditions, args := opsCustomerListFilters(r)
	scope.applyCustomerDataScope("c", &conditions, &args)
	where := whereClause(conditions)
	var total int
	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM customers c`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := api.db.QueryContext(ctx, `
		SELECT `+opsCustomerSelectSQL("c")+`
		FROM customers c
		`+where+`
		ORDER BY c.updated_at DESC, c.id DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		queryArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	customers, err := scanOpsCustomers(rows)
	if err != nil {
		return nil, 0, err
	}
	if err := api.attachOpsTagsDB(ctx, customers); err != nil {
		return nil, 0, err
	}
	return customers, total, nil
}

func opsCustomerListFilters(r *http.Request) ([]string, []any) {
	q := r.URL.Query()
	args := []any{}
	conditions := []string{}
	if keyword := strings.TrimSpace(q.Get("keyword")); keyword != "" {
		args = append(args, "%"+keyword+"%")
		placeholder := "$" + strconv.Itoa(len(args))
		conditions = append(conditions, "(c.name ILIKE "+placeholder+" OR c.mobile_masked ILIKE "+placeholder+" OR c.store_name ILIKE "+placeholder+" OR c.owner_guide_name ILIKE "+placeholder+" OR c.source_channel ILIKE "+placeholder+")")
	}
	if stage := strings.TrimSpace(q.Get("stage")); stage != "" && stage != "all" {
		args = append(args, stage)
		conditions = append(conditions, "c.lifecycle_stage = $"+strconv.Itoa(len(args)))
	}
	if status := strings.TrimSpace(q.Get("status")); status != "" && status != "all" {
		args = append(args, status)
		conditions = append(conditions, "c.status = $"+strconv.Itoa(len(args)))
	}
	return conditions, args
}

func (api *API) opsCustomerDetailDB(ctx context.Context, r *http.Request, customerID string) (map[string]any, error) {
	customer, err := api.opsCustomerScopedDB(ctx, r, customerID)
	if err != nil {
		return nil, err
	}
	return api.opsCustomerDetailFromCustomerDB(ctx, customer)
}

func (api *API) opsCustomerDetailFromCustomerDB(ctx context.Context, customer OpsCustomer) (map[string]any, error) {
	customers := []OpsCustomer{customer}
	if err := api.attachOpsTagsDB(ctx, customers); err != nil {
		return nil, err
	}
	tags, err := api.opsTagsForCustomerDB(ctx, customer.ID)
	if err != nil {
		return nil, err
	}
	followups, err := api.opsFollowupsForCustomerDB(ctx, customer.ID)
	if err != nil {
		return nil, err
	}
	timeline, err := api.opsTimelineForCustomerDB(ctx, customer.ID)
	if err != nil {
		return nil, err
	}
	tasks, err := api.opsTasksForCustomerDB(ctx, customer.ID)
	if err != nil {
		return nil, err
	}
	materials, err := api.opsMaterialsForCustomerDB(ctx, customers[0])
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"customer":  customers[0],
		"tags":      tags,
		"followups": followups,
		"timeline":  timeline,
		"tasks":     tasks,
		"materials": materials,
	}, nil
}

func (api *API) opsCustomerScopedDB(ctx context.Context, r *http.Request, customerID string) (OpsCustomer, error) {
	scope := opsScopeFromRequest(r)
	conditions := []string{"c.id = $1"}
	args := []any{customerID}
	scope.applyCustomerDataScope("c", &conditions, &args)
	rows, err := api.db.QueryContext(ctx, `
		SELECT `+opsCustomerSelectSQL("c")+`
		FROM customers c
		`+whereClause(conditions)+`
		LIMIT 1
	`, args...)
	if err != nil {
		return OpsCustomer{}, err
	}
	customers, err := scanOpsCustomers(rows)
	if err != nil {
		return OpsCustomer{}, err
	}
	if len(customers) > 0 {
		return customers[0], nil
	}
	exists, err := api.opsCustomerExistsDB(ctx, customerID)
	if err != nil {
		return OpsCustomer{}, err
	}
	if exists {
		return OpsCustomer{}, errForbidden
	}
	return OpsCustomer{}, errNotFound
}

func (api *API) opsCustomerExistsDB(ctx context.Context, customerID string) (bool, error) {
	var exists bool
	err := api.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM customers WHERE id = $1)`, customerID).Scan(&exists)
	return exists, err
}

func (api *API) attachOpsTagsDB(ctx context.Context, customers []OpsCustomer) error {
	ids := []string{}
	for _, customer := range customers {
		ids = append(ids, customer.ID)
	}
	tagMap, err := api.opsTagNamesForCustomersDB(ctx, ids)
	if err != nil {
		return err
	}
	for i := range customers {
		customers[i].Tags = tagMap[customers[i].ID]
	}
	return nil
}

func (api *API) opsTagNamesForCustomersDB(ctx context.Context, ids []string) (map[string][]string, error) {
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
		var customerID, tagName string
		if err := rows.Scan(&customerID, &tagName); err != nil {
			return nil, err
		}
		result[customerID] = appendUnique(result[customerID], tagName)
	}
	return result, rows.Err()
}

func (api *API) opsTagsForCustomerDB(ctx context.Context, customerID string) ([]OpsCustomerTag, error) {
	rows, err := api.db.QueryContext(ctx, `
		SELECT t.id, t.name, COALESCE(NULLIF(t.category, ''), g.name), t.color, t.source, t.is_enabled,
			to_char(t.created_at, 'YYYY-MM-DD HH24:MI'),
			to_char(t.updated_at, 'YYYY-MM-DD HH24:MI')
		FROM customer_tags ct
		JOIN tags t ON t.id = ct.tag_id
		JOIN tag_groups g ON g.id = t.tag_group_id
		WHERE ct.customer_id = $1
		ORDER BY t.name
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []OpsCustomerTag{}
	for rows.Next() {
		var tag OpsCustomerTag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Category, &tag.Color, &tag.Source, &tag.IsEnabled, &tag.CreatedAt, &tag.UpdatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (api *API) listOpsTagsDB(ctx context.Context) ([]OpsCustomerTag, error) {
	rows, err := api.db.QueryContext(ctx, `
		SELECT t.id, t.name, COALESCE(NULLIF(t.category, ''), g.name), t.color, t.source, t.is_enabled,
			to_char(t.created_at, 'YYYY-MM-DD HH24:MI'),
			to_char(t.updated_at, 'YYYY-MM-DD HH24:MI')
		FROM tags t
		JOIN tag_groups g ON g.id = t.tag_group_id
		WHERE t.status = 'active'
		ORDER BY g.display_order, t.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []OpsCustomerTag{}
	for rows.Next() {
		var tag OpsCustomerTag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Category, &tag.Color, &tag.Source, &tag.IsEnabled, &tag.CreatedAt, &tag.UpdatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (api *API) opsFollowupsForCustomerDB(ctx context.Context, customerID string) ([]OpsFollowUpRecord, error) {
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, customer_id, store_id, guide_id, follow_up_type, content, result,
			COALESCE(to_char(next_follow_up_time, 'YYYY-MM-DD HH24:MI'), ''),
			stage_before, stage_after, created_by, to_char(created_at, 'YYYY-MM-DD HH24:MI')
		FROM follow_up_records
		WHERE customer_id = $1
		ORDER BY created_at DESC, id DESC
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rowsOut := []OpsFollowUpRecord{}
	for rows.Next() {
		var item OpsFollowUpRecord
		if err := rows.Scan(&item.ID, &item.CustomerID, &item.StoreID, &item.GuideID, &item.FollowUpType, &item.Content, &item.Result, &item.NextFollowUpTime, &item.StageBefore, &item.StageAfter, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		rowsOut = append(rowsOut, item)
	}
	return rowsOut, rows.Err()
}

func (api *API) opsTimelineForCustomerDB(ctx context.Context, customerID string) ([]OpsTimelineEvent, error) {
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, customer_id, event_type, title, content, related_id, operator_id, operator_name,
			to_char(created_at, 'YYYY-MM-DD HH24:MI')
		FROM customer_operation_timeline
		WHERE customer_id = $1
		ORDER BY created_at DESC, id DESC
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOpsTimelineRows(rows)
}

func (api *API) opsTasksForCustomerDB(ctx context.Context, customerID string) ([]OpsTask, error) {
	rows, err := api.db.QueryContext(ctx, `
		SELECT `+opsTaskSelectSQL("t")+`
		FROM sop_tasks t
		WHERE t.customer_id = $1
		ORDER BY t.due_time ASC NULLS LAST, t.created_at DESC
	`, customerID)
	if err != nil {
		return nil, err
	}
	return scanOpsTaskRows(rows)
}

func (api *API) listOpsTasksDB(ctx context.Context, r *http.Request) ([]OpsTask, int, error) {
	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return nil, 0, err
	}
	if !paged {
		page, pageSize = 1, 500
	}
	scope := opsScopeFromRequest(r)
	conditions := []string{}
	args := []any{}
	scope.applyTaskDataScope("t", &conditions, &args)
	q := r.URL.Query()
	if status := strings.TrimSpace(q.Get("status")); status != "" && status != "all" {
		args = append(args, status)
		conditions = append(conditions, "t.status = $"+strconv.Itoa(len(args)))
	}
	if priority := strings.TrimSpace(q.Get("priority")); priority != "" && priority != "all" {
		args = append(args, priority)
		conditions = append(conditions, "t.priority = $"+strconv.Itoa(len(args)))
	}
	if keyword := strings.TrimSpace(q.Get("keyword")); keyword != "" {
		args = append(args, "%"+keyword+"%")
		conditions = append(conditions, "(t.title ILIKE $"+strconv.Itoa(len(args))+" OR t.customer_name ILIKE $"+strconv.Itoa(len(args))+" OR t.store_name ILIKE $"+strconv.Itoa(len(args))+")")
	}
	where := whereClause(conditions)
	var total int
	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM sop_tasks t`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := api.db.QueryContext(ctx, `
		SELECT `+opsTaskSelectSQL("t")+`
		FROM sop_tasks t
		`+where+`
		ORDER BY t.due_time ASC NULLS LAST, t.created_at DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		queryArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	tasks, err := scanOpsTaskRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

func (api *API) opsTaskScopedDB(ctx context.Context, r *http.Request, taskID string) (OpsTask, error) {
	scope := opsScopeFromRequest(r)
	conditions := []string{"t.id = $1"}
	args := []any{taskID}
	scope.applyTaskDataScope("t", &conditions, &args)
	rows, err := api.db.QueryContext(ctx, `
		SELECT `+opsTaskSelectSQL("t")+`
		FROM sop_tasks t
		`+whereClause(conditions)+`
		LIMIT 1
	`, args...)
	if err != nil {
		return OpsTask{}, err
	}
	tasks, err := scanOpsTaskRows(rows)
	if err != nil {
		return OpsTask{}, err
	}
	if len(tasks) > 0 {
		return tasks[0], nil
	}
	var exists bool
	if err := api.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sop_tasks WHERE id = $1)`, taskID).Scan(&exists); err != nil {
		return OpsTask{}, err
	}
	if exists {
		return OpsTask{}, errForbidden
	}
	return OpsTask{}, errNotFound
}

func (api *API) opsMaterialsForCustomerDB(ctx context.Context, customer OpsCustomer) ([]OpsMaterial, error) {
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, title, content, material_type, applicable_stage, applicable_tags, status, created_by,
			to_char(created_at, 'YYYY-MM-DD HH24:MI'), to_char(updated_at, 'YYYY-MM-DD HH24:MI')
		FROM materials
		WHERE status = 'enabled'
		ORDER BY updated_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	materials, err := scanOpsMaterialRows(rows)
	if err != nil {
		return nil, err
	}
	filtered := []OpsMaterial{}
	for _, material := range materials {
		if material.ApplicableStage == customer.LifecycleStage || intersects(material.ApplicableTags, customer.Tags) {
			filtered = append(filtered, material)
		}
	}
	return filtered, nil
}

func (api *API) listOpsMaterialsDB(ctx context.Context) ([]OpsMaterial, error) {
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, title, content, material_type, applicable_stage, applicable_tags, status, created_by,
			to_char(created_at, 'YYYY-MM-DD HH24:MI'), to_char(updated_at, 'YYYY-MM-DD HH24:MI')
		FROM materials
		ORDER BY updated_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	return scanOpsMaterialRows(rows)
}

func (api *API) listOpsExceptionsDB(ctx context.Context, r *http.Request) ([]OpsException, int, error) {
	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return nil, 0, err
	}
	if !paged {
		page, pageSize = 1, 500
	}
	scope := opsScopeFromRequest(r)
	conditions := []string{}
	args := []any{}
	scope.applyExceptionDataScope("e", &conditions, &args)
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" && status != "all" {
		args = append(args, status)
		conditions = append(conditions, "e.status = $"+strconv.Itoa(len(args)))
	}
	where := whereClause(conditions)
	var total int
	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM operation_exceptions e`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, exception_type, title, description, severity, region_id, region_name, store_id, store_name,
			customer_id, customer_name, task_id, assigned_to, status, suggestion,
			to_char(created_at, 'YYYY-MM-DD HH24:MI'),
			COALESCE(to_char(resolved_at, 'YYYY-MM-DD HH24:MI'), '')
		FROM operation_exceptions e
		`+where+`
		ORDER BY CASE severity WHEN 'P0' THEN 0 WHEN 'P1' THEN 1 ELSE 2 END, created_at DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		queryArgs...,
	)
	if err != nil {
		return nil, 0, err
	}
	exceptions, err := scanOpsExceptionRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return exceptions, total, nil
}

func opsCustomerSelectSQL(alias string) string {
	p := scopeAlias(alias)
	return strings.Join([]string{
		p + "id",
		p + "name",
		"COALESCE(NULLIF(" + p + "nickname, ''), " + p + "wecom_name)",
		p + "mobile_masked",
		p + "avatar",
		p + "source_channel",
		p + "region_id",
		p + "region_name",
		"COALESCE(NULLIF(" + p + "store_id, ''), " + p + "source_store_id)",
		"COALESCE(NULLIF(" + p + "store_name, ''), " + p + "source_store_name)",
		"COALESCE(NULLIF(" + p + "owner_guide_id, ''), " + p + "owner_staff_id)",
		"COALESCE(NULLIF(" + p + "owner_guide_name, ''), " + p + "owner_staff_name)",
		p + "lifecycle_stage",
		p + "intention_level",
		p + "status",
		"COALESCE(to_char(" + p + "add_wecom_time, 'YYYY-MM-DD HH24:MI'), '')",
		"COALESCE(to_char(" + p + "last_follow_up_time, 'YYYY-MM-DD HH24:MI'), '')",
		"COALESCE(to_char(" + p + "last_interaction_time, 'YYYY-MM-DD HH24:MI'), '')",
		p + "deal_status",
		p + "risk",
		"to_char(" + p + "created_at, 'YYYY-MM-DD HH24:MI')",
		"to_char(" + p + "updated_at, 'YYYY-MM-DD HH24:MI')",
	}, ", ")
}

func scanOpsCustomers(rows *sql.Rows) ([]OpsCustomer, error) {
	defer rows.Close()
	customers := []OpsCustomer{}
	for rows.Next() {
		var customer OpsCustomer
		if err := rows.Scan(&customer.ID, &customer.Name, &customer.Nickname, &customer.Mobile, &customer.Avatar, &customer.SourceChannel, &customer.RegionID, &customer.RegionName, &customer.StoreID, &customer.StoreName, &customer.OwnerGuideID, &customer.OwnerGuideName, &customer.LifecycleStage, &customer.IntentionLevel, &customer.Status, &customer.AddWeComTime, &customer.LastFollowUpTime, &customer.LastInteractionTime, &customer.DealStatus, &customer.Risk, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
			return nil, err
		}
		customers = append(customers, customer)
	}
	return customers, rows.Err()
}

func opsTaskSelectSQL(alias string) string {
	p := scopeAlias(alias)
	return strings.Join([]string{
		p + "id", p + "task_type", p + "title", p + "description", p + "customer_id", p + "customer_name",
		p + "region_id", p + "region_name", p + "store_id", p + "store_name",
		p + "assigned_to_user_id", p + "assigned_to_name", p + "assigned_to_role", p + "priority", p + "status",
		"COALESCE(to_char(" + p + "due_time, 'YYYY-MM-DD HH24:MI'), '')",
		"COALESCE(to_char(" + p + "completed_at, 'YYYY-MM-DD HH24:MI'), '')",
		p + "source", "to_char(" + p + "created_at, 'YYYY-MM-DD HH24:MI')", "to_char(" + p + "updated_at, 'YYYY-MM-DD HH24:MI')",
	}, ", ")
}

func scanOpsTaskRows(rows *sql.Rows) ([]OpsTask, error) {
	defer rows.Close()
	tasks := []OpsTask{}
	for rows.Next() {
		var task OpsTask
		if err := rows.Scan(&task.ID, &task.TaskType, &task.Title, &task.Description, &task.CustomerID, &task.CustomerName, &task.RegionID, &task.RegionName, &task.StoreID, &task.StoreName, &task.AssignedToUserID, &task.AssignedToName, &task.AssignedToRole, &task.Priority, &task.Status, &task.DueTime, &task.CompletedAt, &task.Source, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func scanOpsTimelineRows(rows *sql.Rows) ([]OpsTimelineEvent, error) {
	defer rows.Close()
	events := []OpsTimelineEvent{}
	for rows.Next() {
		var event OpsTimelineEvent
		if err := rows.Scan(&event.ID, &event.CustomerID, &event.EventType, &event.Title, &event.Content, &event.RelatedID, &event.OperatorID, &event.OperatorName, &event.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func scanOpsMaterialRows(rows *sql.Rows) ([]OpsMaterial, error) {
	defer rows.Close()
	materials := []OpsMaterial{}
	for rows.Next() {
		var item OpsMaterial
		if err := rows.Scan(&item.ID, &item.Title, &item.Content, &item.MaterialType, &item.ApplicableStage, pq.Array(&item.ApplicableTags), &item.Status, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		materials = append(materials, item)
	}
	return materials, rows.Err()
}

func scanOpsExceptionRows(rows *sql.Rows) ([]OpsException, error) {
	defer rows.Close()
	exceptions := []OpsException{}
	for rows.Next() {
		var item OpsException
		if err := rows.Scan(&item.ID, &item.ExceptionType, &item.Title, &item.Description, &item.Severity, &item.RegionID, &item.RegionName, &item.StoreID, &item.StoreName, &item.CustomerID, &item.CustomerName, &item.TaskID, &item.AssignedTo, &item.Status, &item.Suggestion, &item.CreatedAt, &item.ResolvedAt); err != nil {
			return nil, err
		}
		exceptions = append(exceptions, item)
	}
	return exceptions, rows.Err()
}

func parseOptionalBusinessTime(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04", value, businessLocation); err == nil {
		return t
	}
	return value
}

func (api *API) ensureOpsDBTag(ctx context.Context, tx *sql.Tx, tagID, name, category, source string) (OpsCustomerTag, error) {
	if tagID != "" {
		var tag OpsCustomerTag
		err := tx.QueryRowContext(ctx, `
			SELECT id, name, category, color, source, is_enabled,
				to_char(created_at, 'YYYY-MM-DD HH24:MI'), to_char(updated_at, 'YYYY-MM-DD HH24:MI')
			FROM tags WHERE id = $1
		`, tagID).Scan(&tag.ID, &tag.Name, &tag.Category, &tag.Color, &tag.Source, &tag.IsEnabled, &tag.CreatedAt, &tag.UpdatedAt)
		if err == nil {
			return tag, nil
		}
		if err != sql.ErrNoRows {
			return OpsCustomerTag{}, err
		}
	}
	name = strings.TrimSpace(firstNonEmpty(name, tagID))
	if name == "" {
		return OpsCustomerTag{}, fmt.Errorf("%w: tag name is required", errBadRequest)
	}
	groupID, err := ensureDBTagGroup(ctx, tx, firstNonEmpty(category, "手动标签"), "全部门店", 20)
	if err != nil {
		return OpsCustomerTag{}, err
	}
	var tag OpsCustomerTag
	err = tx.QueryRowContext(ctx, `
		INSERT INTO tags (id, tag_group_id, name, status, category, color, source, is_enabled)
		VALUES ($1, $2, $3, 'active', $4, 'blue', $5, true)
		ON CONFLICT (tag_group_id, name) DO UPDATE SET
			status = 'active',
			category = COALESCE(NULLIF(EXCLUDED.category, ''), tags.category),
			source = COALESCE(NULLIF(EXCLUDED.source, ''), tags.source),
			is_enabled = true,
			updated_at = now()
		RETURNING id, name, category, color, source, is_enabled,
			to_char(created_at, 'YYYY-MM-DD HH24:MI'), to_char(updated_at, 'YYYY-MM-DD HH24:MI')
	`, firstNonEmpty(tagID, newID("tag")), groupID, name, firstNonEmpty(category, "手动标签"), firstNonEmpty(source, "manual")).Scan(&tag.ID, &tag.Name, &tag.Category, &tag.Color, &tag.Source, &tag.IsEnabled, &tag.CreatedAt, &tag.UpdatedAt)
	return tag, err
}
