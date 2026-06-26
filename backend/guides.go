package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (api *API) storeGuidesDBHandler(w http.ResponseWriter, r *http.Request, storeID string) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	store, err := api.storeDB(ctx, storeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}
	if r.Method == http.MethodPost {
		var payload struct {
			Name string `json:"name"`
		}
		if err := decode(r, &payload); err != nil {
			return err
		}
		payload.Name = strings.TrimSpace(payload.Name)
		if payload.Name == "" {
			return fmt.Errorf("%w: guide name is required", errBadRequest)
		}
		guide, err := api.createGuideDB(ctx, store, payload.Name)
		if err != nil {
			return err
		}
		api.invalidateCache("summary:")
		return writeJSON(w, guide)
	}
	return api.guidesDBList(w, r, "store", storeID)
}

func (api *API) guidesDBHandler(w http.ResponseWriter, r *http.Request) error {
	return api.guidesDBList(w, r, "all", "")
}

func (api *API) guidesDBList(w http.ResponseWriter, r *http.Request, scope, storeID string) error {
	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		page, pageSize = 1, 200
	}
	where, args := guideDBFilters(r, scope, storeID)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	var total int
	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM guides g JOIN stores s ON s.id = g.store_id `+where, args...).Scan(&total); err != nil {
		return err
	}
	guides, err := api.guidesDB(ctx, where, args, page, pageSize)
	if err != nil {
		return err
	}
	if !paged {
		return writeJSON(w, guides)
	}
	return writeJSON(w, PageResult[Guide]{Data: guides, Page: makePageMeta(page, pageSize, total)})
}

func (api *API) guideActionDBHandler(w http.ResponseWriter, r *http.Request, parts []string) error {
	guideID := parts[0]
	action := parts[1]
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	guide, err := api.guideDB(ctx, guideID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}
	switch action {
	case "pause":
		paused := !guide.Paused
		status := "active"
		employmentStatus := "active"
		handling := "正常服务"
		actionText := "恢复使用"
		result := "恢复轮巡"
		detail := "导购重新进入物料码轮巡池。"
		if paused {
			status = "paused"
			handling = "已暂停使用，物料码轮巡不会分配给该导购"
			actionText = "暂停使用"
			result = "暂停轮巡"
			detail = "不改写存量客户来源和服务关系。"
		}
		if guide.EmploymentStatus == "已移除" {
			return fmt.Errorf("%w: removed guide cannot be paused", errBadRequest)
		}
		if _, err := api.db.ExecContext(ctx, `
			UPDATE guides
			SET paused = $2, status = $3, employment_status = $4, handling = $5, updated_at = now()
			WHERE id = $1
		`, guide.ID, paused, status, employmentStatus, handling); err != nil {
			return err
		}
		if err := api.insertGuideEventDB(ctx, guide.ID, actionText, result, "店长", "导购："+guide.Name, detail); err != nil {
			return err
		}
		updated, err := api.guideDB(ctx, guide.ID)
		if err != nil {
			return err
		}
		api.invalidateCache("summary:")
		return writeJSON(w, updated)
	case "remove":
		if _, err := api.db.ExecContext(ctx, `
			UPDATE guides
			SET paused = true,
				status = 'removed',
				employment_status = 'removed',
				handling = '客户关系保留，后续不再分配新客',
				updated_at = now()
			WHERE id = $1
		`, guide.ID); err != nil {
			return err
		}
		if _, err := api.db.ExecContext(ctx, `
			UPDATE store_codes
			SET status = 'destroyed', destroyed_at = COALESCE(destroyed_at, now())
			WHERE staff_id = $1 AND code_type = 'guide_code'
		`, guide.ID); err != nil {
			return err
		}
		if err := api.insertGuideEventDB(ctx, guide.ID, "移除成员", "停用可恢复", "店长", "导购："+guide.Name, "客户关系保留，活码退出物料码轮巡。"); err != nil {
			return err
		}
		updated, err := api.guideDB(ctx, guide.ID)
		if err != nil {
			return err
		}
		api.invalidateCache("summary:")
		return writeJSON(w, updated)
	case "lifecycle":
		events, err := api.guideEventsDB(ctx, guide.ID)
		if err != nil {
			return err
		}
		return writeJSON(w, events)
	case "qr-download":
		return writeJSON(w, makeQRDownload(guide.Code, "导购活码"))
	default:
		return errNotFound
	}
}

func (api *API) storeDB(ctx context.Context, storeID string) (Store, error) {
	var store Store
	err := api.db.QueryRowContext(ctx, `
		SELECT
			s.id, s.name, s.internal_code, s.external_code, s.brand, s.region, s.store_type,
			s.entry_mode, s.service_guide, s.group_name, s.welcome_rule, s.polling_rule,
			COUNT(DISTINCT g.id)::int,
			COUNT(DISTINCT c.id)::int
		FROM stores s
		LEFT JOIN guides g ON g.store_id = s.id AND g.employment_status <> 'removed'
		LEFT JOIN customers c ON c.source_store_id = s.id
		WHERE s.id = $1
		GROUP BY s.id, s.name, s.internal_code, s.external_code, s.brand, s.region, s.store_type,
			s.entry_mode, s.service_guide, s.group_name, s.welcome_rule, s.polling_rule
	`, storeID).Scan(
		&store.ID, &store.Name, &store.InternalCode, &store.ExternalCode, &store.Brand, &store.Region, &store.Type,
		&store.Config.EntryMode, &store.Config.ServiceGuide, &store.Config.Group, &store.Config.Welcome, &store.Config.Polling,
		&store.GuideCount, &store.PoolCount,
	)
	if err != nil {
		return Store{}, err
	}
	store.BrandRegion = strings.Trim(strings.TrimSpace(store.Brand)+" / "+strings.TrimSpace(store.Region), " /")
	return store, nil
}

func (api *API) createGuideDB(ctx context.Context, store Store, name string) (Guide, error) {
	guideID := newID("g")
	code := storeCodePrefix(store) + "-" + name
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return Guide{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO guides (id, store_id, name, code, status, employment_status, paused, handling)
		VALUES ($1, $2, $3, $4, 'active', 'active', false, '正常服务')
	`, guideID, store.ID, name, code); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return Guide{}, fmt.Errorf("%w: guide name already exists in this store", errBadRequest)
		}
		return Guide{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO store_codes (id, code_type, store_id, staff_id, entry_mode, welcome_rule_id, status, qr_payload)
		VALUES ($1, 'guide_code', $2, $3, 'wecom_first', 'welcome-default', 'active', $4)
		ON CONFLICT (id) DO UPDATE SET
			store_id = EXCLUDED.store_id,
			staff_id = EXCLUDED.staff_id,
			status = 'active',
			qr_payload = EXCLUDED.qr_payload
	`, "code-"+guideID, store.ID, guideID, code); err != nil {
		return Guide{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO guide_events (id, guide_id, action, result, operator, people, detail)
		VALUES ($1, $2, '添加接待导购', '生成导购活码', '店长', $3, $4)
	`, newID("gev"), guideID, "导购："+name, code+" 已进入物料码轮巡池。"); err != nil {
		return Guide{}, err
	}
	if err := tx.Commit(); err != nil {
		return Guide{}, err
	}
	return api.guideDB(ctx, guideID)
}

func (api *API) guideDB(ctx context.Context, id string) (Guide, error) {
	guides, err := api.guidesDB(ctx, ` WHERE g.id = $1 OR g.name = $1`, []any{id}, 1, 1)
	if err != nil {
		return Guide{}, err
	}
	if len(guides) == 0 {
		return Guide{}, sql.ErrNoRows
	}
	return guides[0], nil
}

func (api *API) guidesDB(ctx context.Context, where string, args []any, page, pageSize int) ([]Guide, error) {
	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := api.db.QueryContext(ctx, `
		SELECT
			g.id, g.store_id, g.name, g.code, g.status, g.employment_status, g.paused, g.handling,
			COUNT(DISTINCT c.id)::int,
			COUNT(DISTINCT CASE WHEN c.created_at >= CURRENT_DATE THEN c.id END)::int
		FROM guides g
		JOIN stores s ON s.id = g.store_id
		LEFT JOIN customers c ON c.owner_staff_id = g.id
		`+where+`
		GROUP BY g.id, g.store_id, g.name, g.code, g.status, g.employment_status, g.paused, g.handling, g.updated_at
		ORDER BY g.updated_at DESC, g.id
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		queryArgs...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	guides := []Guide{}
	for rows.Next() {
		var guide Guide
		var status, employmentStatus string
		if err := rows.Scan(&guide.ID, &guide.StoreID, &guide.Name, &guide.Code, &status, &employmentStatus, &guide.Paused, &guide.Handling, &guide.TotalPool, &guide.TodayPool); err != nil {
			return nil, err
		}
		guide.Status = guideStatusFromDB(status)
		guide.EmploymentStatus = guideEmploymentStatusFromDB(employmentStatus)
		guide.Count = fmt.Sprintf("%d / %d", guide.TodayPool, guide.TotalPool)
		guides = append(guides, guide)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return guides, nil
}

func (api *API) guideEventsDB(ctx context.Context, guideID string) ([]AuditEvent, error) {
	rows, err := api.db.QueryContext(ctx, `
		SELECT id,
			to_char(occurred_at, 'YYYY-MM-DD HH24:MI'),
			action, result, operator, people, detail
		FROM guide_events
		WHERE guide_id = $1
		ORDER BY occurred_at DESC, id DESC
	`, guideID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []AuditEvent{}
	for rows.Next() {
		var event AuditEvent
		if err := rows.Scan(&event.ID, &event.Time, &event.Action, &event.Result, &event.Operator, &event.People, &event.Detail); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (api *API) insertGuideEventDB(ctx context.Context, guideID, action, result, operator, people, detail string) error {
	_, err := api.db.ExecContext(ctx, `
		INSERT INTO guide_events (id, guide_id, action, result, operator, people, detail)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, newID("gev"), guideID, action, result, operator, people, detail)
	return err
}

func guideDBFilters(r *http.Request, scope, storeID string) (string, []any) {
	query := r.URL.Query()
	args := []any{}
	conditions := []string{}
	if scope == "store" {
		args = append(args, storeID)
		conditions = append(conditions, "g.store_id = $"+strconv.Itoa(len(args)))
	}
	if keyword := strings.TrimSpace(query.Get("keyword")); keyword != "" {
		args = append(args, "%"+keyword+"%")
		placeholder := "$" + strconv.Itoa(len(args))
		conditions = append(conditions, "(g.name ILIKE "+placeholder+" OR g.code ILIKE "+placeholder+" OR s.name ILIKE "+placeholder+")")
	}
	if status := strings.TrimSpace(query.Get("status")); status != "" && status != "全部状态" {
		if status == "暂停使用" {
			conditions = append(conditions, "g.paused = true")
		} else {
			args = append(args, guideEmploymentStatusToDB(status))
			conditions = append(conditions, "g.employment_status = $"+strconv.Itoa(len(args)))
		}
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func guideStatusFromDB(status string) string {
	switch status {
	case "removed":
		return "已移除"
	case "paused":
		return "暂停使用"
	default:
		return "在职"
	}
}

func guideEmploymentStatusFromDB(status string) string {
	if status == "removed" {
		return "已移除"
	}
	return "在职"
}

func guideEmploymentStatusToDB(status string) string {
	if status == "已移除" || strings.EqualFold(status, "removed") {
		return "removed"
	}
	return "active"
}

func (api *API) guidesHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.guidesDBHandler(w, r)
	}
	api.mu.RLock()
	defer api.mu.RUnlock()
	status := r.URL.Query().Get("status")
	keyword := r.URL.Query().Get("keyword")
	rows := filter(api.guides, func(g Guide) bool {
		statusOK := status == "" || status == "全部状态" || g.EmploymentStatus == status || (status == "暂停使用" && g.Paused)
		return statusOK && match(keyword, g.Name+g.Code)
	})
	return writePaginatedOrList(w, r, rows)
}

func (api *API) guideActionHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/guides/")
	if len(parts) < 2 {
		return errNotFound
	}
	if api.db != nil {
		return api.guideActionDBHandler(w, r, parts)
	}
	guideID := parts[0]
	action := parts[1]
	api.mu.Lock()
	defer api.mu.Unlock()
	guide, ok := api.findGuide(guideID)
	if !ok {
		return errNotFound
	}
	switch action {
	case "pause":
		guide.Paused = !guide.Paused
		if guide.Paused {
			guide.Handling = "已暂停使用，物料码轮巡不会分配给该导购"
			guide.Lifecycle = prependAudit(guide.Lifecycle, audit(stamp(), "暂停使用", "暂停轮巡", "店长", "导购："+guide.Name, "不改写存量客户来源和服务关系。"))
		} else {
			guide.Handling = "正常服务"
			guide.Lifecycle = prependAudit(guide.Lifecycle, audit(stamp(), "恢复使用", "恢复轮巡", "店长", "导购："+guide.Name, "导购重新进入物料码轮巡池。"))
		}
		return writeJSON(w, guide)
	case "remove":
		guide.EmploymentStatus = "已移除"
		guide.Status = "已移除"
		guide.Paused = true
		guide.Handling = "客户关系保留，后续不再分配新客"
		guide.Lifecycle = prependAudit(guide.Lifecycle, audit(stamp(), "移除成员", "停用可恢复", "店长", "导购："+guide.Name, "客户关系保留，活码退出物料码轮巡。"))
		return writeJSON(w, guide)
	case "lifecycle":
		return writeJSON(w, guide.Lifecycle)
	case "qr-download":
		return writeJSON(w, makeQRDownload(guide.Code, "导购活码"))
	default:
		return errNotFound
	}
}
