package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

const wecomAPIBase = "https://qyapi.weixin.qq.com"

type secretCipher struct {
	key []byte
}

func newSecretCipher(rawKey string) secretCipher {
	seed := strings.TrimSpace(rawKey)
	if seed == "" {
		host, _ := os.Hostname()
		seed = "jianghu-scrm-local-wecom:" + host
		log.Printf("wecom secret encryption key uses local fallback; set SCRM_SECRET_ENCRYPTION_KEY for shared environments\n")
	}
	sum := sha256.Sum256([]byte(seed))
	return secretCipher{key: sum[:]}
}

func (c secretCipher) Encrypt(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	body := gcm.Seal(nil, nonce, []byte(value), nil)
	return "v1:" + base64.StdEncoding.EncodeToString(append(nonce, body...)), nil
}

func (c secretCipher) Decrypt(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	value = strings.TrimPrefix(value, "v1:")
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted value is too short")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

type WeComConfig struct {
	ID               string `json:"id"`
	CorpID           string `json:"corpId"`
	AgentID          string `json:"agentId"`
	CallbackURL      string `json:"callbackUrl"`
	TestDepartmentID string `json:"testDepartmentId"`
	Status           string `json:"status"`
	SecretConfigured bool   `json:"secretConfigured"`
	TokenConfigured  bool   `json:"tokenConfigured"`
	AESKeyConfigured bool   `json:"encodingAesKeyConfigured"`
	CreatedAt        string `json:"createdAt,omitempty"`
	UpdatedAt        string `json:"updatedAt,omitempty"`
	secretEncrypted  string
	token            string
	encodingAESKey   string
}

type WeComUser struct {
	ID             string `json:"id"`
	CorpID         string `json:"corpId"`
	UserID         string `json:"userid"`
	Name           string `json:"name"`
	DepartmentID   string `json:"departmentId"`
	DepartmentName string `json:"departmentName"`
	Mobile         string `json:"mobile"`
	Email          string `json:"email"`
	Avatar         string `json:"avatar"`
	Status         int    `json:"status"`
	RoleType       string `json:"roleType"`
	SyncedAt       string `json:"syncedAt"`
}

type WeComContactWay struct {
	ID            string   `json:"id"`
	CorpID        string   `json:"corpId"`
	ConfigID      string   `json:"configId"`
	Name          string   `json:"name"`
	QRCodeURL     string   `json:"qrCodeUrl"`
	Scene         string   `json:"scene"`
	State         string   `json:"state"`
	BoundUserIDs  []string `json:"boundUserids"`
	DepartmentID  string   `json:"departmentId"`
	Status        string   `json:"status"`
	CreatedBy     string   `json:"createdBy"`
	CreatedAt     string   `json:"createdAt"`
	CustomerCount int      `json:"customerCount,omitempty"`
}

type WeComCustomerEvent struct {
	ID             string          `json:"id"`
	CorpID         string          `json:"corpId"`
	ExternalUserID string          `json:"externalUserid"`
	FollowUserID   string          `json:"followUserid"`
	State          string          `json:"state"`
	ContactWayID   string          `json:"contactWayId"`
	EventType      string          `json:"eventType"`
	AddTime        string          `json:"addTime"`
	RawPayload     json.RawMessage `json:"rawPayload,omitempty"`
	CreatedAt      string          `json:"createdAt"`
}

func (api *API) wecomConfigHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: wecom pilot requires postgres mode", errBadRequest)
	}
	switch r.Method {
	case http.MethodGet:
		cfg, err := api.getWeComConfig(r.Context(), false)
		if err != nil {
			if errorsIsSQLNoRows(err) {
				return writeJSON(w, map[string]any{"configured": false})
			}
			return err
		}
		return writeJSON(w, map[string]any{"configured": true, "config": cfg})
	case http.MethodPost:
		var payload struct {
			CorpID           string `json:"corpId"`
			AgentID          string `json:"agentId"`
			Secret           string `json:"secret"`
			Token            string `json:"token"`
			EncodingAESKey   string `json:"encodingAesKey"`
			CallbackURL      string `json:"callbackUrl"`
			TestDepartmentID string `json:"testDepartmentId"`
		}
		if err := decode(r, &payload); err != nil {
			return err
		}
		corpID := strings.TrimSpace(payload.CorpID)
		if corpID == "" {
			return fmt.Errorf("%w: corpId is required", errBadRequest)
		}
		existing, _ := api.getWeComConfig(r.Context(), true)
		secretEncrypted := existing.secretEncrypted
		if strings.TrimSpace(payload.Secret) != "" {
			encrypted, err := api.secretCipher.Encrypt(payload.Secret)
			if err != nil {
				return err
			}
			secretEncrypted = encrypted
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		id := firstNonEmpty(existing.ID, newID("wcc"))
		_, err := api.db.ExecContext(ctx, `
			INSERT INTO wecom_corp_config (
				id, corp_id, agent_id, secret_encrypted, token, encoding_aes_key, callback_url, test_department_id, status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'configured')
			ON CONFLICT (corp_id) DO UPDATE SET
				agent_id = EXCLUDED.agent_id,
				secret_encrypted = CASE WHEN EXCLUDED.secret_encrypted <> '' THEN EXCLUDED.secret_encrypted ELSE wecom_corp_config.secret_encrypted END,
				token = EXCLUDED.token,
				encoding_aes_key = EXCLUDED.encoding_aes_key,
				callback_url = EXCLUDED.callback_url,
				test_department_id = EXCLUDED.test_department_id,
				status = 'configured',
				updated_at = now()
		`, id, corpID, strings.TrimSpace(payload.AgentID), secretEncrypted, strings.TrimSpace(payload.Token), strings.TrimSpace(payload.EncodingAESKey), strings.TrimSpace(payload.CallbackURL), strings.TrimSpace(payload.TestDepartmentID))
		if err != nil {
			return err
		}
		log.Printf("wecom config saved corp_id=%s department_id=%s\n", corpID, strings.TrimSpace(payload.TestDepartmentID))
		cfg, err := api.getWeComConfig(ctx, false)
		if err != nil {
			return err
		}
		return writeJSON(w, map[string]any{"status": "ok", "config": cfg})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
}

func (api *API) wecomTestConnectionHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	token, err := api.wecomAccessToken(r.Context(), true)
	if err != nil {
		log.Printf("wecom test connection failed: %v\n", err)
		return fmt.Errorf("%w: %v", errBadRequest, err)
	}
	if token == "" {
		return fmt.Errorf("%w: empty access_token", errBadRequest)
	}
	log.Printf("wecom test connection ok\n")
	return writeJSON(w, map[string]any{"status": "ok"})
}

func (api *API) wecomUsersHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: wecom pilot requires postgres mode", errBadRequest)
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/sync") {
		return api.syncWeComUsers(w, r)
	}
	if r.URL.Path != "/api/wecom/users" {
		return errNotFound
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	cfg, err := api.getWeComConfig(r.Context(), false)
	if err != nil {
		return err
	}
	where := []string{"corp_id = $1"}
	args := []any{cfg.CorpID}
	if role := strings.TrimSpace(r.URL.Query().Get("role_type")); role != "" && role != "all" {
		where = append(where, "role_type = $"+strconv.Itoa(len(args)+1))
		args = append(args, role)
	}
	if dept := strings.TrimSpace(r.URL.Query().Get("department_id")); dept != "" {
		where = append(where, "department_id = $"+strconv.Itoa(len(args)+1))
		args = append(args, dept)
	}
	if keyword := strings.TrimSpace(r.URL.Query().Get("keyword")); keyword != "" {
		where = append(where, "(name ILIKE $"+strconv.Itoa(len(args)+1)+" OR userid ILIKE $"+strconv.Itoa(len(args)+1)+")")
		args = append(args, "%"+keyword+"%")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, corp_id, userid, name, department_id, department_name, mobile, email, avatar, status, role_type,
			to_char(synced_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM wecom_users
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY role_type, name, userid
	`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	users := []WeComUser{}
	for rows.Next() {
		var user WeComUser
		if err := rows.Scan(&user.ID, &user.CorpID, &user.UserID, &user.Name, &user.DepartmentID, &user.DepartmentName, &user.Mobile, &user.Email, &user.Avatar, &user.Status, &user.RoleType, &user.SyncedAt); err != nil {
			return err
		}
		users = append(users, user)
	}
	return writeJSON(w, users)
}

func (api *API) wecomUserActionHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: wecom pilot requires postgres mode", errBadRequest)
	}
	if r.Method == http.MethodPost && strings.Trim(r.URL.Path, "/") == "api/wecom/users/sync" {
		return api.syncWeComUsers(w, r)
	}
	if r.Method != http.MethodPatch || !strings.HasSuffix(r.URL.Path, "/role") {
		return errNotFound
	}
	userID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/wecom/users/"), "/role")
	userID, _ = url.PathUnescape(strings.Trim(userID, "/"))
	if userID == "" {
		return fmt.Errorf("%w: userid is required", errBadRequest)
	}
	var payload struct {
		RoleType string `json:"roleType"`
	}
	if err := decode(r, &payload); err != nil {
		return err
	}
	roleType := strings.TrimSpace(payload.RoleType)
	if !validWeComRole(roleType) {
		return fmt.Errorf("%w: invalid roleType", errBadRequest)
	}
	cfg, err := api.getWeComConfig(r.Context(), false)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	result, err := api.db.ExecContext(ctx, `UPDATE wecom_users SET role_type = $1, updated_at = now() WHERE corp_id = $2 AND userid = $3`, roleType, cfg.CorpID, userID)
	if err != nil {
		return err
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		return errNotFound
	}
	log.Printf("wecom user role updated corp_id=%s userid=%s role=%s\n", cfg.CorpID, userID, roleType)
	user, err := api.getWeComUser(ctx, cfg.CorpID, userID)
	if err != nil {
		return err
	}
	return writeJSON(w, user)
}

func (api *API) syncWeComUsers(w http.ResponseWriter, r *http.Request) error {
	cfg, err := api.getWeComConfig(r.Context(), true)
	if err != nil {
		return err
	}
	departmentID := firstNonEmpty(cfg.TestDepartmentID, "1")
	token, err := api.wecomAccessToken(r.Context(), false)
	if err != nil {
		log.Printf("wecom sync users token failed corp_id=%s err=%v\n", cfg.CorpID, err)
		return fmt.Errorf("%w: %v", errBadRequest, err)
	}
	departmentName := api.wecomDepartmentName(r.Context(), token, departmentID)
	endpoint := wecomAPIBase + "/cgi-bin/user/list?department_id=" + url.QueryEscape(departmentID) + "&fetch_child=0&access_token=" + url.QueryEscape(token)
	var resp struct {
		ErrCode  int    `json:"errcode"`
		ErrMsg   string `json:"errmsg"`
		UserList []struct {
			UserID     string  `json:"userid"`
			Name       string  `json:"name"`
			Department []int64 `json:"department"`
			Mobile     string  `json:"mobile"`
			Email      string  `json:"email"`
			Avatar     string  `json:"avatar"`
			Status     int     `json:"status"`
		} `json:"userlist"`
	}
	if err := getWeComJSON(r.Context(), endpoint, &resp); err != nil {
		log.Printf("wecom sync users request failed corp_id=%s department_id=%s err=%v\n", cfg.CorpID, cfg.TestDepartmentID, err)
		return fmt.Errorf("%w: %v", errBadRequest, err)
	}
	if resp.ErrCode != 0 {
		if resp.ErrCode == 60011 || resp.ErrCode == 60020 {
			return api.syncWeComFollowUsers(w, r, cfg, token, resp.ErrCode, resp.ErrMsg)
		}
		log.Printf("wecom sync users api failed corp_id=%s department_id=%s errcode=%d errmsg=%s\n", cfg.CorpID, cfg.TestDepartmentID, resp.ErrCode, resp.ErrMsg)
		return fmt.Errorf("%w: wecom errcode=%d errmsg=%s", errBadRequest, resp.ErrCode, resp.ErrMsg)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	added, updated, failed := 0, 0, 0
	for _, item := range resp.UserList {
		deptID := departmentID
		if len(item.Department) > 0 {
			deptID = strconv.FormatInt(item.Department[0], 10)
		}
		result, err := api.db.ExecContext(ctx, `
			INSERT INTO wecom_users (id, corp_id, userid, name, department_id, department_name, mobile, email, avatar, status, role_type, synced_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'unknown', now())
			ON CONFLICT (corp_id, userid) DO UPDATE SET
				name = EXCLUDED.name,
				department_id = EXCLUDED.department_id,
				department_name = EXCLUDED.department_name,
				mobile = EXCLUDED.mobile,
				email = EXCLUDED.email,
				avatar = EXCLUDED.avatar,
				status = EXCLUDED.status,
				synced_at = now(),
				updated_at = now()
		`, newID("wcu"), cfg.CorpID, item.UserID, item.Name, deptID, departmentName, item.Mobile, item.Email, item.Avatar, item.Status)
		if err != nil {
			failed++
			log.Printf("wecom upsert user failed corp_id=%s userid=%s err=%v\n", cfg.CorpID, item.UserID, err)
			continue
		}
		if count, _ := result.RowsAffected(); count == 1 {
			added++
		} else {
			updated++
		}
	}
	log.Printf("wecom users synced corp_id=%s department_id=%s added=%d updated=%d failed=%d\n", cfg.CorpID, departmentID, added, updated, failed)
	return writeJSON(w, map[string]any{"added": added, "updated": updated, "failed": failed, "total": len(resp.UserList), "departmentId": departmentID})
}

func (api *API) syncWeComFollowUsers(w http.ResponseWriter, r *http.Request, cfg WeComConfig, token string, fallbackCode int, fallbackMsg string) error {
	endpoint := wecomAPIBase + "/cgi-bin/externalcontact/get_follow_user_list?access_token=" + url.QueryEscape(token)
	var resp struct {
		ErrCode    int      `json:"errcode"`
		ErrMsg     string   `json:"errmsg"`
		FollowUser []string `json:"follow_user"`
	}
	if err := getWeComJSON(r.Context(), endpoint, &resp); err != nil {
		log.Printf("wecom sync follow users request failed corp_id=%s err=%v\n", cfg.CorpID, err)
		return fmt.Errorf("%w: %v", errBadRequest, err)
	}
	if resp.ErrCode != 0 {
		log.Printf("wecom sync follow users api failed corp_id=%s errcode=%d errmsg=%s\n", cfg.CorpID, resp.ErrCode, resp.ErrMsg)
		return fmt.Errorf("%w: wecom errcode=%d errmsg=%s", errBadRequest, resp.ErrCode, resp.ErrMsg)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	added, updated, failed := 0, 0, 0
	for _, userID := range resp.FollowUser {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		existed := wecomUserExists(ctx, api.db, cfg.CorpID, userID)
		_, err := api.db.ExecContext(ctx, `
			INSERT INTO wecom_users (
				id, corp_id, userid, name, department_id, department_name, mobile, email, avatar, status, role_type, synced_at
			) VALUES ($1, $2, $3, $3, 'externalcontact', '客户联系成员', '', '', '', 1, 'guide', now())
			ON CONFLICT (corp_id, userid) DO UPDATE SET
				name = CASE WHEN wecom_users.name = '' OR wecom_users.name = wecom_users.userid THEN EXCLUDED.name ELSE wecom_users.name END,
				department_id = EXCLUDED.department_id,
				department_name = EXCLUDED.department_name,
				status = EXCLUDED.status,
				role_type = CASE
					WHEN wecom_users.role_type IN ('sales', 'guide', 'admin') THEN wecom_users.role_type
					ELSE EXCLUDED.role_type
				END,
				synced_at = now(),
				updated_at = now()
		`, newID("wcu"), cfg.CorpID, userID)
		if err != nil {
			failed++
			log.Printf("wecom upsert follow user failed corp_id=%s userid=%s err=%v\n", cfg.CorpID, userID, err)
			continue
		}
		if existed {
			updated++
		} else {
			added++
		}
	}
	log.Printf("wecom follow users synced corp_id=%s added=%d updated=%d failed=%d fallback_errcode=%d\n", cfg.CorpID, added, updated, failed, fallbackCode)
	return writeJSON(w, map[string]any{
		"added":               added,
		"updated":             updated,
		"failed":              failed,
		"total":               len(resp.FollowUser),
		"source":              "externalcontact.get_follow_user_list",
		"fallbackFromErrcode": fallbackCode,
		"fallbackFromErrmsg":  fallbackMsg,
	})
}

func wecomUserExists(ctx context.Context, db *sql.DB, corpID, userID string) bool {
	var marker int
	err := db.QueryRowContext(ctx, `SELECT 1 FROM wecom_users WHERE corp_id = $1 AND userid = $2`, corpID, userID).Scan(&marker)
	return err == nil
}

func (api *API) wecomContactWayHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: wecom pilot requires postgres mode", errBadRequest)
	}
	if r.URL.Path != "/api/wecom/contact-way" {
		return errNotFound
	}
	switch r.Method {
	case http.MethodGet:
		return api.listWeComContactWays(w, r)
	case http.MethodPost:
		return fmt.Errorf("%w: creating new Enterprise WeChat contact ways is disabled; bind an existing config_id at /api/scrm/contact-way-bindings", errBadRequest)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
}

func (api *API) createWeComContactWay(w http.ResponseWriter, r *http.Request) error {
	return fmt.Errorf("%w: creating new Enterprise WeChat contact ways is disabled; bind an existing config_id at /api/scrm/contact-way-bindings", errBadRequest)
}

func (api *API) listWeComContactWays(w http.ResponseWriter, r *http.Request) error {
	cfg, err := api.getWeComConfig(r.Context(), false)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	rows, err := api.db.QueryContext(ctx, `
		SELECT cw.id, cw.corp_id, cw.config_id, cw.name, cw.qr_code_url, cw.scene, cw.state,
			cw.bound_userids::text, cw.department_id, cw.status, cw.created_by,
			to_char(cw.created_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'),
			count(ev.id) AS customer_count
		FROM wecom_contact_ways cw
		LEFT JOIN wecom_customer_events ev ON ev.corp_id = cw.corp_id AND ev.contact_way_id = cw.id
		WHERE cw.corp_id = $1
		GROUP BY cw.id
		ORDER BY cw.created_at DESC
	`, cfg.CorpID)
	if err != nil {
		return err
	}
	defer rows.Close()
	ways := []WeComContactWay{}
	for rows.Next() {
		way, err := scanWeComContactWay(rows)
		if err != nil {
			return err
		}
		ways = append(ways, way)
	}
	return writeJSON(w, ways)
}

func (api *API) wecomContactWayActionHandler(w http.ResponseWriter, r *http.Request) error {
	if api.db == nil {
		return fmt.Errorf("%w: wecom pilot requires postgres mode", errBadRequest)
	}
	if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/stats") {
		return errNotFound
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/wecom/contact-way/"), "/stats")
	id = strings.Trim(id, "/")
	cfg, err := api.getWeComConfig(r.Context(), false)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	way, err := api.getWeComContactWay(ctx, cfg.CorpID, id)
	if err != nil {
		return err
	}
	users, err := api.wecomUsersByIDs(ctx, cfg.CorpID, way.BoundUserIDs)
	if err != nil {
		return err
	}
	recent, err := api.recentWeComCustomerEvents(ctx, cfg.CorpID, way.ID)
	if err != nil {
		return err
	}
	byUser, err := api.countWeComEventsBy(ctx, cfg.CorpID, way.ID, "follow_userid")
	if err != nil {
		return err
	}
	byDate, err := api.countWeComEventsBy(ctx, cfg.CorpID, way.ID, "to_char(COALESCE(add_time, created_at), 'YYYY-MM-DD')")
	if err != nil {
		return err
	}
	return writeJSON(w, map[string]any{
		"contactWay":      way,
		"boundUsers":      users,
		"customerCount":   way.CustomerCount,
		"recentCustomers": recent,
		"byFollowUserid":  byUser,
		"byDate":          byDate,
	})
}

func (api *API) wecomCallbackHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if api.db == nil {
		http.Error(w, "wecom pilot requires postgres mode", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		api.verifyWeComCallback(w, r)
	case http.MethodPost:
		api.receiveWeComCallback(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (api *API) verifyWeComCallback(w http.ResponseWriter, r *http.Request) {
	cfg, err := api.getWeComConfig(r.Context(), true)
	if err != nil {
		http.Error(w, "config not found", http.StatusBadRequest)
		return
	}
	plain, err := decryptWeComPayload(cfg, r.URL.Query().Get("msg_signature"), r.URL.Query().Get("timestamp"), r.URL.Query().Get("nonce"), r.URL.Query().Get("echostr"))
	if err != nil {
		log.Printf("wecom callback verify failed corp_id=%s err=%v\n", cfg.CorpID, err)
		http.Error(w, "verify failed", http.StatusForbidden)
		return
	}
	log.Printf("wecom callback verify ok corp_id=%s\n", cfg.CorpID)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(plain))
}

func (api *API) receiveWeComCallback(w http.ResponseWriter, r *http.Request) {
	cfg, err := api.getWeComConfig(r.Context(), true)
	if err != nil {
		http.Error(w, "config not found", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	var encrypted struct {
		Encrypt string `xml:"Encrypt"`
	}
	plain := body
	if err := xml.Unmarshal(body, &encrypted); err == nil && encrypted.Encrypt != "" {
		plainText, err := decryptWeComPayload(cfg, r.URL.Query().Get("msg_signature"), r.URL.Query().Get("timestamp"), r.URL.Query().Get("nonce"), encrypted.Encrypt)
		if err != nil {
			log.Printf("wecom callback decrypt failed corp_id=%s err=%v\n", cfg.CorpID, err)
			http.Error(w, "decrypt failed", http.StatusForbidden)
			return
		}
		plain = []byte(plainText)
	}
	if err := api.saveWeComCustomerEvent(r.Context(), cfg.CorpID, plain); err != nil {
		log.Printf("wecom callback save event failed corp_id=%s err=%v\n", cfg.CorpID, err)
		http.Error(w, "save failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("success"))
}

func (api *API) getWeComConfig(ctx context.Context, includeSecrets bool) (WeComConfig, error) {
	if api.db == nil {
		return WeComConfig{}, sql.ErrNoRows
	}
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var cfg WeComConfig
	err := api.db.QueryRowContext(queryCtx, `
		SELECT id, corp_id, agent_id, secret_encrypted, token, encoding_aes_key, callback_url, test_department_id, status,
			to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'),
			to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM wecom_corp_config
		ORDER BY updated_at DESC
		LIMIT 1
	`).Scan(&cfg.ID, &cfg.CorpID, &cfg.AgentID, &cfg.secretEncrypted, &cfg.token, &cfg.encodingAESKey, &cfg.CallbackURL, &cfg.TestDepartmentID, &cfg.Status, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return WeComConfig{}, err
	}
	cfg.SecretConfigured = cfg.secretEncrypted != ""
	cfg.TokenConfigured = cfg.token != ""
	cfg.AESKeyConfigured = cfg.encodingAESKey != ""
	if !includeSecrets {
		cfg.secretEncrypted = ""
		cfg.token = ""
		cfg.encodingAESKey = ""
	}
	return cfg, nil
}

func (api *API) wecomAccessToken(ctx context.Context, force bool) (string, error) {
	cfg, err := api.getWeComConfig(ctx, true)
	if err != nil {
		return "", err
	}
	if !force {
		var token string
		var expiresAt time.Time
		err := api.db.QueryRowContext(ctx, `SELECT access_token, expires_at FROM wecom_access_tokens WHERE corp_id = $1`, cfg.CorpID).Scan(&token, &expiresAt)
		if err == nil && token != "" && time.Until(expiresAt) > 5*time.Minute {
			return token, nil
		}
		if err != nil && err != sql.ErrNoRows {
			return "", err
		}
	}
	secret, err := api.secretCipher.Decrypt(cfg.secretEncrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt secret failed")
	}
	if cfg.CorpID == "" || secret == "" {
		return "", fmt.Errorf("corpId or secret is not configured")
	}
	endpoint := wecomAPIBase + "/cgi-bin/gettoken?corpid=" + url.QueryEscape(cfg.CorpID) + "&corpsecret=" + url.QueryEscape(secret)
	var resp struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := getWeComJSON(ctx, endpoint, &resp); err != nil {
		log.Printf("wecom access token request failed corp_id=%s err=%v\n", cfg.CorpID, err)
		return "", err
	}
	if resp.ErrCode != 0 {
		log.Printf("wecom access token api failed corp_id=%s errcode=%d errmsg=%s\n", cfg.CorpID, resp.ErrCode, resp.ErrMsg)
		return "", fmt.Errorf("wecom errcode=%d errmsg=%s", resp.ErrCode, resp.ErrMsg)
	}
	expiresAt := time.Now().Add(time.Duration(max(60, resp.ExpiresIn-300)) * time.Second)
	_, err = api.db.ExecContext(ctx, `
		INSERT INTO wecom_access_tokens (corp_id, access_token, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (corp_id) DO UPDATE SET access_token = EXCLUDED.access_token, expires_at = EXCLUDED.expires_at, updated_at = now()
	`, cfg.CorpID, resp.AccessToken, expiresAt)
	if err != nil {
		return "", err
	}
	log.Printf("wecom access token refreshed corp_id=%s expires_at=%s\n", cfg.CorpID, expiresAt.Format(time.RFC3339))
	return resp.AccessToken, nil
}

func (api *API) wecomDepartmentName(ctx context.Context, accessToken, departmentID string) string {
	endpoint := wecomAPIBase + "/cgi-bin/department/get?id=" + url.QueryEscape(departmentID) + "&access_token=" + url.QueryEscape(accessToken)
	var resp struct {
		ErrCode    int    `json:"errcode"`
		ErrMsg     string `json:"errmsg"`
		Department struct {
			Name string `json:"name"`
		} `json:"department"`
	}
	if err := getWeComJSON(ctx, endpoint, &resp); err != nil || resp.ErrCode != 0 {
		if err != nil {
			log.Printf("wecom department name request failed department_id=%s err=%v\n", departmentID, err)
		} else {
			log.Printf("wecom department name api failed department_id=%s errcode=%d errmsg=%s\n", departmentID, resp.ErrCode, resp.ErrMsg)
		}
		return departmentID
	}
	return firstNonEmpty(resp.Department.Name, departmentID)
}

func (api *API) validateWeComContactWayUsers(ctx context.Context, corpID string, userIDs []string) ([]string, error) {
	clean := []string{}
	seen := map[string]bool{}
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID != "" && !seen[userID] {
			clean = append(clean, userID)
			seen[userID] = true
		}
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("%w: userids are required", errBadRequest)
	}
	rows, err := api.db.QueryContext(ctx, `
		SELECT userid, role_type
		FROM wecom_users
		WHERE corp_id = $1 AND userid = ANY($2)
	`, corpID, pq.Array(clean))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roles := map[string]string{}
	for rows.Next() {
		var userID, role string
		if err := rows.Scan(&userID, &role); err != nil {
			return nil, err
		}
		roles[userID] = role
	}
	for _, userID := range clean {
		role, ok := roles[userID]
		if !ok {
			return nil, fmt.Errorf("%w: userid %s is not synced", errBadRequest, userID)
		}
		if role != "sales" && role != "guide" {
			return nil, fmt.Errorf("%w: userid %s must be sales or guide", errBadRequest, userID)
		}
	}
	return clean, nil
}

func (api *API) getWeComUser(ctx context.Context, corpID, userID string) (WeComUser, error) {
	var user WeComUser
	err := api.db.QueryRowContext(ctx, `
		SELECT id, corp_id, userid, name, department_id, department_name, mobile, email, avatar, status, role_type,
			to_char(synced_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM wecom_users WHERE corp_id = $1 AND userid = $2
	`, corpID, userID).Scan(&user.ID, &user.CorpID, &user.UserID, &user.Name, &user.DepartmentID, &user.DepartmentName, &user.Mobile, &user.Email, &user.Avatar, &user.Status, &user.RoleType, &user.SyncedAt)
	return user, err
}

func (api *API) getWeComContactWay(ctx context.Context, corpID, id string) (WeComContactWay, error) {
	row := api.db.QueryRowContext(ctx, `
		SELECT cw.id, cw.corp_id, cw.config_id, cw.name, cw.qr_code_url, cw.scene, cw.state,
			cw.bound_userids::text, cw.department_id, cw.status, cw.created_by,
			to_char(cw.created_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'),
			count(ev.id) AS customer_count
		FROM wecom_contact_ways cw
		LEFT JOIN wecom_customer_events ev ON ev.corp_id = cw.corp_id AND ev.contact_way_id = cw.id
		WHERE cw.corp_id = $1 AND cw.id = $2
		GROUP BY cw.id
	`, corpID, id)
	return scanWeComContactWay(row)
}

func (api *API) wecomUsersByIDs(ctx context.Context, corpID string, userIDs []string) ([]WeComUser, error) {
	if len(userIDs) == 0 {
		return []WeComUser{}, nil
	}
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, corp_id, userid, name, department_id, department_name, mobile, email, avatar, status, role_type,
			to_char(synced_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM wecom_users
		WHERE corp_id = $1 AND userid = ANY($2)
		ORDER BY name, userid
	`, corpID, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []WeComUser{}
	for rows.Next() {
		var user WeComUser
		if err := rows.Scan(&user.ID, &user.CorpID, &user.UserID, &user.Name, &user.DepartmentID, &user.DepartmentName, &user.Mobile, &user.Email, &user.Avatar, &user.Status, &user.RoleType, &user.SyncedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (api *API) recentWeComCustomerEvents(ctx context.Context, corpID, contactWayID string) ([]WeComCustomerEvent, error) {
	rows, err := api.db.QueryContext(ctx, `
		SELECT id, corp_id, external_userid, follow_userid, state, contact_way_id, event_type,
			COALESCE(to_char(add_time, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM'), ''),
			raw_payload::text,
			to_char(created_at, 'YYYY-MM-DD"T"HH24:MI:SSTZH:TZM')
		FROM wecom_customer_events
		WHERE corp_id = $1 AND contact_way_id = $2
		ORDER BY COALESCE(add_time, created_at) DESC
		LIMIT 20
	`, corpID, contactWayID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []WeComCustomerEvent{}
	for rows.Next() {
		var event WeComCustomerEvent
		var raw string
		if err := rows.Scan(&event.ID, &event.CorpID, &event.ExternalUserID, &event.FollowUserID, &event.State, &event.ContactWayID, &event.EventType, &event.AddTime, &raw, &event.CreatedAt); err != nil {
			return nil, err
		}
		event.RawPayload = json.RawMessage(raw)
		events = append(events, event)
	}
	return events, nil
}

func (api *API) countWeComEventsBy(ctx context.Context, corpID, contactWayID, expr string) ([]map[string]any, error) {
	rows, err := api.db.QueryContext(ctx, `
		SELECT `+expr+` AS key, count(*)
		FROM wecom_customer_events
		WHERE corp_id = $1 AND contact_way_id = $2
		GROUP BY key
		ORDER BY key
	`, corpID, contactWayID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []map[string]any{}
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"key": key, "count": count})
	}
	return result, nil
}

func (api *API) saveWeComCustomerEvent(ctx context.Context, corpID string, plainXML []byte) error {
	trimmed := bytes.TrimSpace(plainXML)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return api.handleWeComCustomerEventJSON(ctx, corpID, trimmed)
	}
	return api.handleWeComCustomerEventXML(ctx, corpID, trimmed)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanWeComContactWay(row scanner) (WeComContactWay, error) {
	var way WeComContactWay
	var bound string
	if err := row.Scan(&way.ID, &way.CorpID, &way.ConfigID, &way.Name, &way.QRCodeURL, &way.Scene, &way.State, &bound, &way.DepartmentID, &way.Status, &way.CreatedBy, &way.CreatedAt, &way.CustomerCount); err != nil {
		return WeComContactWay{}, err
	}
	way.BoundUserIDs = stringsFromJSONArray(bound)
	return way, nil
}

func validWeComRole(role string) bool {
	return role == "sales" || role == "guide" || role == "admin" || role == "unknown"
}

func getWeComJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	client := wecomHTTPClient()
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("wecom http status=%d", res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(target)
}

func postWeComJSON(ctx context.Context, endpoint string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := wecomHTTPClient()
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("wecom http status=%d", res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(target)
}

func wecomHTTPClient() *http.Client {
	forwardAddr := strings.TrimSpace(os.Getenv("WECOM_API_FORWARD_ADDR"))
	if forwardAddr == "" {
		return &http.Client{Timeout: 8 * time.Second}
	}
	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.EqualFold(addr, "qyapi.weixin.qq.com:443") {
				return dialer.DialContext(ctx, network, forwardAddr)
			}
			return dialer.DialContext(ctx, network, addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   8 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: transport}
}

func decryptWeComPayload(cfg WeComConfig, signature, timestamp, nonce, encrypted string) (string, error) {
	if cfg.token == "" || cfg.encodingAESKey == "" {
		return "", fmt.Errorf("token or encoding aes key is not configured")
	}
	if wecomSignature(cfg.token, timestamp, nonce, encrypted) != signature {
		return "", fmt.Errorf("invalid signature")
	}
	keyText := cfg.encodingAESKey
	if len(keyText) != 43 {
		return "", fmt.Errorf("encoding aes key length must be 43")
	}
	key, err := base64.StdEncoding.DecodeString(keyText + "=")
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid ciphertext size")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, key[:aes.BlockSize]).CryptBlocks(plain, ciphertext)
	plain, err = pkcs7Unpad(plain, aes.BlockSize)
	if err != nil {
		return "", err
	}
	if len(plain) < 20 {
		return "", fmt.Errorf("invalid plaintext size")
	}
	msgLen := int(binary.BigEndian.Uint32(plain[16:20]))
	if len(plain) < 20+msgLen {
		return "", fmt.Errorf("invalid message length")
	}
	msg := plain[20 : 20+msgLen]
	receiveID := string(plain[20+msgLen:])
	if receiveID != "" && receiveID != cfg.CorpID {
		return "", fmt.Errorf("receive id mismatch")
	}
	return string(msg), nil
}

func wecomSignature(token, timestamp, nonce, encrypted string) string {
	parts := []string{token, timestamp, nonce, encrypted}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(sum[:])
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("invalid pkcs7 data")
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > blockSize || pad > len(data) {
		return nil, fmt.Errorf("invalid pkcs7 padding")
	}
	for _, b := range data[len(data)-pad:] {
		if int(b) != pad {
			return nil, fmt.Errorf("invalid pkcs7 padding")
		}
	}
	return data[:len(data)-pad], nil
}

func errorsIsSQLNoRows(err error) bool {
	return err == sql.ErrNoRows
}
