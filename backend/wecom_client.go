package main

import (
	"context"
	"net/url"
)

type TokenResult struct {
	AccessToken string
	ExpiresIn   int
}

type WeComClient interface {
	GetAccessToken(ctx context.Context, corpID string, secret string) (TokenResult, error)
	GetDepartmentList(ctx context.Context, token string) error
	GetUserList(ctx context.Context, token string, departmentID string) error
	GetFollowUserList(ctx context.Context, token string) error
}

type realWeComClient struct{}

func (realWeComClient) GetAccessToken(ctx context.Context, corpID string, secret string) (TokenResult, error) {
	endpoint := wecomAPIBase + "/cgi-bin/gettoken?corpid=" + url.QueryEscape(corpID) + "&corpsecret=" + url.QueryEscape(secret)
	var resp struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := getWeComJSON(ctx, endpoint, &resp); err != nil {
		return TokenResult{}, err
	}
	if resp.ErrCode != 0 {
		return TokenResult{}, wecomAPIError("gettoken", resp.ErrCode, resp.ErrMsg)
	}
	return TokenResult{AccessToken: resp.AccessToken, ExpiresIn: resp.ExpiresIn}, nil
}

func (realWeComClient) GetDepartmentList(ctx context.Context, token string) error {
	endpoint := wecomAPIBase + "/cgi-bin/department/list?access_token=" + url.QueryEscape(token)
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := getWeComJSON(ctx, endpoint, &resp); err != nil {
		return err
	}
	if resp.ErrCode != 0 {
		return wecomAPIError("department_list", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}

func (realWeComClient) GetUserList(ctx context.Context, token string, departmentID string) error {
	endpoint := wecomAPIBase + "/cgi-bin/user/list?department_id=" + url.QueryEscape(firstNonEmpty(departmentID, "1")) + "&fetch_child=0&access_token=" + url.QueryEscape(token)
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := getWeComJSON(ctx, endpoint, &resp); err != nil {
		return err
	}
	if resp.ErrCode != 0 {
		return wecomAPIError("user_list", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}

func (realWeComClient) GetFollowUserList(ctx context.Context, token string) error {
	endpoint := wecomAPIBase + "/cgi-bin/externalcontact/get_follow_user_list?access_token=" + url.QueryEscape(token)
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := getWeComJSON(ctx, endpoint, &resp); err != nil {
		return err
	}
	if resp.ErrCode != 0 {
		return wecomAPIError("get_follow_user_list", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}
