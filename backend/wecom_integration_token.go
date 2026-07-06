package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
)

func (api *API) scrmWeComTestTokenHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	var payload struct {
		TokenType string `json:"tokenType"`
	}
	if err := decode(r, &payload); err != nil {
		return err
	}
	tokenTypes := []string{}
	if strings.TrimSpace(payload.TokenType) == "" || payload.TokenType == "all" {
		tokenTypes = allWeComTokenTypes()
	} else {
		tokenTypes = []string{strings.TrimSpace(payload.TokenType)}
	}
	results := []WeComTokenTestResult{}
	for _, tokenType := range tokenTypes {
		result, err := api.testSCRMWeComToken(r.Context(), tokenType)
		if err != nil {
			if len(tokenTypes) == 1 {
				return err
			}
			entry := explainError(err)
			_ = api.cacheSCRMWeComTokenFailure(r.Context(), tokenType, entry.Code, entry.Message)
			results = append(results, WeComTokenTestResult{TokenType: tokenType, Status: "failed"})
			continue
		}
		results = append(results, result)
	}
	return writeJSON(w, map[string]any{"results": results})
}

func (api *API) testSCRMWeComToken(ctx context.Context, tokenType string) (WeComTokenTestResult, error) {
	cfg, err := api.getSCRMWeComConfig(ctx, true)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WeComTokenTestResult{}, newCodedError("WECOM_CONFIG_MISSING", "未找到企业微信配置，请先保存 corpId 和 secret", errBadRequest, map[string]any{
				"suggestion": "保存企微配置后再测试 token",
			})
		}
		return WeComTokenTestResult{}, err
	}
	if !cfg.Enabled {
		return WeComTokenTestResult{}, newCodedError("WECOM_CONFIG_DISABLED", "企业微信配置未启用", errBadRequest, map[string]any{"suggestion": "启用配置后再测试 token"})
	}
	secret, err := api.secretForTokenType(ctx, cfg, tokenType)
	if err != nil {
		_ = api.cacheSCRMWeComTokenFailure(ctx, tokenType, inferErrorCode(err), trimErrorMessage(err))
		return WeComTokenTestResult{}, err
	}
	client := api.activeWeComClient()
	token, err := client.GetAccessToken(ctx, cfg.CorpID, secret)
	if err != nil {
		entry := explainError(err)
		_ = api.cacheSCRMWeComTokenFailure(ctx, tokenType, entry.Code, entry.Message)
		return WeComTokenTestResult{}, err
	}
	expiresAt := time.Now().Add(time.Duration(max(60, token.ExpiresIn-300)) * time.Second)
	if err := api.cacheSCRMWeComToken(ctx, cfg.CorpID, tokenType, token.AccessToken, expiresAt); err != nil {
		return WeComTokenTestResult{}, err
	}
	return WeComTokenTestResult{TokenType: tokenType, Status: "ok", ExpiresAt: expiresAt.Format(time.RFC3339)}, nil
}

func (api *API) activeWeComClient() WeComClient {
	if api.wecomClient != nil {
		return api.wecomClient
	}
	return realWeComClient{}
}

func (api *API) cacheSCRMWeComToken(ctx context.Context, corpID, tokenType, accessToken string, expiresAt time.Time) error {
	if api.db != nil {
		_, err := api.db.ExecContext(ctx, `
			INSERT INTO wecom_tokens (id, corp_id, token_type, access_token, expires_at, last_error_code, last_error_message, updated_at)
			VALUES ($1,$2,$3,$4,$5,'','',now())
			ON CONFLICT (corp_id, token_type) DO UPDATE SET
				access_token = EXCLUDED.access_token,
				expires_at = EXCLUDED.expires_at,
				last_error_code = '',
				last_error_message = '',
				updated_at = now()
		`, newID("swt"), corpID, tokenType, accessToken, expiresAt)
		return err
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.wecomTokens == nil {
		api.wecomTokens = map[string]WeComTokenRecord{}
	}
	api.wecomTokens[tokenType] = WeComTokenRecord{
		TokenType:     tokenType,
		Status:        "ok",
		ExpiresAt:     expiresAt.Format(time.RFC3339),
		UpdatedAt:     stamp(),
		accessToken:   accessToken,
		expiresAtTime: expiresAt,
	}
	return nil
}

func (api *API) cacheSCRMWeComTokenFailure(ctx context.Context, tokenType, code, message string) error {
	cfg, _ := api.getSCRMWeComConfig(ctx, true)
	if api.db != nil && cfg.CorpID != "" {
		_, err := api.db.ExecContext(ctx, `
			INSERT INTO wecom_tokens (id, corp_id, token_type, access_token, last_error_code, last_error_message, updated_at)
			VALUES ($1,$2,$3,'',$4,$5,now())
			ON CONFLICT (corp_id, token_type) DO UPDATE SET
				last_error_code = EXCLUDED.last_error_code,
				last_error_message = EXCLUDED.last_error_message,
				updated_at = now()
		`, newID("swt"), cfg.CorpID, tokenType, code, message)
		return err
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.wecomTokens == nil {
		api.wecomTokens = map[string]WeComTokenRecord{}
	}
	api.wecomTokens[tokenType] = WeComTokenRecord{TokenType: tokenType, Status: "failed", LastErrorCode: code, LastErrorMessage: message, UpdatedAt: stamp()}
	return nil
}

func (api *API) latestSCRMWeComTokens(ctx context.Context) map[string]wecomTokenStatus {
	result := map[string]wecomTokenStatus{}
	for _, tokenType := range allWeComTokenTypes() {
		result[tokenType] = wecomTokenStatus{Cached: false, Valid: false}
	}
	if api.db != nil {
		cfg, err := api.getSCRMWeComConfig(ctx, true)
		if err != nil {
			return result
		}
		rows, err := api.db.QueryContext(ctx, `
			SELECT token_type, access_token, expires_at, last_error_code, last_error_message
			FROM wecom_tokens
			WHERE corp_id = $1
		`, cfg.CorpID)
		if err != nil {
			return result
		}
		defer rows.Close()
		for rows.Next() {
			var tokenType, accessToken, errorCode, errorMessage string
			var expiresAt sql.NullTime
			if err := rows.Scan(&tokenType, &accessToken, &expiresAt, &errorCode, &errorMessage); err != nil {
				continue
			}
			status := wecomTokenStatus{Cached: accessToken != "", Valid: false}
			if expiresAt.Valid {
				status.ExpiresAt = expiresAt.Time.Format(time.RFC3339)
				status.SecondsLeft = int64(time.Until(expiresAt.Time).Seconds())
				status.Valid = accessToken != "" && status.SecondsLeft > 300
			}
			result[tokenType] = status
		}
		return result
	}
	api.mu.RLock()
	defer api.mu.RUnlock()
	for tokenType, record := range api.wecomTokens {
		status := wecomTokenStatus{Cached: record.accessToken != "", Valid: false, ExpiresAt: record.ExpiresAt}
		if !record.expiresAtTime.IsZero() {
			status.SecondsLeft = int64(time.Until(record.expiresAtTime).Seconds())
			status.Valid = record.accessToken != "" && status.SecondsLeft > 300
		}
		result[tokenType] = status
	}
	return result
}

func (api *API) tokenAccessValue(ctx context.Context, tokenType string) (string, error) {
	if api.db != nil {
		cfg, err := api.getSCRMWeComConfig(ctx, true)
		if err != nil {
			return "", err
		}
		var token string
		var expiresAt sql.NullTime
		err = api.db.QueryRowContext(ctx, `
			SELECT access_token, expires_at
			FROM wecom_tokens
			WHERE corp_id = $1 AND token_type = $2
		`, cfg.CorpID, tokenType).Scan(&token, &expiresAt)
		if err == nil && token != "" && expiresAt.Valid && time.Until(expiresAt.Time) > 300*time.Second {
			return token, nil
		}
	}
	result, err := api.testSCRMWeComToken(ctx, tokenType)
	if err != nil {
		return "", err
	}
	if result.Status != "ok" {
		return "", newCodedError("WECOM_TOKEN_UNAVAILABLE", "token 不可用", errBadRequest, map[string]any{"tokenType": tokenType})
	}
	if api.db != nil {
		cfg, _ := api.getSCRMWeComConfig(ctx, true)
		var token string
		_ = api.db.QueryRowContext(ctx, `SELECT access_token FROM wecom_tokens WHERE corp_id = $1 AND token_type = $2`, cfg.CorpID, tokenType).Scan(&token)
		return token, nil
	}
	api.mu.RLock()
	defer api.mu.RUnlock()
	return api.wecomTokens[tokenType].accessToken, nil
}

type explainedError struct {
	Code       string
	Message    string
	Suggestion string
	ErrCode    int
	ErrMsg     string
}

func explainError(err error) explainedError {
	payload := apiErrorResponse(err, "")
	out := explainedError{Code: payload.Code, Message: payload.Message}
	if payload.Details != nil {
		if suggestion, ok := payload.Details["suggestion"].(string); ok {
			out.Suggestion = suggestion
		}
		if errcode, ok := payload.Details["errcode"].(int); ok {
			out.ErrCode = errcode
		}
		if errmsg, ok := payload.Details["errmsg"].(string); ok {
			out.ErrMsg = errmsg
		}
	}
	return out
}
