package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/lib/pq"
	"strings"
)

func (api *API) changeOpsCustomerStageDB(ctx context.Context, rCustomer OpsCustomer, stage, reason, operatorID, operatorName string) error {
	if stage == "" {
		return fmt.Errorf("%w: stage is required", errBadRequest)
	}
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := api.changeOpsCustomerStageTx(ctx, tx, rCustomer, stage, reason, operatorID, operatorName); err != nil {
		return err
	}
	return tx.Commit()
}

func (api *API) changeOpsCustomerStageTx(ctx context.Context, tx *sql.Tx, customer OpsCustomer, stage, reason, operatorID, operatorName string) error {
	before := customer.LifecycleStage
	nowStatus := opsStageLabel(stage)
	operatorID = firstNonEmpty(operatorID, opsOperatorID)
	operatorName = firstNonEmpty(operatorName, opsOperatorName)
	reason = firstNonEmpty(reason, "手动调整生命周期")
	if _, err := tx.ExecContext(ctx, `
		UPDATE customers
		SET lifecycle_stage = $2,
			status = $3,
			updated_at = now()
		WHERE id = $1
	`, customer.ID, stage, nowStatus); err != nil {
		return err
	}
	logID := newID("cl")
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO customer_lifecycle_logs (id, customer_id, stage_before, stage_after, reason, operator_id, operator_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, logID, customer.ID, before, stage, reason, operatorID, operatorName); err != nil {
		return err
	}
	if err := api.addOpsTimelineTx(ctx, tx, customer.ID, "lifecycle", "生命周期变更", opsStageLabel(before)+" -> "+opsStageLabel(stage)+"；原因："+reason, logID, operatorID, operatorName); err != nil {
		return err
	}
	if stage == "high_intent" {
		var exists bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM sop_tasks
				WHERE customer_id = $1 AND task_type = '高意向客户回访'
					AND status NOT IN ('completed', 'ignored', 'cancelled')
			)
		`, customer.ID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			task := OpsTask{TaskType: "高意向客户回访", Title: "24小时内回访" + customer.Name, Description: "客户被标记为高意向，自动生成回访任务。", CustomerID: customer.ID, CustomerName: customer.Name, RegionID: customer.RegionID, RegionName: customer.RegionName, StoreID: customer.StoreID, StoreName: customer.StoreName, AssignedToUserID: customer.OwnerGuideID, AssignedToName: customer.OwnerGuideName, AssignedToRole: "guide", Priority: "P1", DueTime: "2026-07-07 18:00", Source: "阶段流转"}
			if _, err := api.createOpsTaskTx(ctx, tx, task); err != nil {
				return err
			}
		}
	}
	return nil
}

func (api *API) addOpsCustomerTagDB(ctx context.Context, customer OpsCustomer, tagID, name, category, source, operatorID, operatorName string) error {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tag, err := api.ensureOpsDBTag(ctx, tx, tagID, name, category, source)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO customer_tags (customer_id, tag_id, source, operator_id, relation_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (customer_id, tag_id) DO NOTHING
	`, customer.ID, tag.ID, firstNonEmpty(source, "manual"), firstNonEmpty(operatorID, opsOperatorID), newID("link")); err != nil {
		return err
	}
	if err := api.addOpsTimelineTx(ctx, tx, customer.ID, "tag", "添加客户标签", tag.Name, tag.ID, firstNonEmpty(operatorID, opsOperatorID), firstNonEmpty(operatorName, opsOperatorName)); err != nil {
		return err
	}
	return tx.Commit()
}

func (api *API) removeOpsCustomerTagDB(ctx context.Context, customer OpsCustomer, tagID, operatorID, operatorName string) error {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var tagName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM tags WHERE id = $1`, tagID).Scan(&tagName); err != nil {
		if err == sql.ErrNoRows {
			tagName = tagID
		} else {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM customer_tags WHERE customer_id = $1 AND tag_id = $2`, customer.ID, tagID); err != nil {
		return err
	}
	if err := api.addOpsTimelineTx(ctx, tx, customer.ID, "tag", "移除客户标签", "标签已移除："+tagName, tagID, firstNonEmpty(operatorID, opsOperatorID), firstNonEmpty(operatorName, opsOperatorName)); err != nil {
		return err
	}
	return tx.Commit()
}

func (api *API) addOpsFollowupDB(ctx context.Context, customer OpsCustomer, followType, content, result, nextTime, stageAfter, createdBy string) error {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	before := customer.LifecycleStage
	recordID := newID("fu")
	createdBy = firstNonEmpty(createdBy, customer.OwnerGuideName)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO follow_up_records (
			id, customer_id, store_id, guide_id, follow_up_type, content, result,
			next_follow_up_time, stage_before, stage_after, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, recordID, customer.ID, customer.StoreID, customer.OwnerGuideID, firstNonEmpty(followType, "日常回访"), content, firstNonEmpty(result, "需继续跟进"), parseOptionalBusinessTime(nextTime), before, stageAfter, createdBy); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE customers SET last_follow_up_time = now(), updated_at = now() WHERE id = $1`, customer.ID); err != nil {
		return err
	}
	if err := api.addOpsTimelineTx(ctx, tx, customer.ID, "followup", "新增跟进记录", firstNonEmpty(followType, "日常回访")+"："+content+"；结果："+firstNonEmpty(result, "需继续跟进"), recordID, customer.OwnerGuideID, createdBy); err != nil {
		return err
	}
	if stageAfter != "" && stageAfter != before {
		if err := api.changeOpsCustomerStageTx(ctx, tx, customer, stageAfter, "跟进记录触发阶段变化", customer.OwnerGuideID, createdBy); err != nil {
			return err
		}
	}
	if strings.TrimSpace(nextTime) != "" {
		task := OpsTask{TaskType: "客户跟进提醒", Title: "跟进" + customer.Name, Description: "跟进记录设置了下次跟进时间。", CustomerID: customer.ID, CustomerName: customer.Name, RegionID: customer.RegionID, RegionName: customer.RegionName, StoreID: customer.StoreID, StoreName: customer.StoreName, AssignedToUserID: customer.OwnerGuideID, AssignedToName: customer.OwnerGuideName, AssignedToRole: "guide", Priority: "P2", DueTime: nextTime, Source: "下次跟进时间"}
		if _, err := api.createOpsTaskTx(ctx, tx, task); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (api *API) createOpsTaskDB(ctx context.Context, payload OpsTask) (OpsTask, error) {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return OpsTask{}, err
	}
	defer tx.Rollback()
	task, err := api.createOpsTaskTx(ctx, tx, payload)
	if err != nil {
		return OpsTask{}, err
	}
	if err := tx.Commit(); err != nil {
		return OpsTask{}, err
	}
	return task, nil
}

func (api *API) createOpsTaskTx(ctx context.Context, tx *sql.Tx, payload OpsTask) (OpsTask, error) {
	if payload.CustomerID != "" && (payload.CustomerName == "" || payload.StoreID == "" || payload.AssignedToUserID == "") {
		var customer OpsCustomer
		err := tx.QueryRowContext(ctx, `
			SELECT `+opsCustomerSelectSQL("c")+`
			FROM customers c WHERE c.id = $1
		`, payload.CustomerID).Scan(&customer.ID, &customer.Name, &customer.Nickname, &customer.Mobile, &customer.Avatar, &customer.SourceChannel, &customer.RegionID, &customer.RegionName, &customer.StoreID, &customer.StoreName, &customer.OwnerGuideID, &customer.OwnerGuideName, &customer.LifecycleStage, &customer.IntentionLevel, &customer.Status, &customer.AddWeComTime, &customer.LastFollowUpTime, &customer.LastInteractionTime, &customer.DealStatus, &customer.Risk, &customer.CreatedAt, &customer.UpdatedAt)
		if err != nil {
			return OpsTask{}, err
		}
		payload.CustomerName = customer.Name
		payload.RegionID = customer.RegionID
		payload.RegionName = customer.RegionName
		payload.StoreID = customer.StoreID
		payload.StoreName = customer.StoreName
		payload.AssignedToUserID = firstNonEmpty(payload.AssignedToUserID, customer.OwnerGuideID)
		payload.AssignedToName = firstNonEmpty(payload.AssignedToName, customer.OwnerGuideName)
	}
	payload.ID = firstNonEmpty(payload.ID, newID("ot"))
	payload.Priority = firstNonEmpty(payload.Priority, "P1")
	payload.Status = firstNonEmpty(payload.Status, "pending")
	payload.Source = firstNonEmpty(payload.Source, "总部派发")
	payload.AssignedToRole = firstNonEmpty(payload.AssignedToRole, "guide")
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sop_tasks (
			id, task_type, title, description, customer_id, customer_name, region_id, region_name,
			store_id, store_name, assigned_to_user_id, assigned_to_name, assigned_to_role,
			priority, status, due_time, source
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`, payload.ID, payload.TaskType, payload.Title, payload.Description, payload.CustomerID, payload.CustomerName, payload.RegionID, payload.RegionName, payload.StoreID, payload.StoreName, payload.AssignedToUserID, payload.AssignedToName, payload.AssignedToRole, payload.Priority, payload.Status, parseOptionalBusinessTime(payload.DueTime), payload.Source); err != nil {
		return OpsTask{}, err
	}
	if err := api.addOpsTaskLogTx(ctx, tx, payload.ID, "create", "", payload.Status, payload.Source); err != nil {
		return OpsTask{}, err
	}
	if payload.CustomerID != "" {
		if err := api.addOpsTimelineTx(ctx, tx, payload.CustomerID, "task", "生成SOP任务", payload.Title, payload.ID, opsOperatorID, opsOperatorName); err != nil {
			return OpsTask{}, err
		}
	}
	return payload, nil
}

func (api *API) setOpsTaskStatusDB(ctx context.Context, task OpsTask, status, action, remark string) error {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	old := task.Status
	completedExpr := "completed_at"
	if status == "completed" {
		completedExpr = "now()"
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sop_tasks
		SET status = $2,
			completed_at = `+completedExpr+`,
			updated_at = now()
		WHERE id = $1
	`, task.ID, status); err != nil {
		return err
	}
	if err := api.addOpsTaskLogTx(ctx, tx, task.ID, action, old, status, remark); err != nil {
		return err
	}
	if task.CustomerID != "" {
		if err := api.addOpsTimelineTx(ctx, tx, task.CustomerID, "task", action+"："+task.Title, firstNonEmpty(remark, task.Description), task.ID, task.AssignedToUserID, firstNonEmpty(task.AssignedToName, opsOperatorName)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE customers SET last_follow_up_time = now(), updated_at = now() WHERE id = $1`, task.CustomerID); err != nil {
			return err
		}
		if status == "completed" && task.TaskType == "新客户首次跟进" {
			var customer OpsCustomer
			err := tx.QueryRowContext(ctx, `SELECT `+opsCustomerSelectSQL("c")+` FROM customers c WHERE c.id = $1`, task.CustomerID).Scan(&customer.ID, &customer.Name, &customer.Nickname, &customer.Mobile, &customer.Avatar, &customer.SourceChannel, &customer.RegionID, &customer.RegionName, &customer.StoreID, &customer.StoreName, &customer.OwnerGuideID, &customer.OwnerGuideName, &customer.LifecycleStage, &customer.IntentionLevel, &customer.Status, &customer.AddWeComTime, &customer.LastFollowUpTime, &customer.LastInteractionTime, &customer.DealStatus, &customer.Risk, &customer.CreatedAt, &customer.UpdatedAt)
			if err != nil {
				return err
			}
			if customer.LifecycleStage == "first_follow_pending" {
				if err := api.changeOpsCustomerStageTx(ctx, tx, customer, "following", "首次跟进任务完成", task.AssignedToUserID, task.AssignedToName); err != nil {
					return err
				}
			}
		}
	}
	if status == "completed" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE operation_exceptions
			SET status = 'resolved', resolved_at = now()
			WHERE task_id = $1 AND status <> 'resolved'
		`, task.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (api *API) assignOpsTaskDB(ctx context.Context, task OpsTask, userID, userName string) error {
	tx, err := api.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE sop_tasks
		SET assigned_to_user_id = COALESCE(NULLIF($2, ''), assigned_to_user_id),
			assigned_to_name = COALESCE(NULLIF($3, ''), assigned_to_name),
			updated_at = now()
		WHERE id = $1
	`, task.ID, userID, userName); err != nil {
		return err
	}
	if err := api.addOpsTaskLogTx(ctx, tx, task.ID, "assign", task.Status, task.Status, "任务转派给："+firstNonEmpty(userName, task.AssignedToName)); err != nil {
		return err
	}
	return tx.Commit()
}

func (api *API) saveOpsMaterialDB(ctx context.Context, payload OpsMaterial) (OpsMaterial, error) {
	payload.ID = firstNonEmpty(payload.ID, newID("om"))
	payload.Status = firstNonEmpty(payload.Status, "enabled")
	payload.CreatedBy = firstNonEmpty(payload.CreatedBy, opsOperatorName)
	err := api.db.QueryRowContext(ctx, `
		INSERT INTO materials (id, title, content, material_type, applicable_stage, applicable_tags, status, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, title, content, material_type, applicable_stage, applicable_tags, status, created_by,
			to_char(created_at, 'YYYY-MM-DD HH24:MI'), to_char(updated_at, 'YYYY-MM-DD HH24:MI')
	`, payload.ID, payload.Title, payload.Content, payload.MaterialType, payload.ApplicableStage, pq.Array(payload.ApplicableTags), payload.Status, payload.CreatedBy).Scan(&payload.ID, &payload.Title, &payload.Content, &payload.MaterialType, &payload.ApplicableStage, pq.Array(&payload.ApplicableTags), &payload.Status, &payload.CreatedBy, &payload.CreatedAt, &payload.UpdatedAt)
	return payload, err
}

func (api *API) addOpsTaskLogTx(ctx context.Context, tx *sql.Tx, taskID, action, oldStatus, newStatus, remark string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO sop_task_logs (id, task_id, action, old_status, new_status, remark, operator_id, operator_name)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, newID("otl"), taskID, action, oldStatus, newStatus, remark, opsOperatorID, opsOperatorName)
	return err
}

func (api *API) addOpsTimelineTx(ctx context.Context, tx *sql.Tx, customerID, eventType, title, content, relatedID, operatorID, operatorName string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO customer_operation_timeline (id, customer_id, event_type, title, content, related_id, operator_id, operator_name)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, newID("tl"), customerID, eventType, title, content, relatedID, firstNonEmpty(operatorID, opsOperatorID), firstNonEmpty(operatorName, opsOperatorName))
	return err
}

func (api *API) refreshOpsExceptionsDB(ctx context.Context) error {
	rows, err := api.db.QueryContext(ctx, `
		SELECT `+opsTaskSelectSQL("t")+`
		FROM sop_tasks t
		WHERE t.status = 'overdue'
	`)
	if err != nil {
		return err
	}
	tasks, err := scanOpsTaskRows(rows)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := api.ensureOpsExceptionDB(ctx, "task_overdue", task.CustomerID, task.ID, "跟进任务逾期", task.Description, "P0", "立即分派导购处理，并使用对应阶段话术。"); err != nil {
			return err
		}
	}
	rows, err = api.db.QueryContext(ctx, `
		SELECT `+opsCustomerSelectSQL("c")+`
		FROM customers c
		WHERE c.lifecycle_stage IN ('first_follow_pending', 'dormant')
	`)
	if err != nil {
		return err
	}
	customers, err := scanOpsCustomers(rows)
	if err != nil {
		return err
	}
	for _, customer := range customers {
		if customer.LifecycleStage == "first_follow_pending" {
			if err := api.ensureOpsExceptionDB(ctx, "first_follow_timeout", customer.ID, "", "首次跟进超时", customer.Risk, "P0", "请店长立即督促导购首次跟进，或转派给可接待导购。"); err != nil {
				return err
			}
		}
		if customer.LifecycleStage == "dormant" {
			if err := api.ensureOpsExceptionDB(ctx, "customer_dormant", customer.ID, "", "沉睡客户", customer.Risk, "P1", "生成沉睡唤醒任务，并使用沉睡唤醒话术。"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (api *API) ensureOpsExceptionDB(ctx context.Context, kind, customerID, taskID, title, description, severity, suggestion string) error {
	var customer OpsCustomer
	err := api.db.QueryRowContext(ctx, `SELECT `+opsCustomerSelectSQL("c")+` FROM customers c WHERE c.id = $1`, customerID).Scan(&customer.ID, &customer.Name, &customer.Nickname, &customer.Mobile, &customer.Avatar, &customer.SourceChannel, &customer.RegionID, &customer.RegionName, &customer.StoreID, &customer.StoreName, &customer.OwnerGuideID, &customer.OwnerGuideName, &customer.LifecycleStage, &customer.IntentionLevel, &customer.Status, &customer.AddWeComTime, &customer.LastFollowUpTime, &customer.LastInteractionTime, &customer.DealStatus, &customer.Risk, &customer.CreatedAt, &customer.UpdatedAt)
	if err != nil {
		return err
	}
	_, err = api.db.ExecContext(ctx, `
		INSERT INTO operation_exceptions (
			id, exception_type, title, description, severity, region_id, region_name, store_id, store_name,
			customer_id, customer_name, task_id, assigned_to, status, suggestion
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'pending',$14)
		ON CONFLICT DO NOTHING
	`, newID("oe"), kind, title, firstNonEmpty(description, customer.Risk), severity, customer.RegionID, customer.RegionName, customer.StoreID, customer.StoreName, customer.ID, customer.Name, taskID, customer.OwnerGuideName, suggestion)
	return err
}
