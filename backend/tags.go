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

func (api *API) tagsDBHandler(w http.ResponseWriter, r *http.Request) error {
	cacheKey := requestCacheKey("tags", r)
	if api.serveCachedJSON(w, r, cacheKey) {
		return nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if r.Method == http.MethodPost {
		var tag Tag
		if err := decode(r, &tag); err != nil {
			return err
		}
		tag.Name = strings.TrimSpace(tag.Name)
		if tag.Name == "" {
			return fmt.Errorf("%w: tag name is required", errBadRequest)
		}
		if tag.Group == "" {
			tag.Group = "客户状态"
		}
		tag.Status = defaultString(tag.Status, "正常")

		tx, err := api.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		groupID, err := ensureDBTagGroup(ctx, tx, tag.Group, "全部门店", 0)
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `
			INSERT INTO tags (id, tag_group_id, name, status, wecom_tag_id)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (tag_group_id, name) DO UPDATE SET
				status = EXCLUDED.status,
				wecom_tag_id = CASE
					WHEN EXCLUDED.wecom_tag_id = '' THEN tags.wecom_tag_id
					ELSE EXCLUDED.wecom_tag_id
				END,
				updated_at = now()
			RETURNING id
		`, newID("tag"), groupID, tag.Name, tagStatusToDB(tag.Status), tag.Wecom).Scan(&tag.ID)
		if err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		api.invalidateCache("tags:", "tag-groups:", "tag-customers:")

		stored, err := api.tagDB(ctx, tag.ID)
		if err != nil {
			return err
		}
		return writeJSON(w, stored)
	}

	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		page, pageSize = 1, 500
	}
	where, args := tagDBFilters(r)

	var total int
	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM tags t JOIN tag_groups g ON g.id = t.tag_group_id `+where, args...).Scan(&total); err != nil {
		return err
	}

	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := api.db.QueryContext(ctx, `
		SELECT
			t.id,
			t.name,
			g.name,
			t.status,
			t.wecom_tag_id,
			COUNT(ct.customer_id)::int
		FROM tags t
		JOIN tag_groups g ON g.id = t.tag_group_id
		LEFT JOIN customer_tags ct ON ct.tag_id = t.id
		`+where+`
		GROUP BY t.id, t.name, g.name, g.display_order, t.status, t.wecom_tag_id, t.created_at
		ORDER BY g.display_order ASC, g.name ASC, t.created_at DESC, t.id DESC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		queryArgs...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	tags := []Tag{}
	for rows.Next() {
		var tag Tag
		var status string
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Group, &status, &tag.Wecom, &tag.Customers); err != nil {
			return err
		}
		tag.Status = tagStatusFromDB(status)
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !paged {
		return api.writeCachedJSON(w, r, cacheKey, tags)
	}
	return api.writeCachedJSON(w, r, cacheKey, PageResult[Tag]{
		Data: tags,
		Page: makePageMeta(page, pageSize, total),
	})
}

func (api *API) tagActionDBHandler(w http.ResponseWriter, r *http.Request, parts []string) error {
	if len(parts) < 2 {
		return errNotFound
	}
	key := parts[0]
	action := parts[1]

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	tag, err := api.tagDB(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}

	switch action {
	case "toggle":
		nextStatus := "active"
		if tag.Status == "正常" {
			nextStatus = "disabled"
		}
		if _, err := api.db.ExecContext(ctx, `UPDATE tags SET status = $1, updated_at = now() WHERE id = $2`, nextStatus, tag.ID); err != nil {
			return err
		}
		api.invalidateCache("tags:", "tag-groups:", "tag-customers:")
		updated, err := api.tagDB(ctx, tag.ID)
		if err != nil {
			return err
		}
		return writeJSON(w, updated)
	case "rename":
		var payload struct {
			Name string `json:"name"`
		}
		if err := decode(r, &payload); err != nil {
			return err
		}
		payload.Name = strings.TrimSpace(payload.Name)
		if payload.Name != "" {
			if _, err := api.db.ExecContext(ctx, `UPDATE tags SET name = $1, updated_at = now() WHERE id = $2`, payload.Name, tag.ID); err != nil {
				var pqErr *pq.Error
				if errors.As(err, &pqErr) && pqErr.Code == "23505" {
					return fmt.Errorf("%w: tag name already exists in this group", errBadRequest)
				}
				return err
			}
		}
		api.invalidateCache("tags:", "tag-groups:", "tag-customers:")
		updated, err := api.tagDB(ctx, tag.ID)
		if err != nil {
			return err
		}
		return writeJSON(w, updated)
	case "customers":
		return api.tagCustomersDBHandler(w, r, tag.ID)
	default:
		return errNotFound
	}
}

func (api *API) tagCustomersDBHandler(w http.ResponseWriter, r *http.Request, tagID string) error {
	cacheKey := requestCacheKey("tag-customers", r)
	if api.serveCachedJSON(w, r, cacheKey) {
		return nil
	}
	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		page, pageSize = 1, 500
	}
	where, args := tagCustomerDBFilters(r, tagID)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	var total int
	if err := api.db.QueryRowContext(ctx, `
		SELECT count(*)
		FROM customers c
		JOIN customer_tags ct ON ct.customer_id = c.id
		`+where,
		args...,
	).Scan(&total); err != nil {
		return err
	}

	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := api.db.QueryContext(ctx, `
		SELECT
			c.id, c.name, c.wecom_name, c.mobile_masked, c.owner_staff_name, c.source_store_name,
			c.lifecycle_stage, c.sales_stage, c.intent_score, c.deal_amount_cents,
			COALESCE(to_char(c.next_follow_up_at, 'YYYY-MM-DD HH24:MI'), ''),
			c.updated_at
		FROM customers c
		JOIN customer_tags ct ON ct.customer_id = c.id
		`+where+`
		ORDER BY c.updated_at DESC, c.id DESC
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

func (api *API) tagGroupsDBHandler(w http.ResponseWriter, r *http.Request) error {
	cacheKey := requestCacheKey("tag-groups", r)
	if api.serveCachedJSON(w, r, cacheKey) {
		return nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if r.Method == http.MethodPost {
		var group TagGroup
		if err := decode(r, &group); err != nil {
			return err
		}
		group.Name = strings.TrimSpace(group.Name)
		if group.Name == "" {
			return fmt.Errorf("%w: tag group name is required", errBadRequest)
		}
		hadID := group.ID != ""
		if group.ID == "" {
			group.ID = newID("tg")
		}

		tx, err := api.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		scope := strings.Join(group.Stores, "、")
		if scope == "" {
			scope = "全部门店"
		}
		upsertSQL := `
			INSERT INTO tag_groups (id, name, scope, display_order)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (name) DO UPDATE SET
				scope = EXCLUDED.scope,
				display_order = EXCLUDED.display_order,
				updated_at = now()
			RETURNING id
		`
		if hadID {
			upsertSQL = `
				INSERT INTO tag_groups (id, name, scope, display_order)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name,
					scope = EXCLUDED.scope,
					display_order = EXCLUDED.display_order,
					updated_at = now()
				RETURNING id
			`
		}
		err = tx.QueryRowContext(ctx, upsertSQL, group.ID, group.Name, scope, group.Order).Scan(&group.ID)
		if err != nil {
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "23505" {
				return fmt.Errorf("%w: tag group name already exists", errBadRequest)
			}
			return err
		}

		for _, tagName := range group.Tags {
			tagName = strings.TrimSpace(tagName)
			if tagName == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO tags (id, tag_group_id, name, status)
				VALUES ($1, $2, $3, 'active')
				ON CONFLICT (tag_group_id, name) DO UPDATE SET updated_at = now()
			`, newID("tag"), group.ID, tagName); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		api.invalidateCache("tags:", "tag-groups:", "tag-customers:")

		stored, err := api.tagGroupDB(ctx, group.ID)
		if err != nil {
			return err
		}
		return writeJSON(w, stored)
	}

	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		page, pageSize = 1, 500
	}
	where, args := tagGroupDBFilters(r)

	var total int
	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM tag_groups g `+where, args...).Scan(&total); err != nil {
		return err
	}

	offset := (page - 1) * pageSize
	queryArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := api.db.QueryContext(ctx, `
		SELECT
			g.id,
			g.name,
			g.scope,
			g.display_order,
			COALESCE(array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::text[])
		FROM tag_groups g
		LEFT JOIN tags t ON t.tag_group_id = g.id
		`+where+`
		GROUP BY g.id, g.name, g.scope, g.display_order
		ORDER BY g.display_order ASC, g.name ASC
		LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		queryArgs...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	groups := []TagGroup{}
	for rows.Next() {
		var group TagGroup
		var scope string
		if err := rows.Scan(&group.ID, &group.Name, &scope, &group.Order, pq.Array(&group.Tags)); err != nil {
			return err
		}
		group.Stores = storesFromScope(scope)
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !paged {
		return api.writeCachedJSON(w, r, cacheKey, groups)
	}
	return api.writeCachedJSON(w, r, cacheKey, PageResult[TagGroup]{
		Data: groups,
		Page: makePageMeta(page, pageSize, total),
	})
}

func (api *API) tagDB(ctx context.Context, key string) (Tag, error) {
	var tag Tag
	var status string
	err := api.db.QueryRowContext(ctx, `
		SELECT
			t.id,
			t.name,
			g.name,
			t.status,
			t.wecom_tag_id,
			COUNT(ct.customer_id)::int
		FROM tags t
		JOIN tag_groups g ON g.id = t.tag_group_id
		LEFT JOIN customer_tags ct ON ct.tag_id = t.id
		WHERE t.id = $1 OR t.name = $1
		GROUP BY t.id, t.name, g.name, g.display_order, t.status, t.wecom_tag_id, t.created_at
		ORDER BY t.created_at ASC
		LIMIT 1
	`, key).Scan(&tag.ID, &tag.Name, &tag.Group, &status, &tag.Wecom, &tag.Customers)
	if err != nil {
		return Tag{}, err
	}
	tag.Status = tagStatusFromDB(status)
	return tag, nil
}

func (api *API) tagGroupDB(ctx context.Context, id string) (TagGroup, error) {
	var group TagGroup
	var scope string
	err := api.db.QueryRowContext(ctx, `
		SELECT
			g.id,
			g.name,
			g.scope,
			g.display_order,
			COALESCE(array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL), ARRAY[]::text[])
		FROM tag_groups g
		LEFT JOIN tags t ON t.tag_group_id = g.id
		WHERE g.id = $1
		GROUP BY g.id, g.name, g.scope, g.display_order
	`, id).Scan(&group.ID, &group.Name, &scope, &group.Order, pq.Array(&group.Tags))
	if err != nil {
		return TagGroup{}, err
	}
	group.Stores = storesFromScope(scope)
	return group, nil
}

func tagDBFilters(r *http.Request) (string, []any) {
	query := r.URL.Query()
	args := []any{}
	conditions := []string{}
	if keyword := strings.TrimSpace(query.Get("keyword")); keyword != "" {
		args = append(args, "%"+keyword+"%")
		placeholder := "$" + strconv.Itoa(len(args))
		conditions = append(conditions, "(t.name ILIKE "+placeholder+" OR g.name ILIKE "+placeholder+")")
	}
	if group := strings.TrimSpace(firstNonEmpty(query.Get("group"), query.Get("tagGroup"))); group != "" && group != "全部分组" && group != "全部标签组" {
		args = append(args, group)
		conditions = append(conditions, "g.name = $"+strconv.Itoa(len(args)))
	}
	if status := strings.TrimSpace(query.Get("status")); status != "" && status != "全部状态" {
		args = append(args, tagStatusToDB(status))
		conditions = append(conditions, "t.status = $"+strconv.Itoa(len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func tagGroupDBFilters(r *http.Request) (string, []any) {
	query := r.URL.Query()
	args := []any{}
	conditions := []string{}
	if keyword := strings.TrimSpace(query.Get("keyword")); keyword != "" {
		args = append(args, "%"+keyword+"%")
		placeholder := "$" + strconv.Itoa(len(args))
		conditions = append(conditions, "(g.name ILIKE "+placeholder+" OR g.scope ILIKE "+placeholder+")")
	}
	if store := strings.TrimSpace(firstNonEmpty(query.Get("store"), query.Get("scope"))); store != "" && store != "全部门店" {
		args = append(args, "%"+store+"%")
		conditions = append(conditions, "g.scope ILIKE $"+strconv.Itoa(len(args)))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func tagCustomerDBFilters(r *http.Request, tagID string) (string, []any) {
	query := r.URL.Query()
	args := []any{tagID}
	conditions := []string{"ct.tag_id = $1"}
	if keyword := strings.TrimSpace(query.Get("keyword")); keyword != "" {
		args = append(args, "%"+keyword+"%")
		placeholder := "$" + strconv.Itoa(len(args))
		conditions = append(conditions, "(c.name ILIKE "+placeholder+" OR c.wecom_name ILIKE "+placeholder+" OR c.mobile_masked ILIKE "+placeholder+" OR c.source_store_name ILIKE "+placeholder+")")
	}
	if guide := strings.TrimSpace(query.Get("guide")); guide != "" && guide != "全部导购" {
		args = append(args, guide)
		conditions = append(conditions, "c.owner_staff_name = $"+strconv.Itoa(len(args)))
	}
	if stage := strings.TrimSpace(query.Get("stage")); stage != "" && stage != "全部阶段" {
		args = append(args, stage)
		conditions = append(conditions, "c.lifecycle_stage = $"+strconv.Itoa(len(args)))
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func ensureDBTagGroup(ctx context.Context, tx *sql.Tx, name, scope string, order int) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "客户状态"
	}
	if scope == "" {
		scope = "全部门店"
	}
	var groupID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM tag_groups WHERE name = $1`, name).Scan(&groupID)
	if err == nil {
		return groupID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	groupID = newID("tg")
	err = tx.QueryRowContext(ctx, `
		INSERT INTO tag_groups (id, name, scope, display_order)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name) DO UPDATE SET updated_at = now()
		RETURNING id
	`, groupID, name, scope, order).Scan(&groupID)
	if err != nil {
		return "", err
	}
	return groupID, nil
}

func tagStatusFromDB(status string) string {
	if status == "disabled" || status == "inactive" || status == "stopped" {
		return "停用"
	}
	return "正常"
}

func tagStatusToDB(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "停用", "disabled", "inactive", "stopped":
		return "disabled"
	default:
		return "active"
	}
}

func storesFromScope(scope string) []string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return nil
	}
	fields := strings.FieldsFunc(scope, func(r rune) bool {
		return r == '、' || r == ',' || r == '，'
	})
	stores := []string{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			stores = append(stores, field)
		}
	}
	return stores
}
