package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
)

const (
	wecomTokenContact  = "contact_token"
	wecomTokenCustomer = "customer_token"
	wecomTokenApp      = "app_token"
)

type WeComIntegrationConfig struct {
	ID                       string `json:"id"`
	CorpID                   string `json:"corpId"`
	CorpIDMasked             string `json:"corpIdMasked,omitempty"`
	ContactSecretConfigured  bool   `json:"contactSecretConfigured"`
	ContactSecretMasked      string `json:"contactSecretMasked,omitempty"`
	CustomerSecretConfigured bool   `json:"customerSecretConfigured"`
	CustomerSecretMasked     string `json:"customerSecretMasked,omitempty"`
	AgentID                  string `json:"agentId"`
	AgentIDMasked            string `json:"agentIdMasked,omitempty"`
	AgentSecretConfigured    bool   `json:"agentSecretConfigured"`
	AgentSecretMasked        string `json:"agentSecretMasked,omitempty"`
	CallbackTokenConfigured  bool   `json:"callbackTokenConfigured"`
	EncodingAESKeyConfigured bool   `json:"encodingAesKeyConfigured"`
	Enabled                  bool   `json:"enabled"`
	CreatedAt                string `json:"createdAt,omitempty"`
	UpdatedAt                string `json:"updatedAt,omitempty"`
	contactSecretEncrypted   string
	customerSecretEncrypted  string
	agentSecretEncrypted     string
	callbackToken            string
	encodingAESKey           string
}

type WeComConfigPayload struct {
	CorpID         string `json:"corpId"`
	ContactSecret  string `json:"contactSecret"`
	CustomerSecret string `json:"customerSecret"`
	AgentID        string `json:"agentId"`
	AgentSecret    string `json:"agentSecret"`
	CallbackToken  string `json:"callbackToken"`
	EncodingAESKey string `json:"encodingAesKey"`
	Enabled        *bool  `json:"enabled"`
}

type WeComTokenRecord struct {
	TokenType        string `json:"tokenType"`
	Status           string `json:"status"`
	ExpiresAt        string `json:"expiresAt,omitempty"`
	LastErrorCode    string `json:"lastErrorCode,omitempty"`
	LastErrorMessage string `json:"lastErrorMessage,omitempty"`
	UpdatedAt        string `json:"updatedAt,omitempty"`
	accessToken      string
	expiresAtTime    time.Time
}

type WeComTokenTestResult struct {
	TokenType string `json:"tokenType"`
	Status    string `json:"status"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

type WeComPermissionCheck struct {
	ID         string `json:"id"`
	CheckType  string `json:"checkType"`
	Status     string `json:"status"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
	LocalCode  string `json:"localCode"`
	Suggestion string `json:"suggestion"`
	CheckedAt  string `json:"checkedAt"`
}

func (api *API) scrmWeComConfigHandler(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		cfg, err := api.getSCRMWeComConfig(r.Context(), false)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return writeJSON(w, map[string]any{"configured": false})
			}
			return err
		}
		return writeJSON(w, map[string]any{"configured": true, "config": cfg})
	case http.MethodPut:
		var payload WeComConfigPayload
		if err := decode(r, &payload); err != nil {
			return err
		}
		cfg, err := api.saveSCRMWeComConfig(r.Context(), payload)
		if err != nil {
			return err
		}
		return writeJSON(w, map[string]any{"status": "ok", "config": cfg})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
}

func (api *API) getSCRMWeComConfig(ctx context.Context, includeSecrets bool) (WeComIntegrationConfig, error) {
	if api.db != nil {
		return api.getSCRMWeComConfigDB(ctx, includeSecrets)
	}
	api.mu.RLock()
	defer api.mu.RUnlock()
	if api.wecomConfigV2 == nil {
		return WeComIntegrationConfig{}, sql.ErrNoRows
	}
	cfg := *api.wecomConfigV2
	return sanitizeSCRMWeComConfig(cfg, includeSecrets), nil
}

func (api *API) saveSCRMWeComConfig(ctx context.Context, payload WeComConfigPayload) (WeComIntegrationConfig, error) {
	payload.CorpID = strings.TrimSpace(payload.CorpID)
	if payload.CorpID == "" {
		return WeComIntegrationConfig{}, newCodedError("WECOM_CONFIG_INVALID", "corpId 不能为空", errBadRequest, map[string]any{"field": "corpId"})
	}
	if api.db != nil {
		return api.saveSCRMWeComConfigDB(ctx, payload)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	existing := WeComIntegrationConfig{}
	if api.wecomConfigV2 != nil {
		existing = *api.wecomConfigV2
	}
	cfg, err := api.buildSCRMWeComConfig(existing, payload)
	if err != nil {
		return WeComIntegrationConfig{}, err
	}
	if cfg.ID == "" {
		cfg.ID = newID("swc")
		cfg.CreatedAt = stamp()
	}
	cfg.UpdatedAt = stamp()
	api.wecomConfigV2 = &cfg
	return sanitizeSCRMWeComConfig(cfg, false), nil
}

func (api *API) buildSCRMWeComConfig(existing WeComIntegrationConfig, payload WeComConfigPayload) (WeComIntegrationConfig, error) {
	cfg := existing
	cfg.CorpID = strings.TrimSpace(payload.CorpID)
	cfg.AgentID = strings.TrimSpace(payload.AgentID)
	if payload.Enabled == nil {
		cfg.Enabled = true
	} else {
		cfg.Enabled = *payload.Enabled
	}
	var err error
	if strings.TrimSpace(payload.ContactSecret) != "" {
		cfg.contactSecretEncrypted, err = api.secretCipher.Encrypt(payload.ContactSecret)
		if err != nil {
			return WeComIntegrationConfig{}, err
		}
	}
	if strings.TrimSpace(payload.CustomerSecret) != "" {
		cfg.customerSecretEncrypted, err = api.secretCipher.Encrypt(payload.CustomerSecret)
		if err != nil {
			return WeComIntegrationConfig{}, err
		}
	}
	if strings.TrimSpace(payload.AgentSecret) != "" {
		cfg.agentSecretEncrypted, err = api.secretCipher.Encrypt(payload.AgentSecret)
		if err != nil {
			return WeComIntegrationConfig{}, err
		}
	}
	cfg.callbackToken = strings.TrimSpace(payload.CallbackToken)
	cfg.encodingAESKey = strings.TrimSpace(payload.EncodingAESKey)
	cfg.ContactSecretConfigured = cfg.contactSecretEncrypted != ""
	cfg.CustomerSecretConfigured = cfg.customerSecretEncrypted != ""
	cfg.AgentSecretConfigured = cfg.agentSecretEncrypted != ""
	cfg.CallbackTokenConfigured = cfg.callbackToken != ""
	cfg.EncodingAESKeyConfigured = cfg.encodingAESKey != ""
	return cfg, nil
}

func sanitizeSCRMWeComConfig(cfg WeComIntegrationConfig, includeSecrets bool) WeComIntegrationConfig {
	cfg.CorpIDMasked = maskMiddle(cfg.CorpID)
	cfg.AgentIDMasked = maskMiddle(cfg.AgentID)
	cfg.ContactSecretConfigured = cfg.contactSecretEncrypted != ""
	cfg.CustomerSecretConfigured = cfg.customerSecretEncrypted != ""
	cfg.AgentSecretConfigured = cfg.agentSecretEncrypted != ""
	cfg.CallbackTokenConfigured = cfg.callbackToken != ""
	cfg.EncodingAESKeyConfigured = cfg.encodingAESKey != ""
	if cfg.ContactSecretConfigured {
		cfg.ContactSecretMasked = "已配置"
	}
	if cfg.CustomerSecretConfigured {
		cfg.CustomerSecretMasked = "已配置"
	}
	if cfg.AgentSecretConfigured {
		cfg.AgentSecretMasked = "已配置"
	}
	if !includeSecrets {
		cfg.contactSecretEncrypted = ""
		cfg.customerSecretEncrypted = ""
		cfg.agentSecretEncrypted = ""
		cfg.callbackToken = ""
		cfg.encodingAESKey = ""
	}
	return cfg
}

func (api *API) getSCRMWeComConfigDB(ctx context.Context, includeSecrets bool) (WeComIntegrationConfig, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var cfg WeComIntegrationConfig
	err := api.db.QueryRowContext(queryCtx, `
		SELECT id, corp_id, contact_secret_encrypted, customer_secret_encrypted, agent_id,
			agent_secret_encrypted, callback_token, encoding_aes_key, enabled,
			to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'),
			to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM wecom_configs
		ORDER BY updated_at DESC
		LIMIT 1
	`).Scan(&cfg.ID, &cfg.CorpID, &cfg.contactSecretEncrypted, &cfg.customerSecretEncrypted, &cfg.AgentID, &cfg.agentSecretEncrypted, &cfg.callbackToken, &cfg.encodingAESKey, &cfg.Enabled, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return WeComIntegrationConfig{}, err
	}
	return sanitizeSCRMWeComConfig(cfg, includeSecrets), nil
}

func (api *API) saveSCRMWeComConfigDB(ctx context.Context, payload WeComConfigPayload) (WeComIntegrationConfig, error) {
	existing, _ := api.getSCRMWeComConfigDB(ctx, true)
	cfg, err := api.buildSCRMWeComConfig(existing, payload)
	if err != nil {
		return WeComIntegrationConfig{}, err
	}
	id := newID("swc")
	if existing.ID != "" && existing.CorpID == cfg.CorpID {
		id = existing.ID
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = api.db.ExecContext(ctx, `
		INSERT INTO wecom_configs (
			id, corp_id, contact_secret_encrypted, customer_secret_encrypted, agent_id,
			agent_secret_encrypted, callback_token, encoding_aes_key, enabled
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (corp_id) DO UPDATE SET
			contact_secret_encrypted = CASE WHEN EXCLUDED.contact_secret_encrypted <> '' THEN EXCLUDED.contact_secret_encrypted ELSE wecom_configs.contact_secret_encrypted END,
			customer_secret_encrypted = CASE WHEN EXCLUDED.customer_secret_encrypted <> '' THEN EXCLUDED.customer_secret_encrypted ELSE wecom_configs.customer_secret_encrypted END,
			agent_id = EXCLUDED.agent_id,
			agent_secret_encrypted = CASE WHEN EXCLUDED.agent_secret_encrypted <> '' THEN EXCLUDED.agent_secret_encrypted ELSE wecom_configs.agent_secret_encrypted END,
			callback_token = EXCLUDED.callback_token,
			encoding_aes_key = EXCLUDED.encoding_aes_key,
			enabled = EXCLUDED.enabled,
			updated_at = now()
	`, id, cfg.CorpID, cfg.contactSecretEncrypted, cfg.customerSecretEncrypted, cfg.AgentID, cfg.agentSecretEncrypted, cfg.callbackToken, cfg.encodingAESKey, cfg.Enabled)
	if err != nil {
		return WeComIntegrationConfig{}, err
	}
	return api.getSCRMWeComConfigDB(ctx, false)
}

func (api *API) secretForTokenType(ctx context.Context, cfg WeComIntegrationConfig, tokenType string) (string, error) {
	var encrypted string
	switch tokenType {
	case wecomTokenContact:
		encrypted = cfg.contactSecretEncrypted
	case wecomTokenCustomer:
		encrypted = cfg.customerSecretEncrypted
	case wecomTokenApp:
		encrypted = cfg.agentSecretEncrypted
	default:
		return "", newCodedError("WECOM_TOKEN_TYPE_INVALID", "不支持的 tokenType", errBadRequest, map[string]any{"tokenType": tokenType})
	}
	if encrypted == "" {
		return "", newCodedError("WECOM_SECRET_MISSING", "当前 token 类型未配置 secret", errBadRequest, map[string]any{"tokenType": tokenType})
	}
	secret, err := api.secretCipher.Decrypt(encrypted)
	if err != nil {
		return "", newCodedError("WECOM_SECRET_DECRYPT_FAILED", "企微 secret 解密失败", errBadRequest, map[string]any{"tokenType": tokenType})
	}
	return secret, nil
}

func allWeComTokenTypes() []string {
	return []string{wecomTokenContact, wecomTokenCustomer, wecomTokenApp}
}
