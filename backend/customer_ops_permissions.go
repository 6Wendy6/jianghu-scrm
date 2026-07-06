package main

import (
	"net/http"
	"strconv"
	"strings"
)

const (
	opsRoleHeadquartersAdmin    = "headquarters_admin"
	opsRoleHeadquartersOperator = "headquarters_operator"
	opsRoleRegionalManager      = "regional_manager"
	opsRoleStoreOwner           = "store_owner"
)

type opsDataScope struct {
	Role     string
	RegionID string
	StoreID  string
	GuideID  string
}

func opsScopeFromRequest(r *http.Request) opsDataScope {
	role := strings.TrimSpace(r.Header.Get("X-SCRM-Role"))
	switch role {
	case "", "hq", "hq_admin":
		role = opsRoleHeadquartersAdmin
	case "region_manager":
		role = opsRoleRegionalManager
	}
	return opsDataScope{
		Role:     role,
		RegionID: strings.TrimSpace(r.Header.Get("X-SCRM-Region-ID")),
		StoreID:  strings.TrimSpace(r.Header.Get("X-SCRM-Store-ID")),
		GuideID:  strings.TrimSpace(r.Header.Get("X-SCRM-Guide-ID")),
	}
}

func (scope opsDataScope) applyCustomerDataScope(alias string, conditions *[]string, args *[]any) {
	prefix := scopeAlias(alias)
	switch scope.Role {
	case opsRoleRegionalManager:
		if scope.RegionID != "" {
			*args = append(*args, scope.RegionID)
			*conditions = append(*conditions, prefix+"region_id = $"+strconv.Itoa(len(*args)))
		}
	case opsRoleStore, opsRoleStoreOwner:
		if scope.StoreID != "" {
			*args = append(*args, scope.StoreID)
			*conditions = append(*conditions, prefix+"store_id = $"+strconv.Itoa(len(*args)))
		}
	case opsRoleGuide:
		if scope.GuideID != "" {
			*args = append(*args, scope.GuideID)
			*conditions = append(*conditions, prefix+"owner_guide_id = $"+strconv.Itoa(len(*args)))
		}
	case opsRoleHeadquartersOperator:
		if scope.RegionID != "" {
			*args = append(*args, scope.RegionID)
			*conditions = append(*conditions, prefix+"region_id = $"+strconv.Itoa(len(*args)))
		}
	}
}

func (scope opsDataScope) applyTaskDataScope(alias string, conditions *[]string, args *[]any) {
	prefix := scopeAlias(alias)
	switch scope.Role {
	case opsRoleRegionalManager:
		if scope.RegionID != "" {
			*args = append(*args, scope.RegionID)
			*conditions = append(*conditions, prefix+"region_id = $"+strconv.Itoa(len(*args)))
		}
	case opsRoleStore, opsRoleStoreOwner:
		if scope.StoreID != "" {
			*args = append(*args, scope.StoreID)
			*conditions = append(*conditions, prefix+"store_id = $"+strconv.Itoa(len(*args)))
		}
	case opsRoleGuide:
		if scope.GuideID != "" {
			*args = append(*args, scope.GuideID)
			*conditions = append(*conditions, prefix+"assigned_to_user_id = $"+strconv.Itoa(len(*args)))
		}
	case opsRoleHeadquartersOperator:
		if scope.RegionID != "" {
			*args = append(*args, scope.RegionID)
			*conditions = append(*conditions, prefix+"region_id = $"+strconv.Itoa(len(*args)))
		}
	}
}

func (scope opsDataScope) applyExceptionDataScope(alias string, conditions *[]string, args *[]any) {
	prefix := scopeAlias(alias)
	switch scope.Role {
	case opsRoleRegionalManager:
		if scope.RegionID != "" {
			*args = append(*args, scope.RegionID)
			*conditions = append(*conditions, prefix+"region_id = $"+strconv.Itoa(len(*args)))
		}
	case opsRoleStore, opsRoleStoreOwner:
		if scope.StoreID != "" {
			*args = append(*args, scope.StoreID)
			*conditions = append(*conditions, prefix+"store_id = $"+strconv.Itoa(len(*args)))
		}
	case opsRoleGuide:
		if scope.GuideID != "" {
			*args = append(*args, scope.GuideID)
			*conditions = append(*conditions, "EXISTS (SELECT 1 FROM customers c WHERE c.id = "+prefix+"customer_id AND c.owner_guide_id = $"+strconv.Itoa(len(*args))+")")
		}
	case opsRoleHeadquartersOperator:
		if scope.RegionID != "" {
			*args = append(*args, scope.RegionID)
			*conditions = append(*conditions, prefix+"region_id = $"+strconv.Itoa(len(*args)))
		}
	}
}

func scopeAlias(alias string) string {
	if alias == "" {
		return ""
	}
	return alias + "."
}

func whereClause(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conditions, " AND ")
}
