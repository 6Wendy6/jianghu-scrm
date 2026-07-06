package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

const (
	opsRoleHQ       = "hq_admin"
	opsRoleRegion   = "region_manager"
	opsRoleStore    = "store_manager"
	opsRoleGuide    = "guide"
	opsOperatorID   = "u_hq_admin"
	opsOperatorName = "总部管理员 张三"
)

func (api *API) seedCustomerOps() {
	now := "2026-07-06 13:30"
	api.opsTags = []OpsCustomerTag{
		{ID: "tag_xhs", Name: "小红书来源", Category: "来源标签", Color: "blue", Source: "auto", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "tag_store", Name: "门店扫码", Category: "来源标签", Color: "green", Source: "auto", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "tag_douyin", Name: "抖音来源", Category: "来源标签", Color: "purple", Source: "auto", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "tag_intent", Name: "高意向", Category: "行为标签", Color: "orange", Source: "manual", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "tag_women", Name: "女装意向", Category: "兴趣标签", Color: "orange", Source: "manual", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "tag_kids", Name: "童装意向", Category: "兴趣标签", Color: "teal", Source: "manual", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "tag_dormant", Name: "沉睡客户", Category: "风险标签", Color: "red", Source: "auto", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "tag_deal", Name: "已成交", Category: "消费标签", Color: "green", Source: "auto", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
	}
	api.opsCustomers = []OpsCustomer{
		{ID: "oc1", Name: "陈女士", Nickname: "小陈", Mobile: "138****8821", Avatar: "陈", SourceChannel: "小红书", RegionID: "r_south", RegionName: "华南", StoreID: "s1", StoreName: "深圳南山万象天地店", OwnerGuideID: "g1", OwnerGuideName: "江诗颖", LifecycleStage: "high_intent", IntentionLevel: "高意向", Status: "跟进中", AddWeComTime: "2026-07-06 14:28", LastFollowUpTime: "2026-07-06 15:10", LastInteractionTime: "2026-07-06 15:28", DealStatus: "未成交", Risk: "24小时内需二次跟进", CreatedAt: now, UpdatedAt: now},
		{ID: "oc2", Name: "王女士", Nickname: "Wendy", Mobile: "136****9120", Avatar: "王", SourceChannel: "门店扫码", RegionID: "r_south", RegionName: "华南", StoreID: "s2", StoreName: "广州天河代理店", OwnerGuideID: "g5", OwnerGuideName: "张婷", LifecycleStage: "purchased", IntentionLevel: "已成交", Status: "已成交", AddWeComTime: "2026-07-04 10:20", LastFollowUpTime: "2026-07-05 20:10", LastInteractionTime: "2026-07-05 20:30", DealStatus: "已成交 ¥980", Risk: "待售后回访", CreatedAt: now, UpdatedAt: now},
		{ID: "oc3", Name: "李先生", Nickname: "Lee", Mobile: "未绑定手机号", Avatar: "李", SourceChannel: "门店物料码", RegionID: "r_south", RegionName: "华南", StoreID: "s1", StoreName: "深圳南山万象天地店", OwnerGuideID: "g2", OwnerGuideName: "王敏", LifecycleStage: "first_follow_pending", IntentionLevel: "待判断", Status: "待首次跟进", AddWeComTime: "2026-07-06 13:02", LastInteractionTime: "2026-07-06 13:02", DealStatus: "未成交", Risk: "超过30分钟未首次跟进", CreatedAt: now, UpdatedAt: now},
		{ID: "oc4", Name: "赵女士", Nickname: "赵赵", Mobile: "", Avatar: "赵", SourceChannel: "抖音", RegionID: "r_east", RegionName: "华东", StoreID: "s3", StoreName: "杭州湖滨银泰店", OwnerGuideID: "g6", OwnerGuideName: "刘雨", LifecycleStage: "dormant", IntentionLevel: "中意向", Status: "沉睡", AddWeComTime: "2026-05-20 11:00", LastFollowUpTime: "2026-05-28 11:00", LastInteractionTime: "2026-06-01 09:10", DealStatus: "未成交", Risk: "30天未互动", CreatedAt: now, UpdatedAt: now},
	}
	api.opsTagLinks = []OpsCustomerTagRelation{
		{ID: "link1", CustomerID: "oc1", TagID: "tag_xhs", Source: "auto", OperatorID: "system", CreatedAt: now},
		{ID: "link2", CustomerID: "oc1", TagID: "tag_intent", Source: "manual", OperatorID: "g1", CreatedAt: now},
		{ID: "link3", CustomerID: "oc1", TagID: "tag_women", Source: "manual", OperatorID: "g1", CreatedAt: now},
		{ID: "link4", CustomerID: "oc2", TagID: "tag_store", Source: "auto", OperatorID: "system", CreatedAt: now},
		{ID: "link5", CustomerID: "oc2", TagID: "tag_deal", Source: "auto", OperatorID: "system", CreatedAt: now},
		{ID: "link6", CustomerID: "oc3", TagID: "tag_store", Source: "auto", OperatorID: "system", CreatedAt: now},
		{ID: "link7", CustomerID: "oc4", TagID: "tag_douyin", Source: "auto", OperatorID: "system", CreatedAt: now},
		{ID: "link8", CustomerID: "oc4", TagID: "tag_kids", Source: "manual", OperatorID: "g6", CreatedAt: now},
		{ID: "link9", CustomerID: "oc4", TagID: "tag_dormant", Source: "auto", OperatorID: "system", CreatedAt: now},
	}
	api.opsTimelines = []OpsTimelineEvent{
		{ID: "tl1", CustomerID: "oc1", EventType: "source", Title: "扫码进入", Content: "小红书活动页扫码进入私域", OperatorID: "system", OperatorName: "系统", CreatedAt: "2026-07-06 14:28"},
		{ID: "tl2", CustomerID: "oc1", EventType: "assignment", Title: "客户分配", Content: "分配给导购江诗颖", OperatorID: "system", OperatorName: "系统", CreatedAt: "2026-07-06 14:31"},
		{ID: "tl3", CustomerID: "oc1", EventType: "followup", Title: "首次跟进", Content: "客户关注夏季连衣裙", OperatorID: "g1", OperatorName: "江诗颖", CreatedAt: "2026-07-06 15:10"},
		{ID: "tl4", CustomerID: "oc2", EventType: "deal", Title: "完成成交", Content: "成交金额 ¥980", OperatorID: "g5", OperatorName: "张婷", CreatedAt: "2026-07-05 20:10"},
		{ID: "tl5", CustomerID: "oc3", EventType: "task", Title: "首次跟进任务逾期", Content: "客户已分配但超过30分钟未首次跟进", OperatorID: "system", OperatorName: "系统", CreatedAt: "2026-07-06 13:35"},
		{ID: "tl6", CustomerID: "oc4", EventType: "risk", Title: "沉睡客户识别", Content: "30天未互动，生成沉睡唤醒任务", OperatorID: "system", OperatorName: "系统", CreatedAt: "2026-07-06 09:00"},
	}
	api.opsFollowups = []OpsFollowUpRecord{
		{ID: "fu1", CustomerID: "oc1", StoreID: "s1", GuideID: "g1", FollowUpType: "首次沟通", Content: "客户关注夏季连衣裙，已发送试穿邀请。", Result: "需继续跟进", NextFollowUpTime: "2026-07-06 18:00", StageBefore: "following", StageAfter: "high_intent", CreatedBy: "江诗颖", CreatedAt: "2026-07-06 15:10"},
	}
	api.opsTasks = []OpsTask{
		{ID: "ot1", TaskType: "新客户首次跟进", Title: "首次跟进李先生", Description: "客户已分配但超过30分钟未首次跟进。", CustomerID: "oc3", CustomerName: "李先生", RegionID: "r_south", RegionName: "华南", StoreID: "s1", StoreName: "深圳南山万象天地店", AssignedToUserID: "g2", AssignedToName: "王敏", AssignedToRole: "guide", Priority: "P0", Status: "overdue", DueTime: "2026-07-06 13:35", Source: "客户分配", CreatedAt: now, UpdatedAt: now},
		{ID: "ot2", TaskType: "高意向客户回访", Title: "回访陈女士连衣裙需求", Description: "客户打开活动链接并被标记为高意向，需要二次触达。", CustomerID: "oc1", CustomerName: "陈女士", RegionID: "r_south", RegionName: "华南", StoreID: "s1", StoreName: "深圳南山万象天地店", AssignedToUserID: "g1", AssignedToName: "江诗颖", AssignedToRole: "guide", Priority: "P1", Status: "pending", DueTime: "2026-07-06 18:00", Source: "SOP", CreatedAt: now, UpdatedAt: now},
		{ID: "ot3", TaskType: "成交客户售后回访", Title: "王女士成交后售后回访", Description: "成交后第3天确认体验并推荐搭配。", CustomerID: "oc2", CustomerName: "王女士", RegionID: "r_south", RegionName: "华南", StoreID: "s2", StoreName: "广州天河代理店", AssignedToUserID: "g5", AssignedToName: "张婷", AssignedToRole: "guide", Priority: "P2", Status: "pending", DueTime: "2026-07-08 11:00", Source: "成交回写", CreatedAt: now, UpdatedAt: now},
		{ID: "ot4", TaskType: "沉睡客户唤醒", Title: "赵女士30天未互动唤醒", Description: "客户超过30天未互动，建议使用童装上新话术。", CustomerID: "oc4", CustomerName: "赵女士", RegionID: "r_east", RegionName: "华东", StoreID: "s3", StoreName: "杭州湖滨银泰店", AssignedToUserID: "g6", AssignedToName: "刘雨", AssignedToRole: "guide", Priority: "P1", Status: "pending", DueTime: "2026-07-06 20:00", Source: "自动规则", CreatedAt: now, UpdatedAt: now},
	}
	api.opsMaterials = []OpsMaterial{
		{ID: "om1", Title: "新客欢迎 + 需求确认", Content: "您好，我是门店顾问。看到您刚添加企业微信，我可以先了解一下您最近想看的品类和预算。", MaterialType: "首次跟进话术", ApplicableStage: "first_follow_pending", ApplicableTags: []string{"首次跟进"}, Status: "enabled", CreatedBy: "总部运营", CreatedAt: now, UpdatedAt: now},
		{ID: "om2", Title: "高意向连衣裙逼单", Content: "这款连衣裙今天门店还有试穿名额，老客价可以保留到今晚。我帮您约一个到店时间？", MaterialType: "高意向跟进话术", ApplicableStage: "high_intent", ApplicableTags: []string{"高意向", "女装意向"}, Status: "enabled", CreatedBy: "总部运营", CreatedAt: now, UpdatedAt: now},
		{ID: "om3", Title: "成交后售后回访", Content: "您上次购买的商品体验怎么样？如果尺码或搭配有问题，我可以帮您一起调整。", MaterialType: "售后回访话术", ApplicableStage: "purchased", ApplicableTags: []string{"已成交"}, Status: "enabled", CreatedBy: "总部运营", CreatedAt: now, UpdatedAt: now},
		{ID: "om4", Title: "沉睡客户唤醒", Content: "我们最近上新了几款适合您之前关注风格的款式，老客户有专属试穿福利。", MaterialType: "沉睡唤醒话术", ApplicableStage: "dormant", ApplicableTags: []string{"沉睡客户"}, Status: "enabled", CreatedBy: "总部运营", CreatedAt: now, UpdatedAt: now},
	}
	api.refreshOpsExceptionsLocked()
}

func (api *API) customerOpsBootstrapHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.customerOpsBootstrapDBHandler(w, r)
	}
	if r.Method != http.MethodGet {
		return errNotFound
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	api.refreshOpsExceptionsLocked()
	return writeJSON(w, OpsBootstrap{
		Customers:  api.scopedOpsCustomersLocked(r),
		Tasks:      api.scopedOpsTasksLocked(r),
		Materials:  append([]OpsMaterial{}, api.opsMaterials...),
		Exceptions: api.scopedOpsExceptionsLocked(r),
		Tags:       append([]OpsCustomerTag{}, api.opsTags...),
	})
}

func (api *API) opsCustomersHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.opsCustomersDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method != http.MethodGet {
		return errNotFound
	}
	rows := api.filterOpsCustomersLocked(r, api.scopedOpsCustomersLocked(r))
	return writePaginatedOrList(w, r, rows)
}

func (api *API) opsCustomerActionHandler(w http.ResponseWriter, r *http.Request, customerID string, parts []string) error {
	if api.db != nil {
		return api.opsCustomerActionDBHandler(w, r, customerID, parts)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	customer := api.findOpsCustomerLocked(customerID)
	if customer == nil || !api.canAccessOpsCustomer(r, *customer) {
		return errNotFound
	}
	if len(parts) == 0 {
		if r.Method != http.MethodGet {
			return errNotFound
		}
		return writeJSON(w, api.opsCustomerDetailLocked(*customer))
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
		if payload.Stage == "" {
			return fmt.Errorf("%w: stage is required", errBadRequest)
		}
		api.changeOpsCustomerStageLocked(customer, payload.Stage, firstNonEmpty(payload.Reason, "手动调整生命周期"), payload.OperatorID, payload.OperatorName)
		return writeJSON(w, api.opsCustomerDetailLocked(*customer))
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
			tag := api.ensureOpsTagLocked(payload.TagID, payload.Name, payload.Category, payload.Source)
			api.addOpsTagToCustomerLocked(customer.ID, tag.ID, firstNonEmpty(payload.Source, "manual"), payload.OperatorID, payload.OperatorName)
			return writeJSON(w, api.opsCustomerDetailLocked(*customer))
		}
		if len(parts) == 2 && r.Method == http.MethodDelete {
			api.removeOpsTagFromCustomerLocked(customer.ID, parts[1])
			api.addOpsTimelineLocked(customer.ID, "tag", "移除客户标签", "标签已移除："+parts[1], "", opsOperatorID, opsOperatorName)
			return writeJSON(w, api.opsCustomerDetailLocked(*customer))
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
		record := api.addOpsFollowupLocked(customer, payload.FollowUpType, payload.Content, payload.Result, payload.NextFollowUpTime, payload.StageAfter, payload.CreatedBy)
		return writeJSON(w, record)
	case "timeline":
		if r.Method != http.MethodGet {
			return errNotFound
		}
		return writeJSON(w, api.opsTimelineForCustomerLocked(customer.ID))
	default:
		return errNotFound
	}
	return errNotFound
}

func (api *API) opsTasksHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.opsTasksDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	switch r.Method {
	case http.MethodGet:
		api.refreshOpsExceptionsLocked()
		rows := api.filterOpsTasksLocked(r, api.scopedOpsTasksLocked(r))
		return writePaginatedOrList(w, r, rows)
	case http.MethodPost:
		var payload OpsTask
		if err := decode(r, &payload); err != nil {
			return err
		}
		task := api.createOpsTaskLocked(payload)
		return writeJSON(w, task)
	default:
		return errNotFound
	}
}

func (api *API) opsTaskActionHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.opsTaskActionDBHandler(w, r)
	}
	parts := pathParts(r.URL.Path, "/api/sop-tasks/")
	if len(parts) == 0 {
		return errNotFound
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	task := api.findOpsTaskLocked(parts[0])
	if task == nil || !api.canAccessOpsTask(r, *task) {
		return errNotFound
	}
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			return errNotFound
		}
		return writeJSON(w, *task)
	}
	var payload struct {
		Remark         string `json:"remark"`
		AssignedToUser string `json:"assignedToUserId"`
		AssignedToName string `json:"assignedToName"`
	}
	_ = decode(r, &payload)
	switch parts[1] {
	case "process":
		api.setOpsTaskStatusLocked(task, "processing", "开始处理任务", payload.Remark)
	case "complete":
		api.setOpsTaskStatusLocked(task, "completed", "完成任务", payload.Remark)
	case "assign":
		task.AssignedToUserID = firstNonEmpty(payload.AssignedToUser, task.AssignedToUserID)
		task.AssignedToName = firstNonEmpty(payload.AssignedToName, task.AssignedToName)
		task.UpdatedAt = stamp()
		api.addOpsTaskLogLocked(task.ID, "assign", task.Status, task.Status, "任务转派给："+task.AssignedToName)
	case "ignore":
		api.setOpsTaskStatusLocked(task, "ignored", "忽略任务", payload.Remark)
	default:
		return errNotFound
	}
	return writeJSON(w, *task)
}

func (api *API) opsMaterialsHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.opsMaterialsDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	switch r.Method {
	case http.MethodGet:
		return writePaginatedOrList(w, r, api.opsMaterials)
	case http.MethodPost:
		var payload OpsMaterial
		if err := decode(r, &payload); err != nil {
			return err
		}
		payload.ID = newID("om")
		payload.Status = firstNonEmpty(payload.Status, "enabled")
		payload.CreatedBy = firstNonEmpty(payload.CreatedBy, opsOperatorName)
		payload.CreatedAt = stamp()
		payload.UpdatedAt = payload.CreatedAt
		api.opsMaterials = append([]OpsMaterial{payload}, api.opsMaterials...)
		return writeJSON(w, payload)
	default:
		return errNotFound
	}
}

func (api *API) opsMaterialActionHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.opsMaterialActionDBHandler(w, r)
	}
	parts := pathParts(r.URL.Path, "/api/materials/")
	if len(parts) == 0 {
		return errNotFound
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	index := -1
	for i := range api.opsMaterials {
		if api.opsMaterials[i].ID == parts[0] {
			index = i
			break
		}
	}
	if index < 0 {
		return errNotFound
	}
	switch r.Method {
	case http.MethodPatch:
		var payload OpsMaterial
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.Title != "" {
			api.opsMaterials[index].Title = payload.Title
		}
		if payload.Content != "" {
			api.opsMaterials[index].Content = payload.Content
		}
		if payload.MaterialType != "" {
			api.opsMaterials[index].MaterialType = payload.MaterialType
		}
		if payload.ApplicableStage != "" {
			api.opsMaterials[index].ApplicableStage = payload.ApplicableStage
		}
		if payload.ApplicableTags != nil {
			api.opsMaterials[index].ApplicableTags = payload.ApplicableTags
		}
		if payload.Status != "" {
			api.opsMaterials[index].Status = payload.Status
		}
		api.opsMaterials[index].UpdatedAt = stamp()
		return writeJSON(w, api.opsMaterials[index])
	case http.MethodDelete:
		removed := api.opsMaterials[index]
		api.opsMaterials = append(api.opsMaterials[:index], api.opsMaterials[index+1:]...)
		return writeJSON(w, removed)
	default:
		return errNotFound
	}
}

func (api *API) opsExceptionsHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.opsExceptionsDBHandler(w, r)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method != http.MethodGet {
		return errNotFound
	}
	api.refreshOpsExceptionsLocked()
	rows := api.filterOpsExceptionsLocked(r, api.scopedOpsExceptionsLocked(r))
	return writePaginatedOrList(w, r, rows)
}

func (api *API) opsExceptionActionHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db != nil {
		return api.opsExceptionActionDBHandler(w, r)
	}
	parts := pathParts(r.URL.Path, "/api/operation-exceptions/")
	if len(parts) < 2 {
		return errNotFound
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	item := api.findOpsExceptionLocked(parts[0])
	if item == nil {
		return errNotFound
	}
	switch parts[1] {
	case "process":
		item.Status = "processing"
	case "resolve":
		item.Status = "resolved"
		item.ResolvedAt = stamp()
	case "ignore":
		item.Status = "ignored"
	default:
		return errNotFound
	}
	return writeJSON(w, *item)
}

func (api *API) createOpsTaskLocked(payload OpsTask) OpsTask {
	customer := api.findOpsCustomerLocked(payload.CustomerID)
	if customer != nil {
		payload.CustomerName = customer.Name
		payload.RegionID = customer.RegionID
		payload.RegionName = customer.RegionName
		payload.StoreID = customer.StoreID
		payload.StoreName = customer.StoreName
		payload.AssignedToUserID = firstNonEmpty(payload.AssignedToUserID, customer.OwnerGuideID)
		payload.AssignedToName = firstNonEmpty(payload.AssignedToName, customer.OwnerGuideName)
	}
	payload.ID = newID("ot")
	payload.Priority = firstNonEmpty(payload.Priority, "P1")
	payload.Status = firstNonEmpty(payload.Status, "pending")
	payload.Source = firstNonEmpty(payload.Source, "总部派发")
	payload.AssignedToRole = firstNonEmpty(payload.AssignedToRole, "guide")
	payload.CreatedAt = stamp()
	payload.UpdatedAt = payload.CreatedAt
	api.opsTasks = append([]OpsTask{payload}, api.opsTasks...)
	if customer != nil {
		api.addOpsTimelineLocked(customer.ID, "task", "生成SOP任务", payload.Title, payload.ID, opsOperatorID, opsOperatorName)
	}
	api.addOpsTaskLogLocked(payload.ID, "create", "", payload.Status, payload.Source)
	return payload
}

func (api *API) setOpsTaskStatusLocked(task *OpsTask, status, action, remark string) {
	old := task.Status
	task.Status = status
	task.UpdatedAt = stamp()
	if status == "completed" {
		task.CompletedAt = task.UpdatedAt
	}
	api.addOpsTaskLogLocked(task.ID, action, old, status, remark)
	if customer := api.findOpsCustomerLocked(task.CustomerID); customer != nil {
		content := firstNonEmpty(remark, task.Description)
		api.addOpsTimelineLocked(customer.ID, "task", action+"："+task.Title, content, task.ID, task.AssignedToUserID, firstNonEmpty(task.AssignedToName, opsOperatorName))
		if status == "completed" && task.TaskType == "新客户首次跟进" && customer.LifecycleStage == "first_follow_pending" {
			api.changeOpsCustomerStageLocked(customer, "following", "首次跟进任务完成", task.AssignedToUserID, task.AssignedToName)
		}
		customer.LastFollowUpTime = stamp()
		customer.UpdatedAt = stamp()
	}
	if status == "completed" {
		api.resolveOpsExceptionByTaskLocked(task.ID)
	}
}

func (api *API) addOpsFollowupLocked(customer *OpsCustomer, followType, content, result, nextTime, stageAfter, createdBy string) OpsFollowUpRecord {
	before := customer.LifecycleStage
	record := OpsFollowUpRecord{
		ID:               newID("fu"),
		CustomerID:       customer.ID,
		StoreID:          customer.StoreID,
		GuideID:          customer.OwnerGuideID,
		FollowUpType:     firstNonEmpty(followType, "日常回访"),
		Content:          content,
		Result:           firstNonEmpty(result, "需继续跟进"),
		NextFollowUpTime: nextTime,
		StageBefore:      before,
		StageAfter:       stageAfter,
		CreatedBy:        firstNonEmpty(createdBy, customer.OwnerGuideName),
		CreatedAt:        stamp(),
	}
	api.opsFollowups = append([]OpsFollowUpRecord{record}, api.opsFollowups...)
	customer.LastFollowUpTime = record.CreatedAt
	customer.UpdatedAt = record.CreatedAt
	api.addOpsTimelineLocked(customer.ID, "followup", "新增跟进记录", record.FollowUpType+"："+content+"；结果："+record.Result, record.ID, customer.OwnerGuideID, record.CreatedBy)
	if stageAfter != "" && stageAfter != before {
		api.changeOpsCustomerStageLocked(customer, stageAfter, "跟进记录触发阶段变化", customer.OwnerGuideID, record.CreatedBy)
	}
	if nextTime != "" {
		api.createOpsTaskLocked(OpsTask{TaskType: "客户跟进提醒", Title: "跟进" + customer.Name, Description: "跟进记录设置了下次跟进时间。", CustomerID: customer.ID, Priority: "P2", DueTime: nextTime, Source: "下次跟进时间"})
	}
	return record
}

func (api *API) changeOpsCustomerStageLocked(customer *OpsCustomer, stage, reason, operatorID, operatorName string) {
	before := customer.LifecycleStage
	customer.LifecycleStage = stage
	customer.Status = opsStageLabel(stage)
	customer.UpdatedAt = stamp()
	log := OpsLifecycleLog{ID: newID("cl"), CustomerID: customer.ID, StageBefore: before, StageAfter: stage, Reason: reason, OperatorID: firstNonEmpty(operatorID, opsOperatorID), OperatorName: firstNonEmpty(operatorName, opsOperatorName), CreatedAt: stamp()}
	api.opsLifecycleLogs = append([]OpsLifecycleLog{log}, api.opsLifecycleLogs...)
	api.addOpsTimelineLocked(customer.ID, "lifecycle", "生命周期变更", opsStageLabel(before)+" -> "+opsStageLabel(stage)+"；原因："+reason, log.ID, log.OperatorID, log.OperatorName)
	if stage == "high_intent" && !api.hasOpenOpsTaskLocked(customer.ID, "高意向客户回访") {
		api.createOpsTaskLocked(OpsTask{TaskType: "高意向客户回访", Title: "24小时内回访" + customer.Name, Description: "客户被标记为高意向，自动生成回访任务。", CustomerID: customer.ID, Priority: "P1", DueTime: "2026-07-07 18:00", Source: "阶段流转"})
	}
}

func (api *API) ensureOpsTagLocked(tagID, name, category, source string) OpsCustomerTag {
	if tagID != "" {
		for _, tag := range api.opsTags {
			if tag.ID == tagID {
				return tag
			}
		}
	}
	name = firstNonEmpty(name, tagID)
	for _, tag := range api.opsTags {
		if tag.Name == name {
			return tag
		}
	}
	tag := OpsCustomerTag{ID: newID("tag"), Name: name, Category: firstNonEmpty(category, "手动标签"), Color: "blue", Source: firstNonEmpty(source, "manual"), IsEnabled: true, CreatedAt: stamp(), UpdatedAt: stamp()}
	api.opsTags = append(api.opsTags, tag)
	return tag
}

func (api *API) addOpsTagToCustomerLocked(customerID, tagID, source, operatorID, operatorName string) {
	for _, link := range api.opsTagLinks {
		if link.CustomerID == customerID && link.TagID == tagID {
			return
		}
	}
	api.opsTagLinks = append(api.opsTagLinks, OpsCustomerTagRelation{ID: newID("link"), CustomerID: customerID, TagID: tagID, Source: source, OperatorID: firstNonEmpty(operatorID, opsOperatorID), CreatedAt: stamp()})
	tagName := api.opsTagNameLocked(tagID)
	api.addOpsTimelineLocked(customerID, "tag", "添加客户标签", tagName, tagID, firstNonEmpty(operatorID, opsOperatorID), firstNonEmpty(operatorName, opsOperatorName))
}

func (api *API) removeOpsTagFromCustomerLocked(customerID, tagID string) {
	next := api.opsTagLinks[:0]
	for _, link := range api.opsTagLinks {
		if link.CustomerID == customerID && link.TagID == tagID {
			continue
		}
		next = append(next, link)
	}
	api.opsTagLinks = next
}

func (api *API) refreshOpsExceptionsLocked() {
	for i := range api.opsTasks {
		task := &api.opsTasks[i]
		if task.Status == "overdue" {
			api.ensureOpsExceptionLocked("task_overdue", task.CustomerID, task.ID, "跟进任务逾期", task.Title, "任务已超过截止时间："+task.DueTime, "P0", "立即分派导购处理，并使用对应阶段话术。")
		}
	}
	for _, customer := range api.opsCustomers {
		switch customer.LifecycleStage {
		case "first_follow_pending":
			api.ensureOpsExceptionLocked("first_follow_timeout", customer.ID, "", "首次跟进超时", customer.Name, customer.Risk, "P0", "请店长立即督促导购首次跟进，或转派给可接待导购。")
		case "dormant":
			api.ensureOpsExceptionLocked("customer_dormant", customer.ID, "", "沉睡客户", customer.Name, customer.Risk, "P1", "生成沉睡唤醒任务，并使用沉睡唤醒话术。")
		}
	}
}

func (api *API) ensureOpsExceptionLocked(kind, customerID, taskID, title, scope, description, severity, suggestion string) {
	for _, item := range api.opsExceptions {
		if item.ExceptionType == kind && item.CustomerID == customerID && item.TaskID == taskID && item.Status != "resolved" && item.Status != "ignored" {
			return
		}
	}
	customer := api.findOpsCustomerLocked(customerID)
	item := OpsException{ID: newID("oe"), ExceptionType: kind, Title: title, Description: description, Severity: severity, CustomerID: customerID, TaskID: taskID, Status: "pending", Suggestion: suggestion, CreatedAt: stamp()}
	if customer != nil {
		item.RegionID = customer.RegionID
		item.RegionName = customer.RegionName
		item.StoreID = customer.StoreID
		item.StoreName = customer.StoreName
		item.CustomerName = customer.Name
		item.AssignedTo = customer.OwnerGuideName
	}
	if scope != "" && item.CustomerName == "" {
		item.CustomerName = scope
	}
	api.opsExceptions = append([]OpsException{item}, api.opsExceptions...)
}

func (api *API) resolveOpsExceptionByTaskLocked(taskID string) {
	for i := range api.opsExceptions {
		if api.opsExceptions[i].TaskID == taskID && api.opsExceptions[i].Status != "resolved" {
			api.opsExceptions[i].Status = "resolved"
			api.opsExceptions[i].ResolvedAt = stamp()
		}
	}
}

func (api *API) addOpsTaskLogLocked(taskID, action, oldStatus, newStatus, remark string) {
	api.opsTaskLogs = append([]OpsTaskLog{{ID: newID("otl"), TaskID: taskID, Action: action, OldStatus: oldStatus, NewStatus: newStatus, Remark: remark, OperatorID: opsOperatorID, OperatorName: opsOperatorName, CreatedAt: stamp()}}, api.opsTaskLogs...)
}

func (api *API) addOpsTimelineLocked(customerID, eventType, title, content, relatedID, operatorID, operatorName string) {
	api.opsTimelines = append([]OpsTimelineEvent{{ID: newID("tl"), CustomerID: customerID, EventType: eventType, Title: title, Content: content, RelatedID: relatedID, OperatorID: firstNonEmpty(operatorID, opsOperatorID), OperatorName: firstNonEmpty(operatorName, opsOperatorName), CreatedAt: stamp()}}, api.opsTimelines...)
}

func (api *API) opsCustomerDetailLocked(customer OpsCustomer) map[string]any {
	return map[string]any{
		"customer":  api.withOpsCustomerTagsLocked(customer),
		"tags":      api.opsTagsForCustomerLocked(customer.ID),
		"followups": api.opsFollowupsForCustomerLocked(customer.ID),
		"timeline":  api.opsTimelineForCustomerLocked(customer.ID),
		"tasks":     api.opsTasksForCustomerLocked(customer.ID),
		"materials": api.opsMaterialsForCustomerLocked(customer),
	}
}

func (api *API) scopedOpsCustomersLocked(r *http.Request) []OpsCustomer {
	rows := []OpsCustomer{}
	for _, customer := range api.opsCustomers {
		if api.canAccessOpsCustomer(r, customer) {
			rows = append(rows, api.withOpsCustomerTagsLocked(customer))
		}
	}
	return rows
}

func (api *API) scopedOpsTasksLocked(r *http.Request) []OpsTask {
	rows := []OpsTask{}
	for _, task := range api.opsTasks {
		if api.canAccessOpsTask(r, task) {
			rows = append(rows, task)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].DueTime < rows[j].DueTime })
	return rows
}

func (api *API) scopedOpsExceptionsLocked(r *http.Request) []OpsException {
	rows := []OpsException{}
	for _, item := range api.opsExceptions {
		if api.canAccessOpsException(r, item) {
			rows = append(rows, item)
		}
	}
	return rows
}

func (api *API) filterOpsCustomersLocked(r *http.Request, rows []OpsCustomer) []OpsCustomer {
	q := r.URL.Query()
	keyword := strings.TrimSpace(q.Get("keyword"))
	stage := strings.TrimSpace(q.Get("stage"))
	tag := strings.TrimSpace(q.Get("tag"))
	return filter(rows, func(customer OpsCustomer) bool {
		text := customer.Name + customer.Mobile + customer.StoreName + customer.OwnerGuideName + customer.SourceChannel + strings.Join(customer.Tags, ",")
		return match(keyword, text) && matchOption(stage, "all", customer.LifecycleStage) && (tag == "" || tag == "all" || contains(customer.Tags, tag))
	})
}

func (api *API) filterOpsTasksLocked(r *http.Request, rows []OpsTask) []OpsTask {
	q := r.URL.Query()
	return filter(rows, func(task OpsTask) bool {
		return matchOption(q.Get("status"), "all", task.Status) && matchOption(q.Get("priority"), "all", task.Priority) && match(q.Get("keyword"), task.Title+task.CustomerName+task.StoreName+task.AssignedToName)
	})
}

func (api *API) filterOpsExceptionsLocked(r *http.Request, rows []OpsException) []OpsException {
	status := r.URL.Query().Get("status")
	return filter(rows, func(item OpsException) bool {
		return matchOption(status, "all", item.Status)
	})
}

func (api *API) canAccessOpsCustomer(r *http.Request, customer OpsCustomer) bool {
	role := opsRole(r)
	switch role {
	case opsRoleRegion:
		return emptyOrEqual(opsHeader(r, "X-SCRM-Region-ID"), customer.RegionID)
	case opsRoleStore:
		return emptyOrEqual(opsHeader(r, "X-SCRM-Store-ID"), customer.StoreID)
	case opsRoleGuide:
		return emptyOrEqual(opsHeader(r, "X-SCRM-Guide-ID"), customer.OwnerGuideID)
	default:
		return true
	}
}

func (api *API) canAccessOpsTask(r *http.Request, task OpsTask) bool {
	role := opsRole(r)
	switch role {
	case opsRoleRegion:
		return emptyOrEqual(opsHeader(r, "X-SCRM-Region-ID"), task.RegionID)
	case opsRoleStore:
		return emptyOrEqual(opsHeader(r, "X-SCRM-Store-ID"), task.StoreID)
	case opsRoleGuide:
		return emptyOrEqual(opsHeader(r, "X-SCRM-Guide-ID"), task.AssignedToUserID)
	default:
		return true
	}
}

func (api *API) canAccessOpsException(r *http.Request, item OpsException) bool {
	role := opsRole(r)
	switch role {
	case opsRoleRegion:
		return emptyOrEqual(opsHeader(r, "X-SCRM-Region-ID"), item.RegionID)
	case opsRoleStore:
		return emptyOrEqual(opsHeader(r, "X-SCRM-Store-ID"), item.StoreID)
	case opsRoleGuide:
		customer := api.findOpsCustomerLocked(item.CustomerID)
		return customer != nil && emptyOrEqual(opsHeader(r, "X-SCRM-Guide-ID"), customer.OwnerGuideID)
	default:
		return true
	}
}

func (api *API) withOpsCustomerTagsLocked(customer OpsCustomer) OpsCustomer {
	customer.Tags = api.opsTagNamesForCustomerLocked(customer.ID)
	return customer
}

func (api *API) opsTagNamesForCustomerLocked(customerID string) []string {
	names := []string{}
	for _, link := range api.opsTagLinks {
		if link.CustomerID == customerID {
			names = appendUnique(names, api.opsTagNameLocked(link.TagID))
		}
	}
	return names
}

func (api *API) opsTagsForCustomerLocked(customerID string) []OpsCustomerTag {
	rows := []OpsCustomerTag{}
	for _, link := range api.opsTagLinks {
		if link.CustomerID == customerID {
			if tag := api.findOpsTagLocked(link.TagID); tag != nil {
				rows = append(rows, *tag)
			}
		}
	}
	return rows
}

func (api *API) opsFollowupsForCustomerLocked(customerID string) []OpsFollowUpRecord {
	return filter(api.opsFollowups, func(item OpsFollowUpRecord) bool { return item.CustomerID == customerID })
}

func (api *API) opsTasksForCustomerLocked(customerID string) []OpsTask {
	return filter(api.opsTasks, func(item OpsTask) bool { return item.CustomerID == customerID })
}

func (api *API) opsTimelineForCustomerLocked(customerID string) []OpsTimelineEvent {
	rows := filter(api.opsTimelines, func(item OpsTimelineEvent) bool { return item.CustomerID == customerID })
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].CreatedAt > rows[j].CreatedAt })
	return rows
}

func (api *API) opsMaterialsForCustomerLocked(customer OpsCustomer) []OpsMaterial {
	rows := []OpsMaterial{}
	tags := api.opsTagNamesForCustomerLocked(customer.ID)
	for _, material := range api.opsMaterials {
		if material.ApplicableStage == customer.LifecycleStage || intersects(material.ApplicableTags, tags) {
			rows = append(rows, material)
		}
	}
	return rows
}

func (api *API) findOpsCustomerLocked(id string) *OpsCustomer {
	for i := range api.opsCustomers {
		if api.opsCustomers[i].ID == id {
			return &api.opsCustomers[i]
		}
	}
	return nil
}

func (api *API) findOpsTaskLocked(id string) *OpsTask {
	for i := range api.opsTasks {
		if api.opsTasks[i].ID == id {
			return &api.opsTasks[i]
		}
	}
	return nil
}

func (api *API) findOpsExceptionLocked(id string) *OpsException {
	for i := range api.opsExceptions {
		if api.opsExceptions[i].ID == id {
			return &api.opsExceptions[i]
		}
	}
	return nil
}

func (api *API) findOpsTagLocked(id string) *OpsCustomerTag {
	for i := range api.opsTags {
		if api.opsTags[i].ID == id {
			return &api.opsTags[i]
		}
	}
	return nil
}

func (api *API) opsTagNameLocked(id string) string {
	if tag := api.findOpsTagLocked(id); tag != nil {
		return tag.Name
	}
	return id
}

func (api *API) hasOpenOpsTaskLocked(customerID, taskType string) bool {
	for _, task := range api.opsTasks {
		if task.CustomerID == customerID && task.TaskType == taskType && task.Status != "completed" && task.Status != "ignored" && task.Status != "cancelled" {
			return true
		}
	}
	return false
}

func opsRole(r *http.Request) string {
	return firstNonEmpty(opsHeader(r, "X-SCRM-Role"), opsRoleHQ)
}

func opsHeader(r *http.Request, key string) string {
	return strings.TrimSpace(r.Header.Get(key))
}

func emptyOrEqual(scope, value string) bool {
	return scope == "" || scope == value
}

func intersects(a, b []string) bool {
	for _, left := range a {
		for _, right := range b {
			if left == right {
				return true
			}
		}
	}
	return false
}

func opsStageLabel(stage string) string {
	labels := map[string]string{
		"new_lead":             "新线索",
		"added_wecom":          "已加企微",
		"pending_assignment":   "待分配",
		"assigned":             "已分配",
		"first_follow_pending": "待首次跟进",
		"following":            "跟进中",
		"high_intent":          "高意向",
		"purchased":            "已成交",
		"repurchase":           "复购客户",
		"dormant":              "沉睡客户",
		"lost":                 "流失客户",
		"blocked":              "黑名单",
	}
	return firstNonEmpty(labels[stage], stage)
}
