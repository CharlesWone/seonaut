package models

import (
	"time"
)

type Project struct {
	Id                 int64
	URL                string
	Host               string
	IgnoreRobotsTxt    bool
	FollowNofollow     bool
	IncludeNoindex     bool
	Created            time.Time
	CrawlSitemap       bool
	AllowSubdomains    bool
	Deleting           bool
	BasicAuth          bool
	CheckExternalLinks bool
	Archive            bool
	UserAgent          string
	CronExpr           string
	DingtalkWebhookUrl string
	EncryptedId        string // 加密后的id
}
