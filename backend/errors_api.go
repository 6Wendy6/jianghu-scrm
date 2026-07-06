package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type apiErrorPayload struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

type codedError struct {
	code    string
	message string
	details map[string]any
	cause   error
}

func (e codedError) Error() string {
	if e.message != "" {
		return e.message
	}
	if e.cause != nil {
		return e.cause.Error()
	}
	return e.code
}

func (e codedError) Unwrap() error {
	return e.cause
}

func newCodedError(code, message string, cause error, details map[string]any) error {
	code = strings.TrimSpace(code)
	if code == "" {
		code = "SYSTEM_ERROR"
	}
	if message == "" && cause != nil {
		message = cause.Error()
	}
	return codedError{code: code, message: message, cause: cause, details: details}
}

func apiErrorResponse(err error, requestID string) apiErrorPayload {
	payload := apiErrorPayload{
		Code:      inferErrorCode(err),
		Message:   trimErrorMessage(err),
		RequestID: requestID,
	}
	var coded codedError
	if errors.As(err, &coded) {
		payload.Code = coded.code
		if coded.message != "" {
			payload.Message = coded.message
		}
		if len(coded.details) > 0 {
			payload.Details = coded.details
		}
	}
	return payload
}

func inferErrorCode(err error) string {
	if errors.Is(err, errForbidden) {
		return "PERMISSION_DENIED"
	}
	if errors.Is(err, errBadRequest) {
		return "BAD_REQUEST"
	}
	if errors.Is(err, errNotFound) {
		return "NOT_FOUND"
	}
	return "SYSTEM_ERROR"
}

func trimErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	for _, prefix := range []string{"bad request: ", "forbidden: ", "not found: "} {
		message = strings.TrimPrefix(message, prefix)
	}
	return message
}

func wecomAPIError(operation string, errcode int, errmsg string) error {
	entry := explainWeComError(errcode)
	message := entry.Message
	if message == "" {
		message = fmt.Sprintf("企业微信接口调用失败：%s", strings.TrimSpace(errmsg))
	}
	details := map[string]any{
		"operation":  operation,
		"errcode":    errcode,
		"errmsg":     errmsg,
		"suggestion": entry.Suggestion,
	}
	return newCodedError(entry.Code, message, errBadRequest, details)
}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(requestIDKey{}).(string); ok {
		return value
	}
	return ""
}

func rawJSONMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"raw": string(raw)}
	}
	return out
}
