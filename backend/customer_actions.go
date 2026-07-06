package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func (api *API) customerActionHandler(w http.ResponseWriter, r *http.Request) error {
	parts := pathParts(r.URL.Path, "/api/customers/")
	if len(parts) == 0 {
		return errNotFound
	}
	if len(parts) == 1 && parts[0] == "batch-tags" {
		return api.batchTags(w, r)
	}
	if len(parts) == 1 && parts[0] == "batch-tags-async" {
		return api.enqueueBatchTags(w, r)
	}
	if len(parts) >= 2 {
		switch parts[1] {
		case "stage", "tags", "follow-ups", "timeline":
			return api.opsCustomerActionHandler(w, r, parts[0], parts[1:])
		}
	}
	if api.db != nil && len(parts) == 1 && r.Method == http.MethodGet && strings.HasPrefix(parts[0], "oc") {
		return api.opsCustomerActionHandler(w, r, parts[0], nil)
	}
	if len(parts) == 1 && r.Method == http.MethodGet {
		api.mu.RLock()
		opsCustomerExists := api.findOpsCustomerLocked(parts[0]) != nil
		api.mu.RUnlock()
		if opsCustomerExists {
			return api.opsCustomerActionHandler(w, r, parts[0], nil)
		}
	}
	if api.db != nil {
		return api.customerActionDBHandler(w, r, parts)
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
