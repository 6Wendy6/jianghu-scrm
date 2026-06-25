package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type API struct {
	mu             sync.RWMutex
	stores         []Store
	guides         []Guide
	customers      []Customer
	handover       []HandoverItem
	touches        []TouchRule
	groups         []CustomerGroup
	groupMassTasks []GroupMassTask
	groupWelcomes  []GroupWelcome
	groupSOPs      []GroupSOP
	groupCalendar  []GroupCalendarEvent
	groupReminders []GroupReminder
	groupTagGroups []GroupTagGroup
	tags           []Tag
	tagGroups      []TagGroup
	autoRules      []AutoTagRule
	preTagRules    []PreTagRule
	wecomSynced    bool
}

type Store struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	InternalCode string      `json:"internalCode"`
	ExternalCode string      `json:"externalCode"`
	Brand        string      `json:"brand"`
	Region       string      `json:"region"`
	BrandRegion  string      `json:"brandRegion"`
	Type         string      `json:"type"`
	GuideCount   int         `json:"guideCount"`
	PoolCount    int         `json:"poolCount"`
	Config       StoreConfig `json:"config"`
}

type StoreConfig struct {
	EntryMode    string `json:"entryMode"`
	ServiceGuide string `json:"serviceGuide"`
	Group        string `json:"group"`
	Welcome      string `json:"welcome"`
	Polling      string `json:"polling"`
}

type Guide struct {
	ID               string       `json:"id"`
	StoreID          string       `json:"storeId"`
	Name             string       `json:"name"`
	Code             string       `json:"code"`
	Count            string       `json:"count"`
	TotalPool        int          `json:"totalPool"`
	TodayPool        int          `json:"todayPool"`
	Status           string       `json:"status"`
	EmploymentStatus string       `json:"employmentStatus"`
	Paused           bool         `json:"paused"`
	Handling         string       `json:"handling"`
	Lifecycle        []AuditEvent `json:"lifecycle"`
}

type Relation struct {
	ID       string `json:"id"`
	GuideID  string `json:"guideId"`
	Guide    string `json:"guide"`
	Code     string `json:"code"`
	Store    string `json:"store"`
	LinkedAt string `json:"linkedAt"`
	EndedAt  string `json:"endedAt"`
	Main     bool   `json:"main"`
}

type Customer struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	Wecom            string       `json:"wecom"`
	Mobile           string       `json:"mobile"`
	Owner            string       `json:"owner"`
	Source           string       `json:"source"`
	SourceStore      string       `json:"sourceStore"`
	Stage            string       `json:"stage"`
	TagGroup         string       `json:"tagGroup"`
	Tags             []string     `json:"tags"`
	WecomTags        []string     `json:"wecomTags"`
	LastActive       string       `json:"lastActive"`
	IntentScore      int          `json:"intentScore"`
	SalesStage       string       `json:"salesStage"`
	ConversionSource string       `json:"conversionSource"`
	DealAmount       string       `json:"dealAmount"`
	NextFollowUp     string       `json:"nextFollowUp"`
	Signals          []string     `json:"signals"`
	Relations        []Relation   `json:"relations"`
	Timeline         []AuditEvent `json:"timeline"`
}

type HandoverItem struct {
	ID             string `json:"id"`
	CustomerID     string `json:"customerId"`
	Customer       string `json:"customer"`
	Reason         string `json:"reason"`
	CurrentOwner   string `json:"currentOwner"`
	RequiredAction string `json:"requiredAction"`
	Receiver       string `json:"receiver"`
	Status         string `json:"status"`
}

type TouchRule struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Scope  string `json:"scope"`
	Status string `json:"status"`
}

type CustomerGroup struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Store      string   `json:"store"`
	Owner      string   `json:"owner"`
	Tags       []string `json:"tags"`
	Count      int      `json:"count"`
	TodayJoin  int      `json:"todayJoin"`
	TodayQuit  int      `json:"todayQuit"`
	CreatedAt  string   `json:"createdAt"`
	Status     string   `json:"status"`
	TodayEvent string   `json:"todayEvent"`
}

type GroupMassTask struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Groups  []string `json:"groups"`
	Content string   `json:"content"`
	Status  string   `json:"status"`
}

type GroupWelcome struct {
	ID      string `json:"id"`
	Group   string `json:"group"`
	Content string `json:"content"`
	Status  string `json:"status"`
}

type GroupSOP struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Groups  []string `json:"groups"`
	Stage   string   `json:"stage"`
	Content string   `json:"content"`
	Status  string   `json:"status"`
}

type GroupCalendarEvent struct {
	ID     string `json:"id"`
	Group  string `json:"group"`
	Title  string `json:"title"`
	Date   string `json:"date"`
	Owner  string `json:"owner"`
	Status string `json:"status"`
}

type GroupReminder struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Trigger string `json:"trigger"`
	Owner   string `json:"owner"`
	Status  string `json:"status"`
}

type GroupTagGroup struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Tags   []string `json:"tags"`
	Scope  string   `json:"scope"`
	Owner  string   `json:"owner"`
	Status string   `json:"status"`
}

type Tag struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Group     string `json:"group"`
	Status    string `json:"status"`
	Customers int    `json:"customers"`
	Wecom     string `json:"wecom"`
}

type TagGroup struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Stores []string `json:"stores"`
	Tags   []string `json:"tags"`
	Order  int      `json:"order"`
}

type AutoTagRule struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Trigger string   `json:"trigger"`
	Actions []string `json:"actions"`
	Scope   string   `json:"scope"`
	Impact  int      `json:"impact"`
	Status  string   `json:"status"`
}

type PreTagRule struct {
	ID      string   `json:"id"`
	Entry   string   `json:"entry"`
	Tags    []string `json:"tags"`
	Trigger string   `json:"trigger"`
	Period  string   `json:"period"`
	Status  string   `json:"status"`
}

type AuditEvent struct {
	ID       string `json:"id"`
	Time     string `json:"time"`
	Action   string `json:"action"`
	Result   string `json:"result"`
	Operator string `json:"operator"`
	People   string `json:"people"`
	Detail   string `json:"detail"`
}

type QRDownload struct {
	FileName string `json:"fileName"`
	MimeType string `json:"mimeType"`
	Content  string `json:"content"`
}

func main() {
	api := newAPI()
	mux := http.NewServeMux()
	api.register(mux)
	addr := os.Getenv("SCRM_API_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	log.Printf("AI SCRM API listening on http://%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

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
	mux.HandleFunc("/api/bootstrap", api.withJSON(api.bootstrap))
	mux.HandleFunc("/api/summary", api.withJSON(api.summary))
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
}

func (api *API) withJSON(next func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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

var (
	errNotFound   = errors.New("not found")
	errBadRequest = errors.New("bad request")
)

func (api *API) health(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, map[string]string{"status": "ok", "time": time.Now().Format(time.RFC3339)})
}

func (api *API) bootstrap(w http.ResponseWriter, r *http.Request) error {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return writeJSON(w, map[string]any{
		"summary":        api.summaryData(),
		"stores":         api.stores,
		"guides":         api.guides,
		"customers":      api.customers,
		"handover":       api.handover,
		"touches":        api.touches,
		"groups":         api.groups,
		"groupMassTasks": api.groupMassTasks,
		"groupWelcomes":  api.groupWelcomes,
		"groupSOPs":      api.groupSOPs,
		"groupCalendar":  api.groupCalendar,
		"groupReminders": api.groupReminders,
		"groupTagGroups": api.groupTagGroups,
		"tags":           api.tags,
		"tagGroups":      api.tagGroups,
		"autoRules":      api.autoRules,
		"preTagRules":    api.preTagRules,
	})
}

func (api *API) summary(w http.ResponseWriter, r *http.Request) error {
	api.mu.RLock()
	defer api.mu.RUnlock()
	return writeJSON(w, api.summaryData())
}

func (api *API) summaryData() map[string]any {
	return map[string]any{
		"todayPool":       128,
		"attributionRate": "96.8%",
		"pending":         len(api.handover) + 160,
		"touchRate":       "82%",
		"trend":           []int{36, 42, 48, 38, 54, 66, 58},
		"suggestions":     []string{"南山店物料码入池环比 -20%", "高意向客户 48 人 24 小时未跟进", "天河代理店关注优先完成率更高"},
	}
}

func (api *API) storesHandler(w http.ResponseWriter, r *http.Request) error {
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
	return writeJSON(w, rows)
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
	return writeJSON(w, rows)
}

func (api *API) guidesHandler(w http.ResponseWriter, r *http.Request) error {
	api.mu.RLock()
	defer api.mu.RUnlock()
	status := r.URL.Query().Get("status")
	keyword := r.URL.Query().Get("keyword")
	rows := filter(api.guides, func(g Guide) bool {
		statusOK := status == "" || status == "全部状态" || g.EmploymentStatus == status || (status == "暂停使用" && g.Paused)
		return statusOK && match(keyword, g.Name+g.Code)
	})
	return writeJSON(w, rows)
}

func (api *API) guideActionHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/guides/")
	if len(parts) < 2 {
		return errNotFound
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

func (api *API) customersHandler(w http.ResponseWriter, r *http.Request) error {
	api.mu.Lock()
	defer api.mu.Unlock()
	if r.Method == http.MethodPost {
		var payload Customer
		if err := decode(r, &payload); err != nil {
			return err
		}
		if payload.Name == "" {
			return fmt.Errorf("%w: customer name is required", errBadRequest)
		}
		payload.ID = newID("c")
		payload.Timeline = []AuditEvent{audit(stamp(), "后台新增客户", "已入客户池", "运营", "客户："+payload.Name, "用于补录或测试，来源归属需要显式填写。")}
		api.customers = append([]Customer{payload}, api.customers...)
		return writeJSON(w, payload)
	}
	keyword := r.URL.Query().Get("keyword")
	guide := r.URL.Query().Get("guide")
	stage := r.URL.Query().Get("stage")
	rows := filter(api.customers, func(c Customer) bool {
		return match(keyword, c.Name+c.Mobile+c.Wecom+c.Source) && matchOption(guide, "全部导购", c.Owner) && matchOption(stage, "全部阶段", c.Stage)
	})
	return writeJSON(w, rows)
}

func (api *API) customerActionHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/customers/")
	if len(parts) == 0 {
		return errNotFound
	}
	if len(parts) == 1 && parts[0] == "batch-tags" {
		return api.batchTags(w, r)
	}
	customerID := parts[0]
	api.mu.Lock()
	defer api.mu.Unlock()
	customer, ok := api.findCustomer(customerID)
	if !ok {
		return errNotFound
	}
	if len(parts) == 1 {
		return writeJSON(w, customer)
	}
	switch parts[1] {
	case "touch":
		var payload struct {
			Content string `json:"content"`
			Method  string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.Method == "" {
			payload.Method = "企微单聊待办"
		}
		customer.Timeline = prependAudit(customer.Timeline, audit(stamp(), "创建触达任务", "待导购确认发送", "运营", "客户："+customer.Name+"；导购："+customer.Owner, payload.Method+"｜"+payload.Content))
		return writeJSON(w, customer)
	case "followups":
		var payload struct {
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		customer.Timeline = prependAudit(customer.Timeline, audit(stamp(), "新增跟进", "已记录客户沟通结果", customer.Owner, "客户："+customer.Name, payload.Detail))
		return writeJSON(w, customer)
	case "relations":
		if len(parts) == 2 {
			var payload struct {
				GuideID string `json:"guideId"`
				Guide   string `json:"guide"`
			}
			if err := decode(r, &payload); err != nil {
				return err
			}
			if payload.Guide == "" {
				return fmt.Errorf("%w: guide is required", errBadRequest)
			}
			relation := Relation{ID: newID("r"), GuideID: payload.GuideID, Guide: payload.Guide, Code: "南山店-" + payload.Guide, Store: customer.SourceStore, LinkedAt: today(), EndedAt: "-", Main: false}
			customer.Relations = append(customer.Relations, relation)
			customer.Timeline = prependAudit(customer.Timeline, audit(stamp(), "新增关联导购", "已建立服务关系", "运营", "客户："+customer.Name+"；导购："+payload.Guide, "历史来源不回改。"))
			return writeJSON(w, customer)
		}
		return api.customerRelationAction(w, r, customer, parts[2:])
	case "lead":
		customer.SalesStage = "已建线索"
		customer.Stage = "新客待转化"
		customer.Timeline = prependAudit(customer.Timeline, audit(stamp(), "建线索 / 商机", "已进入销售承接", "运营", "客户："+customer.Name+"；负责人："+customer.Owner, "来源行为："+first(customer.Signals)))
		return writeJSON(w, customer)
	case "order":
		var payload struct {
			Amount string `json:"amount"`
			Source string `json:"source"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.Amount == "" {
			payload.Amount = "¥1,280"
		}
		customer.DealAmount = payload.Amount
		customer.SalesStage = "已成交"
		customer.Stage = "复购培育"
		customer.Tags = appendUnique(customer.Tags, "已成交")
		customer.Timeline = prependAudit(customer.Timeline, audit(stamp(), "记录成交 / 回款", "已回写标签和转化阶段", "运营", "客户："+customer.Name+"；导购："+customer.Owner, "成交来源："+payload.Source+"；金额："+payload.Amount))
		return writeJSON(w, customer)
	default:
		return errNotFound
	}
}

func (api *API) customerRelationAction(w http.ResponseWriter, r *http.Request, customer *Customer, parts []string) error {
	if len(parts) < 2 {
		return errNotFound
	}
	relationID, action := parts[0], parts[1]
	index := -1
	for i := range customer.Relations {
		if customer.Relations[i].ID == relationID {
			index = i
			break
		}
	}
	if index < 0 {
		return errNotFound
	}
	relation := &customer.Relations[index]
	switch action {
	case "set-main":
		if relation.Main {
			return fmt.Errorf("%w: relation is already main guide", errBadRequest)
		}
		for i := range customer.Relations {
			customer.Relations[i].Main = false
		}
		relation.Main = true
		customer.Owner = relation.Guide
		customer.Timeline = prependAudit(customer.Timeline, audit(stamp(), "设置主跟进导购", "主跟进已更新", "运营", "客户："+customer.Name+"；导购："+relation.Guide, "之前的主跟进自动变为否。"))
	case "end":
		if relation.Main {
			return fmt.Errorf("%w: main guide relation cannot be ended", errBadRequest)
		}
		relation.EndedAt = today()
		customer.Timeline = prependAudit(customer.Timeline, audit(stamp(), "结束关联导购", "关系已结束", "运营", "客户："+customer.Name+"；导购："+relation.Guide, "系统留痕，历史来源不回改。"))
	default:
		return errNotFound
	}
	return writeJSON(w, customer)
}

func (api *API) batchTags(w http.ResponseWriter, r *http.Request) error {
	api.mu.Lock()
	defer api.mu.Unlock()
	var payload struct {
		CustomerIDs []string `json:"customerIds"`
		Tags        []string `json:"tags"`
	}
	if err := decode(r, &payload); err != nil {
		return err
	}
	if len(payload.CustomerIDs) == 0 {
		return fmt.Errorf("%w: customerIds is required", errBadRequest)
	}
	updated := 0
	for i := range api.customers {
		if contains(payload.CustomerIDs, api.customers[i].ID) {
			api.customers[i].Tags = appendUnique(api.customers[i].Tags, payload.Tags...)
			api.customers[i].Timeline = prependAudit(api.customers[i].Timeline, audit(stamp(), "批量打标签", "系统标签已更新", "运营", "客户："+api.customers[i].Name, strings.Join(payload.Tags, "、")))
			updated++
		}
	}
	return writeJSON(w, map[string]any{"updated": updated})
}

func (api *API) syncHandover(w http.ResponseWriter, r *http.Request, storeID string) error {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.wecomSynced = true
	for i := range api.handover {
		if api.handover[i].Status == "待同步" {
			api.handover[i].Status = "待处理"
		}
	}
	return writeJSON(w, api.handover)
}

func (api *API) submitHandover(w http.ResponseWriter, r *http.Request, storeID string) error {
	api.mu.Lock()
	defer api.mu.Unlock()
	if !api.wecomSynced {
		return fmt.Errorf("%w: sync wecom leave data first", errBadRequest)
	}
	var payload struct {
		Items []HandoverItem `json:"items"`
	}
	if err := decode(r, &payload); err != nil {
		return err
	}
	for _, item := range payload.Items {
		if item.Receiver == "" && strings.Contains(item.RequiredAction, "强制") {
			return fmt.Errorf("%w: receiver is required for forced handover", errBadRequest)
		}
		for i := range api.customers {
			if api.customers[i].ID == item.CustomerID && item.Receiver != "" {
				api.customers[i].Owner = item.Receiver
				api.customers[i].Stage = "新客待转化"
				api.customers[i].Timeline = prependAudit(api.customers[i].Timeline, audit(stamp(), "提交导购交接", "主跟进导购已更新", "店长", "客户："+api.customers[i].Name+"；接收人："+item.Receiver, item.Reason))
			}
		}
	}
	api.handover = []HandoverItem{}
	return writeJSON(w, map[string]any{"status": "completed", "handover": api.handover})
}

func (api *API) touchesHandler(w http.ResponseWriter, r *http.Request) error {
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
	return writeJSON(w, api.touches)
}

func (api *API) groupsHandler(w http.ResponseWriter, r *http.Request) error {
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
	return writeJSON(w, api.groups)
}

func (api *API) groupActionHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/groups/")
	if len(parts) == 0 {
		return errNotFound
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
	return writeJSON(w, api.tags)
}

func (api *API) tagActionHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/tags/")
	if len(parts) < 2 {
		return errNotFound
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
			return writeJSON(w, rows)
		default:
			return errNotFound
		}
	}
	return errNotFound
}

func (api *API) tagGroupsHandler(w http.ResponseWriter, r *http.Request) error {
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
	return writeJSON(w, api.tagGroups)
}

func (api *API) autoRulesHandler(w http.ResponseWriter, r *http.Request) error {
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
	return writeJSON(w, api.autoRules)
}

func (api *API) preTagRulesHandler(w http.ResponseWriter, r *http.Request) error {
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
	return writeJSON(w, api.preTagRules)
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

func customer(id, name, wecom, mobile, owner, source, sourceStore, stage string, intent int, salesStage, conversionSource, dealAmount, nextFollowUp string, tags, wecomTags []string, relations []Relation, signals []string) Customer {
	return Customer{
		ID: id, Name: name, Wecom: wecom, Mobile: mobile, Owner: owner, Source: source, SourceStore: sourceStore, Stage: stage,
		TagGroup: "行为标签 / 生命周期标签", Tags: tags, WecomTags: wecomTags, LastActive: "今天 15:10", IntentScore: intent,
		SalesStage: salesStage, ConversionSource: conversionSource, DealAmount: dealAmount, NextFollowUp: nextFollowUp, Relations: relations, Signals: signals,
		Timeline: []AuditEvent{audit("2026-06-25 15:10", "扫码入池", "已锁定来源并进入客户池", "系统", "客户："+name+"；导购："+owner, "来源活码："+source)},
	}
}

func writeJSON(w http.ResponseWriter, v any) error {
	return json.NewEncoder(w).Encode(v)
}

func decode(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("%w: empty body", errBadRequest)
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("%w: %v", errBadRequest, err)
	}
	return nil
}

func pathParts(path, prefix string) []string {
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	for i := range parts {
		parts[i], _ = url.PathUnescape(parts[i])
	}
	return parts
}

func createOrList[T any](w http.ResponseWriter, r *http.Request, items *[]T, prepare func(*T)) error {
	if r.Method == http.MethodPost {
		var item T
		if err := decode(r, &item); err != nil {
			return err
		}
		prepare(&item)
		*items = append([]T{item}, (*items)...)
		return writeJSON(w, item)
	}
	return writeJSON(w, *items)
}

func filter[T any](rows []T, keep func(T) bool) []T {
	result := make([]T, 0, len(rows))
	for _, row := range rows {
		if keep(row) {
			result = append(result, row)
		}
	}
	return result
}

func match(keyword, text string) bool {
	return keyword == "" || strings.Contains(strings.ToLower(text), strings.ToLower(keyword))
}

func matchOption(value, all, text string) bool {
	return value == "" || value == all || strings.Contains(text, value)
}

func contains[T comparable](rows []T, value T) bool {
	for _, row := range rows {
		if row == value {
			return true
		}
	}
	return false
}

func appendUnique(rows []string, values ...string) []string {
	seen := map[string]bool{}
	for _, row := range rows {
		seen[row] = true
	}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		rows = append(rows, value)
		seen[value] = true
	}
	return rows
}

func prependAudit(rows []AuditEvent, event AuditEvent) []AuditEvent {
	return append([]AuditEvent{event}, rows...)
}

func audit(timeText, action, result, operator, people, detail string) AuditEvent {
	return AuditEvent{ID: newID("ev"), Time: timeText, Action: action, Result: result, Operator: operator, People: people, Detail: detail}
}

func makeQRDownload(name, kind string) QRDownload {
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="360" height="420" viewBox="0 0 360 420"><rect width="360" height="420" fill="#fff"/><text x="180" y="34" text-anchor="middle" font-size="20" font-family="Arial" fill="#1f2937">%s</text><rect x="70" y="60" width="220" height="220" fill="#f8fafc" stroke="#1f2937" stroke-width="6"/><g fill="#111827"><rect x="90" y="80" width="48" height="48"/><rect x="222" y="80" width="48" height="48"/><rect x="90" y="212" width="48" height="48"/><rect x="154" y="94" width="16" height="16"/><rect x="184" y="94" width="22" height="22"/><rect x="150" y="138" width="26" height="26"/><rect x="194" y="142" width="18" height="18"/><rect x="226" y="154" width="22" height="22"/></g><text x="180" y="330" text-anchor="middle" font-size="16" font-family="Arial" fill="#475467">线下门店私域 %s</text></svg>`, name, kind)
	return QRDownload{FileName: name + "-" + kind + ".svg", MimeType: "image/svg+xml", Content: svg}
}

func stamp() string {
	return time.Now().Format("2006-01-02 15:04")
}

func today() string {
	return time.Now().Format("2006-01-02")
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func newID(prefix string) string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return prefix + hex.EncodeToString(b[:])
}
