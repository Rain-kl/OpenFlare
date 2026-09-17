// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/repository"
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	analyticsDays = 7

	denyingRobotsFile  = "User-Agent: *\nDisallow: /\n"
	allowingRobotsFile = "User-Agent: *\nAllow: /\n"
)

// RecentSystemLogs reads a page of the process log ring buffer.
func RecentSystemLogs(cursor, limit int) model.LogsResponse {
	entries, hasMore := logger.GlobalRingBuffer.Query(cursor, limit)

	resp := model.LogsResponse{
		Lines:   entries,
		HasMore: hasMore,
	}
	if len(entries) > 0 {
		resp.NextCursor = entries[0].Index
	}
	return resp
}

// RobotsTxtBody resolves the robots.txt payload from the indexing setting.
func RobotsTxtBody(ctx context.Context) string {
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeySearchEngineIndexingEnabled)
	if err == nil && enabled {
		return allowingRobotsFile
	}
	return denyingRobotsFile
}

// IsAllowedLogOrigin reports whether a WebSocket handshake origin may subscribe to logs.
// extraHosts are reverse-proxy hosts such as X-Forwarded-Host (the browser origin
// when Next.js rewrites /api to the backend).
func IsAllowedLogOrigin(ctx context.Context, origin, host string, extraHosts ...string) bool {
	if origin == "" {
		return true
	}

	u, err := url.Parse(origin)
	if err == nil {
		if originMatchesHost(u.Host, host, extraHosts...) {
			return true
		}
	}

	sc, cfgErr := repository.GetSystemConfigByKey(ctx, model.ConfigKeyServerAddress)
	if cfgErr != nil || sc.Value == "" {
		return false
	}
	originToCheck := strings.TrimRight(strings.TrimSpace(origin), "/")
	for _, allowed := range strings.Split(sc.Value, ",") {
		allowed = strings.TrimRight(strings.TrimSpace(allowed), "/")
		if allowed != "" && strings.EqualFold(allowed, originToCheck) {
			return true
		}
	}
	return false
}

func originMatchesHost(originHost, host string, extraHosts ...string) bool {
	if originHost == "" {
		return false
	}
	if hostMatches(originHost, host) {
		return true
	}
	for _, extra := range extraHosts {
		if hostMatches(originHost, extra) {
			return true
		}
	}
	return false
}

func hostMatches(originHost, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false
	}
	if strings.EqualFold(originHost, candidate) {
		return true
	}
	// Reverse-proxy / local Next rewrite: Origin is :3000, backend Host is :8000.
	return strings.EqualFold(hostName(originHost), hostName(candidate))
}

func hostName(hostport string) string {
	h, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return h
}

// AccessLogs queries the analytical access log store and decorates rows with user names.
func AccessLogs(ctx context.Context, q model.AccessLogQuery) (model.AccessLogsResponse, error) {
	rc := GetRiskControlService(ctx)
	if rc == nil {
		return model.AccessLogsResponse{}, errs.ErrLogStoreUnavailable
	}

	filter, err := buildAccessLogFilter(ctx, q)
	if err != nil {
		return model.AccessLogsResponse{}, err
	}
	if filter.UserIDs != nil && len(filter.UserIDs) == 0 {
		return model.AccessLogsResponse{Total: 0, List: []model.AccessLogItem{}}, nil
	}

	logs, total, err := rc.QueryAccessLogs(ctx, filter, q.Page, q.PageSize)
	if err != nil {
		return model.AccessLogsResponse{}, err
	}
	if total == 0 {
		return model.AccessLogsResponse{Total: 0, List: []model.AccessLogItem{}}, nil
	}

	list := make([]model.AccessLogItem, len(logs))
	for i, logItem := range logs {
		list[i] = model.AccessLogItem{
			ID:        logItem.ID,
			UserID:    logItem.UserID,
			Path:      logItem.Path,
			Method:    logItem.Method,
			IP:        logItem.IP,
			UserAgent: logItem.UserAgent,
			Status:    logItem.Status,
			Latency:   logItem.Latency,
			CreatedAt: logItem.CreatedAt.Format(time.RFC3339),
		}
	}
	enrichAccessLogsWithUsers(ctx, list)

	return model.AccessLogsResponse{Total: total, List: list}, nil
}

// AccessLogAnalytics aggregates the daily trend of the access log store.
func AccessLogAnalytics(ctx context.Context) (model.LogsAnalyticsResponse, error) {
	rc := GetRiskControlService(ctx)
	if rc == nil {
		return model.LogsAnalyticsResponse{}, errs.ErrLogStoreUnavailable
	}

	stats, err := rc.QueryAccessLogStats(ctx, analyticsDays)
	if err != nil {
		return model.LogsAnalyticsResponse{}, fmt.Errorf("%s%w", errs.ErrQueryAccessTrendFailed, err)
	}

	trendList := make([]model.TrendItem, len(stats))
	for i, st := range stats {
		trendList[i] = model.TrendItem{
			Date:  st.Date,
			Count: st.PV,
		}
	}

	return model.LogsAnalyticsResponse{
		Trend:    trendList,
		Browsers: []model.BrowserItem{},
		TopUsers: []model.TopUserItem{},
	}, nil
}

// findUserIDsByUsername resolves the user id filter behind a username search term.
func findUserIDsByUsername(ctx context.Context, username string) ([]uint64, error) {
	if userSvc := GetUserService(ctx); userSvc != nil {
		users, _, err := userSvc.ListUsers(ctx, 1, userQueryMaxLimit, username)
		if err != nil {
			return nil, fmt.Errorf(errs.ErrQueryUserFailed, err)
		}
		ids := make([]uint64, 0, len(users))
		for _, u := range users {
			ids = append(ids, u.ID)
		}
		return ids, nil
	}

	ids, err := repository.SearchUserIDsByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf(errs.ErrQueryUserFailed, err)
	}
	return ids, nil
}

const userQueryMaxLimit = 100

func buildAccessLogFilter(ctx context.Context, q model.AccessLogQuery) (contracts.AccessLogFilterDTO, error) {
	filter := contracts.AccessLogFilterDTO{}

	if q.Username != "" {
		userIDs, err := findUserIDsByUsername(ctx, q.Username)
		if err != nil {
			return filter, err
		}
		filter.UserIDs = userIDs
	}

	if q.Path != "" {
		filter.Path = q.Path
	}

	if q.StartTime != "" {
		if t, err := parseAccessLogTime(q.StartTime); err == nil {
			filter.StartTime = &t
		}
	}

	if q.EndTime != "" {
		if t, err := parseAccessLogTime(q.EndTime); err == nil {
			filter.EndTime = &t
		}
	}

	return filter, nil
}

func parseAccessLogTime(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", value)
}

// enrichAccessLogsWithUsers attaches usernames and nicknames to access log rows.
func enrichAccessLogsWithUsers(ctx context.Context, list []model.AccessLogItem) {
	if len(list) == 0 {
		return
	}

	userIDs := make([]uint64, 0, len(list))
	seen := make(map[uint64]struct{}, len(list))
	for _, item := range list {
		if _, ok := seen[item.UserID]; ok {
			continue
		}
		seen[item.UserID] = struct{}{}
		userIDs = append(userIDs, item.UserID)
	}

	userMap := make(map[uint64]repository.UserDisplayName, len(userIDs))
	if userSvc := GetUserService(ctx); userSvc != nil {
		if users, err := userSvc.GetUsersByIDs(ctx, userIDs); err == nil {
			for _, u := range users {
				if u != nil {
					userMap[u.ID] = repository.UserDisplayName{Username: u.Username, Nickname: u.Nickname}
				}
			}
		}
	} else if names, err := repository.LoadUserDisplayNames(ctx, userIDs); err == nil {
		userMap = names
	}

	for i := range list {
		if info, ok := userMap[list[i].UserID]; ok {
			list[i].Username = info.Username
			list[i].Nickname = info.Nickname
		}
	}
}
