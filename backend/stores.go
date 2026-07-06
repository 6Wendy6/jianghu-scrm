package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (api *API) storesDBHandler(w http.ResponseWriter, r *http.Request) error {
	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		page, pageSize = 1, 200
	}
	where, args := storeDBFilters(r)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var total int
	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM stores s `+where, args...).Scan(&total); err != nil {
		return err
	}

	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := api.db.QueryContext(ctx, `
		SELECT
			s.id, s.name, s.internal_code, s.external_code, s.brand, s.region, s.store_type,
			s.entry_mode, s.service_guide, s.group_name, s.welcome_rule, s.polling_rule,
			COUNT(DISTINCT g.id)::int,
			COUNT(DISTINCT c.id)::int
		FROM stores s
		LEFT JOIN guides g ON g.store_id = s.id AND g.employment_status <> 'removed'
		LEFT JOIN customers c ON c.source_store_id = s.id
		`+where+`
		GROUP BY s.id, s.name, s.internal_code, s.external_code, s.brand, s.region, s.store_type,
			s.entry_mode, s.service_guide, s.group_name, s.welcome_rule, s.polling_rule, s.updated_at
		ORDER BY s.updated_at DESC, s.id
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		queryArgs...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	stores := []Store{}
	for rows.Next() {
		var store Store
		if err := rows.Scan(
			&store.ID, &store.Name, &store.InternalCode, &store.ExternalCode, &store.Brand, &store.Region, &store.Type,
			&store.Config.EntryMode, &store.Config.ServiceGuide, &store.Config.Group, &store.Config.Welcome, &store.Config.Polling,
			&store.GuideCount, &store.PoolCount,
		); err != nil {
			return err
		}
		store.BrandRegion = strings.Trim(strings.TrimSpace(store.Brand)+" / "+strings.TrimSpace(store.Region), " /")
		stores = append(stores, store)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !paged {
		return writeJSON(w, stores)
	}
	return writeJSON(w, PageResult[Store]{Data: stores, Page: makePageMeta(page, pageSize, total)})
}

func (api *API) storeConfigDBHandler(w http.ResponseWriter, r *http.Request, storeID string) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	store, err := api.storeDB(ctx, storeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}
	if r.Method == http.MethodPatch || r.Method == http.MethodPost {
		var payload StoreConfig
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.EntryMode != "" {
			store.Config.EntryMode = payload.EntryMode
		}
		if payload.ServiceGuide != "" {
			store.Config.ServiceGuide = payload.ServiceGuide
		}
		if payload.Group != "" {
			store.Config.Group = payload.Group
		}
		if payload.Welcome != "" {
			store.Config.Welcome = payload.Welcome
		}
		if payload.Polling != "" {
			store.Config.Polling = payload.Polling
		}
		if _, err := api.db.ExecContext(ctx, `
			UPDATE stores
			SET entry_mode = $2, service_guide = $3, group_name = $4,
				welcome_rule = $5, polling_rule = $6, updated_at = now()
			WHERE id = $1
		`, store.ID, store.Config.EntryMode, store.Config.ServiceGuide, store.Config.Group, store.Config.Welcome, store.Config.Polling); err != nil {
			return err
		}
		api.invalidateCache("summary:")
	}
	return writeJSON(w, store.Config)
}

func storeDBFilters(r *http.Request) (string, []any) {
	query := r.URL.Query()
	args := []any{}
	conditions := []string{}
	if keyword := strings.TrimSpace(query.Get("keyword")); keyword != "" {
		args = append(args, "%"+keyword+"%")
		placeholder := "$" + strconv.Itoa(len(args))
		conditions = append(conditions, "(s.name ILIKE "+placeholder+" OR s.internal_code ILIKE "+placeholder+" OR s.external_code ILIKE "+placeholder+" OR s.brand ILIKE "+placeholder+" OR s.region ILIKE "+placeholder+")")
	}
	if brand := strings.TrimSpace(query.Get("brand")); brand != "" && brand != "全部品牌" {
		args = append(args, brand)
		conditions = append(conditions, "s.brand = $"+strconv.Itoa(len(args)))
	}
	if region := strings.TrimSpace(query.Get("region")); region != "" && region != "全部区域" {
		args = append(args, region)
		conditions = append(conditions, "s.region = $"+strconv.Itoa(len(args)))
	}
	if storeType := strings.TrimSpace(query.Get("type")); storeType != "" && storeType != "全部类型" {
		args = append(args, storeType)
		conditions = append(conditions, "s.store_type = $"+strconv.Itoa(len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func storeCodePrefix(store Store) string {
	if strings.Contains(store.Name, "南山") {
		return "南山店"
	}
	if strings.Contains(store.Name, "天河") {
		return "天河店"
	}
	if strings.Contains(store.Name, "杭州") || strings.Contains(store.Name, "湖滨") {
		return "杭州店"
	}
	if store.Name == "" {
		return "门店"
	}
	return store.Name
}

func relationCodePrefix(storeName string) string {
	storeName = strings.TrimSpace(storeName)
	if strings.Contains(storeName, "南山") {
		return "南山店"
	}
	if strings.Contains(storeName, "天河") {
		return "天河店"
	}
	if strings.Contains(storeName, "杭州") || strings.Contains(storeName, "湖滨") {
		return "杭州店"
	}
	if storeName == "" {
		return "门店"
	}
	return storeName
}

func (api *API) storesHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.storesDBHandler(w, r)
	}
	api.mu.RLock()
	defer api.mu.RUnlock()
	keyword := r.URL.Query().Get("keyword")
	brand := r.URL.Query().Get("brand")
	region := r.URL.Query().Get("region")
	storeType := r.URL.Query().Get("type")
	rows := filter(api.stores, func(s Store) bool {
		text := s.Name + s.InternalCode + s.ExternalCode + s.BrandRegion + s.Type
		return match(keyword, text) && matchOption(brand, "全部品牌", s.Brand) && matchOption(region, "全部区域", s.Region) && matchOption(storeType, "全部类型", s.Type)
	})
	return writePaginatedOrList(w, r, rows)
}

func (api *API) storeActionHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/stores/")
	if len(parts) == 0 {
		return errNotFound
	}
	storeID := parts[0]
	switch {
	case len(parts) == 2 && parts[1] == "config":
		return api.storeConfigHandler(w, r, storeID)
	case len(parts) == 2 && parts[1] == "guides":
		return api.storeGuidesHandler(w, r, storeID)
	case len(parts) == 3 && parts[1] == "handover" && parts[2] == "sync":
		return api.syncHandover(w, r, storeID)
	case len(parts) == 3 && parts[1] == "handover" && parts[2] == "submit":
		return api.submitHandover(w, r, storeID)
	default:
		return errNotFound
	}
}

func (api *API) storeConfigHandler(w http.ResponseWriter, r *http.Request, storeID string) error {
	if api.db != nil {
		return api.storeConfigDBHandler(w, r, storeID)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	store, ok := api.findStore(storeID)
	if !ok {
		return errNotFound
	}
	if r.Method == http.MethodPatch || r.Method == http.MethodPost {
		var payload StoreConfig
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.EntryMode != "" {
			store.Config.EntryMode = payload.EntryMode
		}
		if payload.ServiceGuide != "" {
			store.Config.ServiceGuide = payload.ServiceGuide
		}
		if payload.Group != "" {
			store.Config.Group = payload.Group
		}
		if payload.Welcome != "" {
			store.Config.Welcome = payload.Welcome
		}
		if payload.Polling != "" {
			store.Config.Polling = payload.Polling
		}
	}
	return writeJSON(w, store.Config)
}

func (api *API) storeGuidesHandler(w http.ResponseWriter, r *http.Request, storeID string) error {
	if api.db != nil {
		return api.storeGuidesDBHandler(w, r, storeID)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method == http.MethodPost {
		var payload struct {
			Name string `json:"name"`
		}
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.Name == "" {
			return fmt.Errorf("%w: guide name is required", errBadRequest)
		}
		guide := Guide{ID: newID("g"), StoreID: storeID, Name: payload.Name, Code: "南山店-" + payload.Name, Count: "0 / 0", Status: "在职", EmploymentStatus: "在职", Handling: "正常服务"}
		guide.Lifecycle = []AuditEvent{audit(stamp(), "添加接待导购", "生成导购活码", "店长", "导购："+payload.Name, guide.Code+" 已进入物料码轮巡池。")}
		api.guides = append(api.guides, guide)
		return writeJSON(w, guide)
	}
	rows := filter(api.guides, func(g Guide) bool { return g.StoreID == storeID })
	return writePaginatedOrList(w, r, rows)
}
