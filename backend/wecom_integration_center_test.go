package main

import (
	"errors"
	"testing"
)

func TestWeComErrorDictionaryKnownCodes(t *testing.T) {
	entry := explainWeComError(60011)
	if entry.Code != "WECOM_PERMISSION_DENIED" {
		t.Fatalf("expected WECOM_PERMISSION_DENIED, got %s", entry.Code)
	}
	if entry.Suggestion == "" {
		t.Fatalf("expected suggestion for 60011")
	}
}

func TestWeComErrorDictionaryUnknownCode(t *testing.T) {
	entry := explainWeComError(999999)
	if entry.Code != "WECOM_ERR_999999" {
		t.Fatalf("unexpected unknown code mapping: %s", entry.Code)
	}
}

func TestAPIErrorResponseKeepsCodedDetails(t *testing.T) {
	err := wecomAPIError("gettoken", 60020, "not allow to access from your ip")
	payload := apiErrorResponse(err, "req-test")
	if payload.Code != "WECOM_IP_NOT_ALLOWED" {
		t.Fatalf("expected WECOM_IP_NOT_ALLOWED, got %s", payload.Code)
	}
	if payload.RequestID != "req-test" {
		t.Fatalf("expected request id to be preserved")
	}
	if payload.Details["errcode"] != 60020 {
		t.Fatalf("expected errcode details, got %#v", payload.Details)
	}
	if !errors.Is(err, errBadRequest) {
		t.Fatalf("coded wecom api error should unwrap to errBadRequest")
	}
}

func TestMaskMiddle(t *testing.T) {
	if got := maskMiddle("ww123456789"); got != "ww1***789" {
		t.Fatalf("unexpected mask: %s", got)
	}
	if got := maskMiddle("abc"); got != "a***c" {
		t.Fatalf("unexpected short mask: %s", got)
	}
}
