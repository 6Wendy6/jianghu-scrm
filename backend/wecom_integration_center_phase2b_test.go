package main

import (
	"context"
	"errors"
	"testing"
)

type fakeWeComClient struct {
	tokenResult TokenResult
	tokenErr    error
	deptErr     error
	userErr     error
	followErr   error
}

func (f fakeWeComClient) GetAccessToken(ctx context.Context, corpID string, secret string) (TokenResult, error) {
	if f.tokenErr != nil {
		return TokenResult{}, f.tokenErr
	}
	result := f.tokenResult
	if result.AccessToken == "" {
		result.AccessToken = "fake-access-token"
	}
	if result.ExpiresIn == 0 {
		result.ExpiresIn = 7200
	}
	return result, nil
}

func (f fakeWeComClient) GetDepartmentList(ctx context.Context, token string) error {
	return f.deptErr
}

func (f fakeWeComClient) GetUserList(ctx context.Context, token string, departmentID string) error {
	return f.userErr
}

func (f fakeWeComClient) GetFollowUserList(ctx context.Context, token string) error {
	return f.followErr
}

func newWeComPhase2BTestAPI() *API {
	api := newAPI()
	api.secretCipher = newSecretCipher("phase2b-test-key")
	api.wecomClient = fakeWeComClient{}
	return api
}

func TestSCRMWeComConfigMemorySaveReadMasksSecrets(t *testing.T) {
	api := newWeComPhase2BTestAPI()
	enabled := true
	cfg, err := api.saveSCRMWeComConfig(context.Background(), WeComConfigPayload{
		CorpID:         "ww123456789",
		ContactSecret:  "contact-secret",
		CustomerSecret: "customer-secret",
		AgentID:        "100001",
		AgentSecret:    "agent-secret",
		CallbackToken:  "callback-token",
		EncodingAESKey: "encoding-key",
		Enabled:        &enabled,
	})
	if err != nil {
		t.Fatalf("save config: %v", err)
	}
	if cfg.CorpID != "ww123456789" || cfg.CorpIDMasked != "ww1***789" {
		t.Fatalf("unexpected corp id masking: %#v", cfg)
	}
	if !cfg.ContactSecretConfigured || !cfg.CustomerSecretConfigured || !cfg.AgentSecretConfigured {
		t.Fatalf("expected all secrets configured: %#v", cfg)
	}
	if cfg.contactSecretEncrypted != "" || cfg.customerSecretEncrypted != "" || cfg.agentSecretEncrypted != "" {
		t.Fatalf("sanitized config leaked encrypted secret internals: %#v", cfg)
	}
	read, err := api.getSCRMWeComConfig(context.Background(), false)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if read.ContactSecretMasked != "已配置" || read.AgentSecretMasked != "已配置" {
		t.Fatalf("expected masked secret flags, got %#v", read)
	}
}

func TestSCRMWeComTestTokenCachesWithoutLeakingAccessToken(t *testing.T) {
	api := newWeComPhase2BTestAPI()
	api.wecomClient = fakeWeComClient{tokenResult: TokenResult{AccessToken: "top-secret-token", ExpiresIn: 7200}}
	_, err := api.saveSCRMWeComConfig(context.Background(), WeComConfigPayload{CorpID: "ww123456789", ContactSecret: "contact-secret"})
	if err != nil {
		t.Fatalf("save config: %v", err)
	}
	result, err := api.testSCRMWeComToken(context.Background(), wecomTokenContact)
	if err != nil {
		t.Fatalf("test token: %v", err)
	}
	if result.Status != "ok" || result.TokenType != wecomTokenContact || result.ExpiresAt == "" {
		t.Fatalf("unexpected token result: %#v", result)
	}
	if resultString := result.Status + result.ExpiresAt + result.TokenType; resultString == "top-secret-token" {
		t.Fatalf("token response leaked access token")
	}
	status := api.latestSCRMWeComTokens(context.Background())[wecomTokenContact]
	if !status.Cached || !status.Valid {
		t.Fatalf("expected cached valid token, got %#v", status)
	}
}

func TestSCRMWeComTestTokenFailureMapsIPWhitelist(t *testing.T) {
	api := newWeComPhase2BTestAPI()
	api.wecomClient = fakeWeComClient{tokenErr: wecomAPIError("gettoken", 60020, "not allow to access from your ip")}
	_, err := api.saveSCRMWeComConfig(context.Background(), WeComConfigPayload{CorpID: "ww123456789", ContactSecret: "contact-secret"})
	if err != nil {
		t.Fatalf("save config: %v", err)
	}
	_, err = api.testSCRMWeComToken(context.Background(), wecomTokenContact)
	if err == nil {
		t.Fatalf("expected token error")
	}
	payload := apiErrorResponse(err, "req-ip")
	if payload.Code != "WECOM_IP_NOT_ALLOWED" {
		t.Fatalf("expected WECOM_IP_NOT_ALLOWED, got %#v", payload)
	}
}

func TestSCRMWeComPermissionCheckRecordsFailureAndException(t *testing.T) {
	api := newWeComPhase2BTestAPI()
	api.wecomClient = fakeWeComClient{userErr: wecomAPIError("user_list", 60011, "permission denied")}
	_, err := api.saveSCRMWeComConfig(context.Background(), WeComConfigPayload{
		CorpID:         "ww123456789",
		ContactSecret:  "contact-secret",
		CustomerSecret: "customer-secret",
		AgentSecret:    "agent-secret",
	})
	if err != nil {
		t.Fatalf("save config: %v", err)
	}
	checks := api.runSCRMWeComPermissionCheck(context.Background(), "req-permission")
	foundFailed := false
	for _, check := range checks {
		if check.CheckType == "user_read" && check.Status == "failed" && check.LocalCode == "WECOM_PERMISSION_DENIED" {
			foundFailed = true
		}
	}
	if !foundFailed {
		t.Fatalf("expected failed user_read check, got %#v", checks)
	}
	api.mu.RLock()
	exceptions := append([]OpsException{}, api.opsExceptions...)
	api.mu.RUnlock()
	foundException := false
	for _, item := range exceptions {
		foundException = foundException || item.ExceptionType == "wecom_permission_error"
	}
	if !foundException {
		t.Fatalf("expected wecom permission exception, got %#v", exceptions)
	}
}

func TestSCRMWeComPhase2BDBPersistsConfigTokenAndChecks(t *testing.T) {
	api, _ := newOpsDBTestServer(t)
	api.wecomClient = fakeWeComClient{userErr: wecomAPIError("user_list", 60011, "permission denied")}
	enabled := true
	cfg, err := api.saveSCRMWeComConfig(context.Background(), WeComConfigPayload{
		CorpID:         "ww-db-test",
		ContactSecret:  "contact-secret",
		CustomerSecret: "customer-secret",
		AgentID:        "agent-1",
		AgentSecret:    "agent-secret",
		Enabled:        &enabled,
	})
	if err != nil {
		t.Fatalf("save db config: %v", err)
	}
	if !cfg.ContactSecretConfigured || cfg.contactSecretEncrypted != "" {
		t.Fatalf("expected sanitized db config, got %#v", cfg)
	}
	if _, err := api.testSCRMWeComToken(context.Background(), wecomTokenContact); err != nil {
		t.Fatalf("test db token: %v", err)
	}
	if _, err := api.saveSCRMWeComConfig(context.Background(), WeComConfigPayload{
		CorpID:        "ww-db-test-second",
		ContactSecret: "contact-secret-second",
		Enabled:       &enabled,
	}); err != nil {
		t.Fatalf("save second db config should not reuse old primary key: %v", err)
	}
	var token, encryptedSecret string
	if err := api.db.QueryRow(`SELECT access_token FROM wecom_tokens WHERE corp_id = 'ww-db-test' AND token_type = 'contact_token'`).Scan(&token); err != nil {
		t.Fatalf("query token: %v", err)
	}
	if token == "" {
		t.Fatalf("expected token cached in DB")
	}
	if err := api.db.QueryRow(`SELECT contact_secret_encrypted FROM wecom_configs WHERE corp_id = 'ww-db-test'`).Scan(&encryptedSecret); err != nil {
		t.Fatalf("query config secret: %v", err)
	}
	if encryptedSecret == "" || encryptedSecret == "contact-secret" {
		t.Fatalf("expected encrypted secret at rest, got %q", encryptedSecret)
	}
	api.runSCRMWeComPermissionCheck(context.Background(), "req-db")
	var checks, exceptions int
	_ = api.db.QueryRow(`SELECT count(*) FROM wecom_permission_checks WHERE local_code = 'WECOM_PERMISSION_DENIED'`).Scan(&checks)
	_ = api.db.QueryRow(`SELECT count(*) FROM operation_exceptions WHERE exception_type = 'wecom_permission_error' AND status = 'pending'`).Scan(&exceptions)
	if checks == 0 || exceptions == 0 {
		t.Fatalf("expected recorded permission check and exception, checks=%d exceptions=%d", checks, exceptions)
	}
}

func TestExplainErrorPreservesBadRequestFallback(t *testing.T) {
	out := explainError(errors.New("plain failure"))
	if out.Code != "SYSTEM_ERROR" || out.Message == "" {
		t.Fatalf("unexpected plain error explanation: %#v", out)
	}
}
