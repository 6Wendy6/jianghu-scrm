package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSCRMContactWayStateCarriesBindingID(t *testing.T) {
	state := buildSCRMContactWayState("BU:001", "manager user", "guide:1", "binding:1")
	if strings.Contains(state, "BU:001") || strings.Contains(state, "manager user") || strings.Contains(state, "guide:1") {
		t.Fatalf("state should sanitize colon and spaces, got %q", state)
	}
	if got := parseBindingIDFromSCRMState(state); got != "binding_1" {
		t.Fatalf("binding id mismatch: got %q", got)
	}
}

func TestPositiveModuloSupportsRoundRobinWrap(t *testing.T) {
	cases := []struct {
		value int
		mod   int
		want  int
	}{
		{0, 3, 0},
		{1, 3, 1},
		{3, 3, 0},
		{-1, 3, 2},
		{5, 0, 0},
	}
	for _, tc := range cases {
		if got := positiveModulo(tc.value, tc.mod); got != tc.want {
			t.Fatalf("positiveModulo(%d,%d)=%d want %d", tc.value, tc.mod, got, tc.want)
		}
	}
}

func TestCustomerAssignmentFiltersUseExpectedColumns(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/scrm/customer-assignments?storeId=s1&managerUserid=m1&guideUserid=g1&configId=c1&status=added&start=2026-07-04T00:00&end=2026-07-04T23:59", nil)
	where, args := scrmCustomerAssignmentFilters(req)
	for _, fragment := range []string{
		"a.store_id = $1",
		"a.manager_userid = $2",
		"a.guide_userid = $3",
		"a.config_id = $4",
		"a.status = $5",
		"COALESCE(a.add_time, a.created_at) >= $6::timestamptz",
		"COALESCE(a.add_time, a.created_at) <= $7::timestamptz",
	} {
		if !strings.Contains(where, fragment) {
			t.Fatalf("where clause missing %q: %s", fragment, where)
		}
	}
	if len(args) != 7 {
		t.Fatalf("args length=%d want 7", len(args))
	}
}

func TestDoctorOverallStatus(t *testing.T) {
	ok := []scrmWeComCheck{{Name: "access_token", Status: "ok"}}
	if got := doctorOverallStatus(ok); got != "ok" {
		t.Fatalf("doctorOverallStatus ok=%q", got)
	}
	attention := []scrmWeComCheck{{Name: "access_token", Status: "fail"}}
	if got := doctorOverallStatus(attention); got != "attention" {
		t.Fatalf("doctorOverallStatus attention=%q", got)
	}
}
