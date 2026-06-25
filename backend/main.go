package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type Store struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	InternalCode string `json:"internalCode"`
	ExternalCode string `json:"externalCode"`
	BrandRegion  string `json:"brandRegion"`
	Type         string `json:"type"`
	GuideCount   int    `json:"guideCount"`
	PoolCount    int    `json:"poolCount"`
}

type Guide struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Count    string `json:"count"`
	Status   string `json:"status"`
	Handling string `json:"handling"`
}

type Relation struct {
	Guide    string `json:"guide"`
	Code     string `json:"code"`
	Store    string `json:"store"`
	LinkedAt string `json:"linkedAt"`
	EndedAt  string `json:"endedAt"`
	Main     bool   `json:"main"`
}

type Customer struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Wecom         string     `json:"wecom"`
	Mobile        string     `json:"mobile"`
	Owner         string     `json:"owner"`
	Source        string     `json:"source"`
	Stage         string     `json:"stage"`
	TagGroup      string     `json:"tagGroup"`
	Tags          []string   `json:"tags"`
	WecomTags     []string   `json:"wecomTags"`
	LastActive    string     `json:"lastActive"`
	Relations     []Relation `json:"relations"`
}

type Group struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Owner     string   `json:"owner"`
	Tags      []string `json:"tags"`
	Count     int      `json:"count"`
	TodayJoin int      `json:"todayJoin"`
	TodayQuit int      `json:"todayQuit"`
	CreatedAt string   `json:"createdAt"`
}

type Touch struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Scope  string `json:"scope"`
	Status string `json:"status"`
}

type Tag struct {
	Name      string `json:"name"`
	Group     string `json:"group"`
	Status    string `json:"status"`
	Customers int    `json:"customers"`
	Wecom     string `json:"wecom"`
}

type TagGroup struct {
	Name   string   `json:"name"`
	Stores []string `json:"stores"`
	Tags   []string `json:"tags"`
}

var tags = []Tag{
	{Name: "线下门店", Group: "来源渠道", Status: "正常", Customers: 1284, Wecom: "企微-线下"},
	{Name: "南山店", Group: "来源渠道", Status: "正常", Customers: 826, Wecom: "企微-南山"},
	{Name: "高意向", Group: "客户状态", Status: "正常", Customers: 486, Wecom: "企微-高意向"},
	{Name: "已绑定会员", Group: "客户状态", Status: "正常", Customers: 902, Wecom: "企微-会员"},
	{Name: "已流失", Group: "客户状态", Status: "停用", Customers: 35, Wecom: "企微-流失"},
}

var tagGroups = []TagGroup{
	{Name: "来源渠道", Stores: []string{"华南直营门店", "深圳南山万象天地店"}, Tags: []string{"线下门店", "南山店"}},
	{Name: "客户状态", Stores: []string{"全部门店"}, Tags: []string{"高意向", "已绑定会员", "已流失"}},
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/summary", withJSON(summaryHandler))
	mux.HandleFunc("/api/stores", withJSON(storesHandler))
	mux.HandleFunc("/api/guides", withJSON(guidesHandler))
	mux.HandleFunc("/api/customers", withJSON(customersHandler))
	mux.HandleFunc("/api/groups", withJSON(groupsHandler))
	mux.HandleFunc("/api/touches", withJSON(touchesHandler))
	mux.HandleFunc("/api/tags", withJSON(tagsHandler))
	mux.HandleFunc("/api/tags/", withJSON(tagActionHandler))
	mux.HandleFunc("/api/tag-groups", withJSON(tagGroupsHandler))
	log.Println("AI SCRM API listening on http://127.0.0.1:8080")
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", mux))
}

func withJSON(next func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func summaryHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"todayPool":       128,
		"attributionRate": "96.8%",
		"pending":         162,
		"touchRate":       "82%",
		"trend":           []int{36, 42, 48, 38, 54, 66, 58},
		"suggestions":     []string{"南山店物料码入池环比 -20%", "高意向客户 48 人 24 小时未跟进", "天河代理店关注优先完成率更高"},
	})
}

func storesHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, []Store{
		{ID: "s1", Name: "深圳南山万象天地店", InternalCode: "BU-KST-HN-001", ExternalCode: "POS-3201", BrandRegion: "蔻斯汀 / 华南", Type: "直营", GuideCount: 8, PoolCount: 326},
		{ID: "s2", Name: "广州天河代理店", InternalCode: "BU-BA-HN-014", ExternalCode: "POS-5108", BrandRegion: "品牌 A / 华南", Type: "代理", GuideCount: 5, PoolCount: 188},
		{ID: "s3", Name: "杭州湖滨银泰店", InternalCode: "BU-BB-HD-006", ExternalCode: "POS-7702", BrandRegion: "品牌 B / 华东", Type: "直营", GuideCount: 6, PoolCount: 241},
	})
}

func guidesHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, []Guide{
		{ID: "g1", Name: "江诗颖", Code: "南山店-江诗颖", Count: "388 / 12", Status: "在职", Handling: "正常服务"},
		{ID: "g2", Name: "王敏", Code: "南山店-王敏", Count: "205 / 7", Status: "在职", Handling: "正常服务"},
		{ID: "g3", Name: "林浩", Code: "南山店-林浩", Count: "96 / 0", Status: "已调店", Handling: "6 人进交接池"},
		{ID: "g4", Name: "赵敏", Code: "南山店-赵敏", Count: "142 / 0", Status: "已离职", Handling: "9 人强制交接"},
	})
}

func customersHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, []Customer{
		{ID: "c1", Name: "陈女士", Wecom: "小陈", Mobile: "138****8821", Owner: "江诗颖", Source: "南山店-江诗颖", Stage: "新客待转化", TagGroup: "行为标签 / 生命周期标签", Tags: []string{"线下门店", "高意向"}, WecomTags: []string{"企微好友", "南山门店"}, LastActive: "今天 15:10", Relations: []Relation{{Guide: "江诗颖", Code: "南山店-江诗颖", Store: "蔻斯汀 / 南山店", LinkedAt: "2026-06-24", EndedAt: "-", Main: true}}},
		{ID: "c2", Name: "王女士", Wecom: "Wendy", Mobile: "136****9120", Owner: "王敏", Source: "南山店-王敏", Stage: "复购培育", TagGroup: "会员标签 / 生命周期标签", Tags: []string{"VIP", "已成交"}, WecomTags: []string{"企微好友", "老客"}, LastActive: "昨天 20:10", Relations: []Relation{{Guide: "王敏", Code: "南山店-王敏", Store: "蔻斯汀 / 南山店", LinkedAt: "2026-06-20", EndedAt: "-", Main: true}}},
		{ID: "c3", Name: "李先生", Wecom: "Lee", Mobile: "未绑定手机号", Owner: "赵敏", Source: "南山店-赵敏", Stage: "待交接", TagGroup: "风险标签 / 生命周期标签", Tags: []string{"待补资料"}, WecomTags: []string{"企微好友"}, LastActive: "今天 13:02", Relations: []Relation{{Guide: "赵敏", Code: "南山店-赵敏", Store: "蔻斯汀 / 南山店", LinkedAt: "2026-06-18", EndedAt: "-", Main: true}}},
	})
}

func groupsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, []Group{
		{ID: "cg1", Name: "南山店会员福利群", Owner: "江诗颖", Tags: []string{"门店群", "会员福利"}, Count: 286, TodayJoin: 18, TodayQuit: 2, CreatedAt: "2026-06-09 10:40"},
		{ID: "cg2", Name: "618 试用活动群", Owner: "张婷", Tags: []string{"活动群", "高意向"}, Count: 198, TodayJoin: 9, TodayQuit: 1, CreatedAt: "2026-06-10 14:22"},
	})
}

func touchesHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, []Touch{
		{ID: "t1", Name: "门店默认欢迎语", Type: "欢迎语", Scope: "全部线下门店", Status: "启用"},
		{ID: "t2", Name: "会员日活动通知", Type: "群发任务", Scope: "华南直营门店", Status: "执行中"},
		{ID: "t3", Name: "新客 1/3/7 天培育", Type: "SOP", Scope: "新入池客户", Status: "启用"},
	})
}

func tagsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var tag Tag
		if err := json.NewDecoder(r.Body).Decode(&tag); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if tag.Name == "" {
			http.Error(w, "tag name is required", http.StatusBadRequest)
			return
		}
		tag.Status = "正常"
		tags = append([]Tag{tag}, tags...)
		writeJSON(w, tag)
		return
	}
	writeJSON(w, tags)
}

func tagActionHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/tags/"), "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	name, err := url.PathUnescape(parts[0])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	action := parts[1]
	for i := range tags {
		if tags[i].Name != name {
			continue
		}
		switch action {
		case "toggle":
			if tags[i].Status == "正常" {
				tags[i].Status = "停用"
			} else {
				tags[i].Status = "正常"
			}
			writeJSON(w, tags[i])
		case "rename":
			var payload struct{ Name string `json:"name"` }
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload.Name != "" {
				tags[i].Name = payload.Name
			}
			writeJSON(w, tags[i])
		default:
			http.NotFound(w, r)
		}
		return
	}
	http.NotFound(w, r)
}

func tagGroupsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var group TagGroup
		if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for i := range tagGroups {
			if tagGroups[i].Name == group.Name {
				tagGroups[i] = group
				writeJSON(w, group)
				return
			}
		}
		tagGroups = append(tagGroups, group)
		writeJSON(w, group)
		return
	}
	writeJSON(w, tagGroups)
}
