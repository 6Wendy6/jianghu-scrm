package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (api *API) touchesDBHandler(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if r.Method == http.MethodPost {
		var payload TouchRule
		if err := decode(r, &payload); err != nil {
			return err
		}
		payload.Name = strings.TrimSpace(payload.Name)
		if payload.Name == "" {
			return fmt.Errorf("%w: touch name is required", errBadRequest)
		}
		if payload.ID == "" {
			payload.ID = newID("t")
		}
		payload.Status = defaultString(payload.Status, "启用")
		if _, err := api.db.ExecContext(ctx, `
			INSERT INTO touch_rules (id, name, rule_type, scope, status)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				rule_type = EXCLUDED.rule_type,
				scope = EXCLUDED.scope,
				status = EXCLUDED.status,
				updated_at = now()
		`, payload.ID, payload.Name, payload.Type, payload.Scope, payload.Status); err != nil {
			return err
		}
		return writeJSON(w, payload)
	}
	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		page, pageSize = 1, 500
	}
	var total int
	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM touch_rules`).Scan(&total); err != nil {
		return err
	}
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, name, rule_type, scope, status
		FROM touch_rules
		ORDER BY updated_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, pageSize, (page-1)*pageSize)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []TouchRule{}
	for rows.Next() {
		var item TouchRule
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.Scope, &item.Status); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !paged {
		return writeJSON(w, items)
	}
	return writeJSON(w, PageResult[TouchRule]{Data: items, Page: makePageMeta(page, pageSize, total)})
}

func (api *API) groupsDBHandler(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if r.Method == http.MethodPost {
		var payload CustomerGroup
		if err := decode(r, &payload); err != nil {
			return err
		}
		payload.Name = strings.TrimSpace(payload.Name)
		if payload.Name == "" {
			return fmt.Errorf("%w: group name is required", errBadRequest)
		}
		if payload.ID == "" {
			payload.ID = newID("cg")
		}
		payload.Status = defaultString(payload.Status, "运营中")
		tagsJSON, err := jsonStringArray(payload.Tags)
		if err != nil {
			return err
		}
		if _, err := api.db.ExecContext(ctx, `
			INSERT INTO customer_groups (id, name, store_name, owner_name, tags, member_count, today_join, today_quit, status, today_event)
			VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $9, $10)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				store_name = EXCLUDED.store_name,
				owner_name = EXCLUDED.owner_name,
				tags = EXCLUDED.tags,
				member_count = EXCLUDED.member_count,
				today_join = EXCLUDED.today_join,
				today_quit = EXCLUDED.today_quit,
				status = EXCLUDED.status,
				today_event = EXCLUDED.today_event,
				updated_at = now()
		`, payload.ID, payload.Name, payload.Store, payload.Owner, tagsJSON, payload.Count, payload.TodayJoin, payload.TodayQuit, payload.Status, payload.TodayEvent); err != nil {
			return err
		}
		return writeJSON(w, payload)
	}
	page, pageSize, paged, err := paginationParams(r)
	if err != nil {
		return err
	}
	if !paged {
		page, pageSize = 1, 500
	}
	var total int
	if err := api.db.QueryRowContext(ctx, `SELECT count(*) FROM customer_groups`).Scan(&total); err != nil {
		return err
	}
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, name, store_name, owner_name, tags::text, member_count, today_join, today_quit,
			to_char(created_at, 'YYYY-MM-DD HH24:MI'), status, today_event
		FROM customer_groups
		ORDER BY updated_at DESC, id DESC
		LIMIT $1 OFFSET $2
	`, pageSize, (page-1)*pageSize)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []CustomerGroup{}
	for rows.Next() {
		var item CustomerGroup
		var tags string
		if err := rows.Scan(&item.ID, &item.Name, &item.Store, &item.Owner, &tags, &item.Count, &item.TodayJoin, &item.TodayQuit, &item.CreatedAt, &item.Status, &item.TodayEvent); err != nil {
			return err
		}
		item.Tags = stringsFromJSONArray(tags)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !paged {
		return writeJSON(w, items)
	}
	return writeJSON(w, PageResult[CustomerGroup]{Data: items, Page: makePageMeta(page, pageSize, total)})
}

func (api *API) groupActionDBHandler(w http.ResponseWriter, r *http.Request, parts []string) error {
	switch parts[0] {
	case "mass":
		return api.groupMassTasksDBHandler(w, r)
	case "welcomes":
		return api.groupWelcomesDBHandler(w, r)
	case "sops":
		return api.groupSOPsDBHandler(w, r)
	case "calendar":
		return api.groupCalendarDBHandler(w, r)
	case "reminders":
		return api.groupRemindersDBHandler(w, r)
	case "tag-groups":
		return api.groupTagGroupsDBHandler(w, r)
	default:
		return errNotFound
	}
}

func (api *API) groupMassTasksDBHandler(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if r.Method == http.MethodPost {
		var item GroupMassTask
		if err := decode(r, &item); err != nil {
			return err
		}
		if item.ID == "" {
			item.ID = newID("gm")
		}
		item.Status = defaultString(item.Status, "待群主确认")
		groupsJSON, err := jsonStringArray(item.Groups)
		if err != nil {
			return err
		}
		if _, err := api.db.ExecContext(ctx, `
			INSERT INTO group_mass_tasks (id, name, groups, content, status)
			VALUES ($1, $2, $3::jsonb, $4, $5)
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, groups = EXCLUDED.groups, content = EXCLUDED.content, status = EXCLUDED.status, updated_at = now()
		`, item.ID, item.Name, groupsJSON, item.Content, item.Status); err != nil {
			return err
		}
		return writeJSON(w, item)
	}
	rows, err := api.db.QueryContext(ctx, `SELECT id, name, groups::text, content, status FROM group_mass_tasks ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []GroupMassTask{}
	for rows.Next() {
		var item GroupMassTask
		var groups string
		if err := rows.Scan(&item.ID, &item.Name, &groups, &item.Content, &item.Status); err != nil {
			return err
		}
		item.Groups = stringsFromJSONArray(groups)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writePaginatedOrList(w, r, items)
}

func (api *API) groupWelcomesDBHandler(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if r.Method == http.MethodPost {
		var item GroupWelcome
		if err := decode(r, &item); err != nil {
			return err
		}
		if item.ID == "" {
			item.ID = newID("gw")
		}
		item.Status = defaultString(item.Status, "启用")
		if _, err := api.db.ExecContext(ctx, `
			INSERT INTO group_welcomes (id, group_name, content, status)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO UPDATE SET group_name = EXCLUDED.group_name, content = EXCLUDED.content, status = EXCLUDED.status, updated_at = now()
		`, item.ID, item.Group, item.Content, item.Status); err != nil {
			return err
		}
		return writeJSON(w, item)
	}
	rows, err := api.db.QueryContext(ctx, `SELECT id, group_name, content, status FROM group_welcomes ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []GroupWelcome{}
	for rows.Next() {
		var item GroupWelcome
		if err := rows.Scan(&item.ID, &item.Group, &item.Content, &item.Status); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writePaginatedOrList(w, r, items)
}

func (api *API) groupSOPsDBHandler(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if r.Method == http.MethodPost {
		var item GroupSOP
		if err := decode(r, &item); err != nil {
			return err
		}
		if item.ID == "" {
			item.ID = newID("gs")
		}
		item.Status = defaultString(item.Status, "启用")
		groupsJSON, err := jsonStringArray(item.Groups)
		if err != nil {
			return err
		}
		if _, err := api.db.ExecContext(ctx, `
			INSERT INTO group_sops (id, name, groups, stage, content, status)
			VALUES ($1, $2, $3::jsonb, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, groups = EXCLUDED.groups, stage = EXCLUDED.stage, content = EXCLUDED.content, status = EXCLUDED.status, updated_at = now()
		`, item.ID, item.Name, groupsJSON, item.Stage, item.Content, item.Status); err != nil {
			return err
		}
		return writeJSON(w, item)
	}
	rows, err := api.db.QueryContext(ctx, `SELECT id, name, groups::text, stage, content, status FROM group_sops ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []GroupSOP{}
	for rows.Next() {
		var item GroupSOP
		var groups string
		if err := rows.Scan(&item.ID, &item.Name, &groups, &item.Stage, &item.Content, &item.Status); err != nil {
			return err
		}
		item.Groups = stringsFromJSONArray(groups)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writePaginatedOrList(w, r, items)
}

func (api *API) groupCalendarDBHandler(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if r.Method == http.MethodPost {
		var item GroupCalendarEvent
		if err := decode(r, &item); err != nil {
			return err
		}
		if item.ID == "" {
			item.ID = newID("gc")
		}
		item.Status = defaultString(item.Status, "待执行")
		if _, err := api.db.ExecContext(ctx, `
			INSERT INTO group_calendar_events (id, group_name, title, event_date, owner_name, status)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET group_name = EXCLUDED.group_name, title = EXCLUDED.title, event_date = EXCLUDED.event_date, owner_name = EXCLUDED.owner_name, status = EXCLUDED.status, updated_at = now()
		`, item.ID, item.Group, item.Title, item.Date, item.Owner, item.Status); err != nil {
			return err
		}
		return writeJSON(w, item)
	}
	rows, err := api.db.QueryContext(ctx, `SELECT id, group_name, title, event_date, owner_name, status FROM group_calendar_events ORDER BY event_date ASC, updated_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []GroupCalendarEvent{}
	for rows.Next() {
		var item GroupCalendarEvent
		if err := rows.Scan(&item.ID, &item.Group, &item.Title, &item.Date, &item.Owner, &item.Status); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writePaginatedOrList(w, r, items)
}

func (api *API) groupRemindersDBHandler(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if r.Method == http.MethodPost {
		var item GroupReminder
		if err := decode(r, &item); err != nil {
			return err
		}
		if item.ID == "" {
			item.ID = newID("gr")
		}
		item.Status = defaultString(item.Status, "启用")
		if _, err := api.db.ExecContext(ctx, `
			INSERT INTO group_reminders (id, name, trigger_text, owner_name, status)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, trigger_text = EXCLUDED.trigger_text, owner_name = EXCLUDED.owner_name, status = EXCLUDED.status, updated_at = now()
		`, item.ID, item.Name, item.Trigger, item.Owner, item.Status); err != nil {
			return err
		}
		return writeJSON(w, item)
	}
	rows, err := api.db.QueryContext(ctx, `SELECT id, name, trigger_text, owner_name, status FROM group_reminders ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []GroupReminder{}
	for rows.Next() {
		var item GroupReminder
		if err := rows.Scan(&item.ID, &item.Name, &item.Trigger, &item.Owner, &item.Status); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writePaginatedOrList(w, r, items)
}

func (api *API) groupTagGroupsDBHandler(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if r.Method == http.MethodPost {
		var item GroupTagGroup
		if err := decode(r, &item); err != nil {
			return err
		}
		if item.ID == "" {
			item.ID = newID("gt")
		}
		item.Status = defaultString(item.Status, "正常")
		tagsJSON, err := jsonStringArray(item.Tags)
		if err != nil {
			return err
		}
		if _, err := api.db.ExecContext(ctx, `
			INSERT INTO group_tag_groups (id, name, tags, scope, owner_name, status)
			VALUES ($1, $2, $3::jsonb, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, tags = EXCLUDED.tags, scope = EXCLUDED.scope, owner_name = EXCLUDED.owner_name, status = EXCLUDED.status, updated_at = now()
		`, item.ID, item.Name, tagsJSON, item.Scope, item.Owner, item.Status); err != nil {
			return err
		}
		return writeJSON(w, item)
	}
	rows, err := api.db.QueryContext(ctx, `SELECT id, name, tags::text, scope, owner_name, status FROM group_tag_groups ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []GroupTagGroup{}
	for rows.Next() {
		var item GroupTagGroup
		var tags string
		if err := rows.Scan(&item.ID, &item.Name, &tags, &item.Scope, &item.Owner, &item.Status); err != nil {
			return err
		}
		item.Tags = stringsFromJSONArray(tags)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writePaginatedOrList(w, r, items)
}

func (api *API) autoRulesDBHandler(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if r.Method == http.MethodPost {
		var item AutoTagRule
		if err := decode(r, &item); err != nil {
			return err
		}
		if item.ID == "" {
			item.ID = newID("ar")
		}
		item.Status = defaultString(item.Status, "启用")
		item.Impact = max(item.Impact, 92)
		actionsJSON, err := jsonStringArray(item.Actions)
		if err != nil {
			return err
		}
		if _, err := api.db.ExecContext(ctx, `
			INSERT INTO auto_tag_rules (id, name, trigger_text, actions, scope, impact, status)
			VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, trigger_text = EXCLUDED.trigger_text, actions = EXCLUDED.actions, scope = EXCLUDED.scope, impact = EXCLUDED.impact, status = EXCLUDED.status, updated_at = now()
		`, item.ID, item.Name, item.Trigger, actionsJSON, item.Scope, item.Impact, item.Status); err != nil {
			return err
		}
		return writeJSON(w, item)
	}
	rows, err := api.db.QueryContext(ctx, `SELECT id, name, trigger_text, actions::text, scope, impact, status FROM auto_tag_rules ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []AutoTagRule{}
	for rows.Next() {
		var item AutoTagRule
		var actions string
		if err := rows.Scan(&item.ID, &item.Name, &item.Trigger, &actions, &item.Scope, &item.Impact, &item.Status); err != nil {
			return err
		}
		item.Actions = stringsFromJSONArray(actions)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writePaginatedOrList(w, r, items)
}

func (api *API) preTagRulesDBHandler(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if r.Method == http.MethodPost {
		var item PreTagRule
		if err := decode(r, &item); err != nil {
			return err
		}
		if item.ID == "" {
			item.ID = newID("pr")
		}
		item.Status = defaultString(item.Status, "启用")
		tagsJSON, err := jsonStringArray(item.Tags)
		if err != nil {
			return err
		}
		if _, err := api.db.ExecContext(ctx, `
			INSERT INTO pre_tag_rules (id, entry, tags, trigger_text, period, status)
			VALUES ($1, $2, $3::jsonb, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET entry = EXCLUDED.entry, tags = EXCLUDED.tags, trigger_text = EXCLUDED.trigger_text, period = EXCLUDED.period, status = EXCLUDED.status, updated_at = now()
		`, item.ID, item.Entry, tagsJSON, item.Trigger, item.Period, item.Status); err != nil {
			return err
		}
		return writeJSON(w, item)
	}
	rows, err := api.db.QueryContext(ctx, `SELECT id, entry, tags::text, trigger_text, period, status FROM pre_tag_rules ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []PreTagRule{}
	for rows.Next() {
		var item PreTagRule
		var tags string
		if err := rows.Scan(&item.ID, &item.Entry, &tags, &item.Trigger, &item.Period, &item.Status); err != nil {
			return err
		}
		item.Tags = stringsFromJSONArray(tags)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writePaginatedOrList(w, r, items)
}

func (api *API) touchesHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.touchesDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method == http.MethodPost {
		var payload TouchRule
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.Name == "" {
			return fmt.Errorf("%w: touch name is required", errBadRequest)
		}
		payload.ID = newID("t")
		if payload.Status == "" {
			payload.Status = "启用"
		}
		api.touches = append([]TouchRule{payload}, api.touches...)
		return writeJSON(w, payload)
	}
	return writePaginatedOrList(w, r, api.touches)
}

func (api *API) groupsHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.groupsDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method == http.MethodPost {
		var payload CustomerGroup
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.Name == "" {
			return fmt.Errorf("%w: group name is required", errBadRequest)
		}
		payload.ID = newID("cg")
		payload.CreatedAt = stamp()
		payload.Status = "运营中"
		api.groups = append([]CustomerGroup{payload}, api.groups...)
		return writeJSON(w, payload)
	}
	return writePaginatedOrList(w, r, api.groups)
}

func (api *API) groupActionHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/groups/")
	if len(parts) == 0 {
		return errNotFound
	}
	if api.db != nil {
		return api.groupActionDBHandler(w, r, parts)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	switch parts[0] {
	case "mass":
		return createOrList(w, r, &api.groupMassTasks, func(item *GroupMassTask) {
			item.ID = newID("gm")
			item.Status = defaultString(item.Status, "待群主确认")
		})
	case "welcomes":
		return createOrList(w, r, &api.groupWelcomes, func(item *GroupWelcome) {
			item.ID = newID("gw")
			item.Status = defaultString(item.Status, "启用")
		})
	case "sops":
		return createOrList(w, r, &api.groupSOPs, func(item *GroupSOP) {
			item.ID = newID("gs")
			item.Status = defaultString(item.Status, "启用")
		})
	case "calendar":
		return createOrList(w, r, &api.groupCalendar, func(item *GroupCalendarEvent) {
			item.ID = newID("gc")
			item.Status = defaultString(item.Status, "待执行")
		})
	case "reminders":
		return createOrList(w, r, &api.groupReminders, func(item *GroupReminder) {
			item.ID = newID("gr")
			item.Status = defaultString(item.Status, "启用")
		})
	case "tag-groups":
		return createOrList(w, r, &api.groupTagGroups, func(item *GroupTagGroup) {
			item.ID = newID("gt")
			item.Status = defaultString(item.Status, "正常")
		})
	default:
		return errNotFound
	}
}

func (api *API) tagsHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.tagsDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method == http.MethodPost {
		var tag Tag
		if err := decode(r, &tag); err != nil {
			return err
		}
		if tag.Name == "" {
			return fmt.Errorf("%w: tag name is required", errBadRequest)
		}
		tag.ID = newID("tag")
		tag.Status = defaultString(tag.Status, "正常")
		api.tags = append([]Tag{tag}, api.tags...)
		return writeJSON(w, tag)
	}
	return writePaginatedOrList(w, r, api.tags)
}

func (api *API) tagActionHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/tags/")
	if len(parts) < 2 {
		return errNotFound
	}
	if api.db != nil {
		return api.tagActionDBHandler(w, r, parts)
	}
	name, _ := url.PathUnescape(parts[0])
	action := parts[1]
	api.mu.Lock()
	defer api.mu.Unlock()
	for i := range api.tags {
		if api.tags[i].Name != name && api.tags[i].ID != name {
			continue
		}
		switch action {
		case "toggle":
			if api.tags[i].Status == "正常" {
				api.tags[i].Status = "停用"
			} else {
				api.tags[i].Status = "正常"
			}
			return writeJSON(w, api.tags[i])
		case "rename":
			var payload struct {
				Name string `json:"name"`
			}
			if err := decode(r, &payload); err != nil {
				return err
			}
			if payload.Name != "" {
				api.tags[i].Name = payload.Name
			}
			return writeJSON(w, api.tags[i])
		case "customers":
			rows := filter(api.customers, func(c Customer) bool { return contains(c.Tags, api.tags[i].Name) })
			return writePaginatedOrList(w, r, rows)
		default:
			return errNotFound
		}
	}
	return errNotFound
}

func (api *API) tagGroupsHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.tagGroupsDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method == http.MethodPost {
		var group TagGroup
		if err := decode(r, &group); err != nil {
			return err
		}
		if group.Name == "" {
			return fmt.Errorf("%w: tag group name is required", errBadRequest)
		}
		if group.ID == "" {
			group.ID = newID("tg")
		}
		for i := range api.tagGroups {
			if api.tagGroups[i].ID == group.ID || api.tagGroups[i].Name == group.Name {
				api.tagGroups[i] = group
				return writeJSON(w, group)
			}
		}
		api.tagGroups = append(api.tagGroups, group)
		return writeJSON(w, group)
	}
	return writePaginatedOrList(w, r, api.tagGroups)
}

func (api *API) autoRulesHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.autoRulesDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method == http.MethodPost {
		var rule AutoTagRule
		if err := decode(r, &rule); err != nil {
			return err
		}
		rule.ID = newID("ar")
		rule.Status = defaultString(rule.Status, "启用")
		rule.Impact = max(rule.Impact, 92)
		api.autoRules = append([]AutoTagRule{rule}, api.autoRules...)
		return writeJSON(w, rule)
	}
	return writePaginatedOrList(w, r, api.autoRules)
}

func (api *API) preTagRulesHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.preTagRulesDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method == http.MethodPost {
		var rule PreTagRule
		if err := decode(r, &rule); err != nil {
			return err
		}
		rule.ID = newID("pr")
		rule.Status = defaultString(rule.Status, "启用")
		api.preTagRules = append([]PreTagRule{rule}, api.preTagRules...)
		return writeJSON(w, rule)
	}
	return writePaginatedOrList(w, r, api.preTagRules)
}
