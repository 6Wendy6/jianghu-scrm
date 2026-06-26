package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

func newAPI() *API {
	now := "2026-06-25 10:30"
	api := &API{}
	api.stores = []Store{
		{ID: "s1", Name: "深圳南山万象天地店", InternalCode: "BU-KST-HN-001", ExternalCode: "POS-3201", Brand: "蔻斯汀", Region: "华南", BrandRegion: "蔻斯汀 / 华南", Type: "直营", GuideCount: 8, PoolCount: 326, Config: StoreConfig{EntryMode: "跟随全局 · 企微优先", ServiceGuide: "开启", Group: "南山店会员福利群", Welcome: "门店默认欢迎语", Polling: "按顺序轮巡"}},
		{ID: "s2", Name: "广州天河代理店", InternalCode: "BU-BA-HN-014", ExternalCode: "POS-5108", Brand: "品牌 A", Region: "华南", BrandRegion: "品牌 A / 华南", Type: "代理", GuideCount: 5, PoolCount: 188, Config: StoreConfig{EntryMode: "关注优先", ServiceGuide: "开启", Group: "618 试用活动群", Welcome: "天河关注优先欢迎语", Polling: "按新导购优先（入职2个月内保护期）"}},
		{ID: "s3", Name: "杭州湖滨银泰店", InternalCode: "BU-BB-HD-006", ExternalCode: "POS-7702", Brand: "品牌 B", Region: "华东", BrandRegion: "品牌 B / 华东", Type: "直营", GuideCount: 6, PoolCount: 241, Config: StoreConfig{EntryMode: "跟随全局 · 企微优先", ServiceGuide: "开启", Group: "杭州会员福利群", Welcome: "门店默认欢迎语", Polling: "按顺序轮巡"}},
	}
	api.guides = []Guide{
		{ID: "g1", StoreID: "s1", Name: "江诗颖", Code: "南山店-江诗颖", Count: "388 / 12", TotalPool: 388, TodayPool: 12, Status: "在职", EmploymentStatus: "在职", Handling: "正常服务", Lifecycle: []AuditEvent{audit(now, "添加接待导购", "生成导购活码", "店长", "导购：江诗颖", "南山店-江诗颖 已进入物料码轮巡池。")}},
		{ID: "g2", StoreID: "s1", Name: "王敏", Code: "南山店-王敏", Count: "205 / 7", TotalPool: 205, TodayPool: 7, Status: "在职", EmploymentStatus: "在职", Handling: "正常服务", Lifecycle: []AuditEvent{audit(now, "添加接待导购", "生成导购活码", "店长", "导购：王敏", "南山店-王敏 已进入物料码轮巡池。")}},
		{ID: "g3", StoreID: "s1", Name: "林浩", Code: "南山店-林浩", Count: "96 / 0", TotalPool: 96, TodayPool: 0, Status: "已调店", EmploymentStatus: "已调店", Handling: "6 人进交接池，纯手动", Lifecycle: []AuditEvent{audit(now, "成员调店", "原门店码停用", "系统", "导购：林浩", "客户进入交接池，由店长判断是否交接。")}},
		{ID: "g4", StoreID: "s1", Name: "赵敏", Code: "南山店-赵敏", Count: "142 / 0", TotalPool: 142, TodayPool: 0, Status: "已离职", EmploymentStatus: "已离职", Handling: "9 人强制交接", Lifecycle: []AuditEvent{audit(now, "成员离职", "活码销毁不可恢复", "系统", "导购：赵敏", "存量客户必须指派接收人。")}},
	}
	api.customers = []Customer{
		customer("c1", "陈女士", "小陈", "138****8821", "江诗颖", "南山店-江诗颖", "蔻斯汀 / 南山店", "新客待转化", 86, "待转化", "新品试用活动", "-", "明天 10:00", []string{"线下门店", "高意向"}, []string{"企微好友", "南山门店"}, []Relation{{ID: "r1", GuideID: "g1", Guide: "江诗颖", Code: "南山店-江诗颖", Store: "蔻斯汀 / 南山店", LinkedAt: "2026-06-24", EndedAt: "-", Main: true}}, []string{"打开新品试用链接 3 次", "入群后点击群发链接"}),
		customer("c2", "王女士", "Wendy", "136****9120", "王敏", "南山店-王敏", "蔻斯汀 / 南山店", "复购培育", 70, "已成交", "门店导购码", "¥980", "2026-06-28 11:00", []string{"VIP", "已成交"}, []string{"企微好友", "老客"}, []Relation{{ID: "r2", GuideID: "g2", Guide: "王敏", Code: "南山店-王敏", Store: "蔻斯汀 / 南山店", LinkedAt: "2026-06-20", EndedAt: "-", Main: true}}, []string{"成交后 7 天回访"}),
		customer("c3", "李先生", "Lee", "未绑定手机号", "赵敏", "南山店-赵敏", "蔻斯汀 / 南山店", "待交接", 42, "待识别", "门店物料码", "-", "待指派", []string{"待补资料"}, []string{"企微好友"}, []Relation{{ID: "r3", GuideID: "g4", Guide: "赵敏", Code: "南山店-赵敏", Store: "蔻斯汀 / 南山店", LinkedAt: "2026-06-18", EndedAt: "-", Main: true}}, []string{"未绑定手机号"}),
	}
	api.handover = []HandoverItem{
		{ID: "h1", CustomerID: "c3", Customer: "李先生", Reason: "赵敏离职", CurrentOwner: "赵敏", RequiredAction: "强制指派", Receiver: "", Status: "待同步"},
		{ID: "h2", CustomerID: "c4", Customer: "周女士", Reason: "林浩调店", CurrentOwner: "林浩", RequiredAction: "手动判断", Receiver: "", Status: "待同步"},
	}
	api.touches = []TouchRule{
		{ID: "t1", Name: "门店默认欢迎语", Type: "欢迎语", Scope: "全部线下门店", Status: "启用"},
		{ID: "t2", Name: "会员日活动通知", Type: "群发任务", Scope: "华南直营门店", Status: "执行中"},
		{ID: "t3", Name: "新客 1/3/7 天培育", Type: "SOP", Scope: "新入池客户", Status: "启用"},
	}
	api.groups = []CustomerGroup{
		{ID: "cg1", Name: "南山店会员福利群", Store: "蔻斯汀 / 南山店", Owner: "江诗颖", Tags: []string{"门店群", "会员福利"}, Count: 286, TodayJoin: 18, TodayQuit: 2, CreatedAt: "2026-06-09 10:40", Status: "运营中", TodayEvent: "入群 18 / 退群 2"},
		{ID: "cg2", Name: "618 试用活动群", Store: "华南门店", Owner: "张婷", Tags: []string{"活动群", "高意向"}, Count: 198, TodayJoin: 9, TodayQuit: 1, CreatedAt: "2026-06-10 14:22", Status: "运营中", TodayEvent: "关键词提醒 3"},
	}
	api.groupWelcomes = []GroupWelcome{{ID: "gw1", Group: "南山店会员福利群", Content: "欢迎加入南山店会员福利群。", Status: "启用"}}
	api.groupSOPs = []GroupSOP{{ID: "gs1", Name: "新入群 3 天转化", Groups: []string{"南山店会员福利群"}, Stage: "新入群", Content: "群主第 1/3 天提醒新品试用权益。", Status: "启用"}}
	api.groupCalendar = []GroupCalendarEvent{{ID: "gc1", Group: "南山店会员福利群", Title: "会员日福利提醒", Date: "2026-06-28", Owner: "江诗颖", Status: "待发送"}}
	api.groupReminders = []GroupReminder{{ID: "gr1", Name: "关键词提醒", Trigger: "退货/投诉/价格", Owner: "店长", Status: "启用"}}
	api.groupTagGroups = []GroupTagGroup{{ID: "gt1", Name: "群类型", Tags: []string{"门店群", "活动群"}, Scope: "全部门店", Owner: "运营", Status: "正常"}}
	api.tags = []Tag{
		{ID: "tag1", Name: "线下门店", Group: "来源渠道", Status: "正常", Customers: 1284, Wecom: "企微-线下"},
		{ID: "tag2", Name: "南山店", Group: "来源渠道", Status: "正常", Customers: 826, Wecom: "企微-南山"},
		{ID: "tag3", Name: "高意向", Group: "客户状态", Status: "正常", Customers: 486, Wecom: "企微-高意向"},
		{ID: "tag4", Name: "已绑定会员", Group: "客户状态", Status: "正常", Customers: 902, Wecom: "企微-会员"},
		{ID: "tag5", Name: "已流失", Group: "客户状态", Status: "停用", Customers: 35, Wecom: "企微-流失"},
	}
	api.tagGroups = []TagGroup{
		{ID: "tg1", Name: "来源渠道", Stores: []string{"华南直营门店", "深圳南山万象天地店"}, Tags: []string{"线下门店", "南山店"}, Order: 1},
		{ID: "tg2", Name: "客户状态", Stores: []string{"全部门店"}, Tags: []string{"高意向", "已绑定会员", "已流失"}, Order: 2},
	}
	api.autoRules = []AutoTagRule{{ID: "ar1", Name: "门店码入池打来源", Trigger: "扫码来源=门店活码", Actions: []string{"打线下门店", "打门店标签"}, Scope: "全部门店", Impact: 3426, Status: "启用"}}
	api.preTagRules = []PreTagRule{{ID: "pr1", Entry: "南山店试用活动", Tags: []string{"南山店", "高意向"}, Trigger: "扫码/提交表单/进群", Period: "2026-06-24 至 2026-07-24", Status: "启用"}}
	return api
}

func (api *API) register(mux *http.ServeMux) {
	mux.HandleFunc("/api/health", api.withJSON(api.health))
	mux.HandleFunc("/api/metrics", api.metricsHandler)
	mux.HandleFunc("/api/bootstrap", api.withJSON(api.bootstrap))
	mux.HandleFunc("/api/summary", api.withJSON(api.summary))
	mux.HandleFunc("/api/metrics/refresh", api.withJSON(api.refreshMetricsHandler))
	mux.HandleFunc("/api/stores", api.withJSON(api.storesHandler))
	mux.HandleFunc("/api/stores/", api.withJSON(api.storeActionHandler))
	mux.HandleFunc("/api/guides", api.withJSON(api.guidesHandler))
	mux.HandleFunc("/api/guides/", api.withJSON(api.guideActionHandler))
	mux.HandleFunc("/api/customers", api.withJSON(api.customersHandler))
	mux.HandleFunc("/api/customers/", api.withJSON(api.customerActionHandler))
	mux.HandleFunc("/api/touches", api.withJSON(api.touchesHandler))
	mux.HandleFunc("/api/groups", api.withJSON(api.groupsHandler))
	mux.HandleFunc("/api/groups/", api.withJSON(api.groupActionHandler))
	mux.HandleFunc("/api/tags", api.withJSON(api.tagsHandler))
	mux.HandleFunc("/api/tags/", api.withJSON(api.tagActionHandler))
	mux.HandleFunc("/api/tag-groups", api.withJSON(api.tagGroupsHandler))
	mux.HandleFunc("/api/tag-rules/auto", api.withJSON(api.autoRulesHandler))
	mux.HandleFunc("/api/tag-rules/pre", api.withJSON(api.preTagRulesHandler))
	mux.HandleFunc("/api/events/inbox", api.withJSON(api.eventInboxHandler))
	mux.HandleFunc("/api/tasks", api.withJSON(api.tasksHandler))
	mux.HandleFunc("/api/tasks/", api.withJSON(api.taskActionHandler))
}

func (api *API) withJSON(next func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, X-SCRM-API-Token, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err := next(w, r); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, errNotFound) {
				status = http.StatusNotFound
			}
			if errors.Is(err, errBadRequest) {
				status = http.StatusBadRequest
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		}
	}
}
