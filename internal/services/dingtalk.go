package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/stjudewashere/seonaut/internal/config"
	"github.com/stjudewashere/seonaut/internal/models"
)

// DingTalkService handles sending notifications to DingTalk webhook
type DingTalkService struct {
	config *config.DingTalkConfig
	client *http.Client
}

// DingTalkMessage represents the message structure for DingTalk webhook
type DingTalkMessage struct {
	MsgType  string           `json:"msgtype"`
	Markdown DingTalkMarkdown `json:"markdown"`
	At       DingTalkAt       `json:"at"`
}

type DingTalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type DingTalkAt struct {
	AtMobiles []string `json:"atMobiles,omitempty"`
	AtUserIds []string `json:"atUserIds,omitempty"`
	IsAtAll   bool     `json:"isAtAll,omitempty"`
}

// NewDingTalkService creates a new DingTalk service instance
func NewDingTalkService(config *config.DingTalkConfig) *DingTalkService {
	return &DingTalkService{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendCrawlReport sends crawl completion report to DingTalk
func (s *DingTalkService) SendCrawlReport(crawl *models.Crawl, project *models.Project) error {
	if !s.config.Enabled || s.config.WebhookURL == "" {
		return nil
	}

	markdownText := s.formatCrawlReport(crawl, project)

	message := DingTalkMessage{
		MsgType: "markdown",
		Markdown: DingTalkMarkdown{
			Title: fmt.Sprintf("SEO审计报告 - %s", project.URL),
			Text:  markdownText,
		},
		At: DingTalkAt{
			IsAtAll: false,
		},
	}

	return s.sendMessage(message)
}

// formatCrawlReport formats the crawl data into Markdown
func (s *DingTalkService) formatCrawlReport(crawl *models.Crawl, project *models.Project) string {
	duration := crawl.End.Sub(crawl.Start)

	var statusEmoji string
	if crawl.CriticalIssues > 0 {
		statusEmoji = "🔴"
	} else if crawl.AlertIssues > 0 {
		statusEmoji = "🟡"
	} else if crawl.WarningIssues > 0 {
		statusEmoji = "🟠"
	} else {
		statusEmoji = "🟢"
	}

	text := fmt.Sprintf(`## %s SEO审计报告完成

**网站：** %s
**爬取时间：** %s
**爬取耗时：** %s

### 📊 爬取统计
- **总URL数：** %d
- **被robots.txt阻止：** %d
- **Noindex页面：** %d

### ⚠️ 问题统计
- **🔴 严重问题：** %d
- **🟡 警告问题：** %d  
- **🟠 提示问题：** %d
- **总问题数：** %d

### 🔗 链接统计
- **内部Follow链接：** %d
- **内部NoFollow链接：** %d
- **外部Follow链接：** %d
- **外部NoFollow链接：** %d
- **赞助链接：** %d
- **UGC链接：** %d

### 🤖 技术信息
- **Robots.txt存在：** %s
- **Sitemap存在：** %s
- **Sitemap被阻止：** %s

---
*报告生成时间：%s*`,
		statusEmoji,
		project.URL,
		crawl.Start.Format("2006-01-02 15:04:05"),
		formatDuration(duration),
		crawl.TotalURLs,
		crawl.BlockedByRobotstxt,
		crawl.Noindex,
		crawl.CriticalIssues,
		crawl.AlertIssues,
		crawl.WarningIssues,
		crawl.TotalIssues,
		crawl.InternalFollowLinks,
		crawl.InternalNoFollowLinks,
		crawl.ExternalFollowLinks,
		crawl.ExternalNoFollowLinks,
		crawl.SponsoredLinks,
		crawl.UGCLinks,
		formatBool(crawl.RobotstxtExists),
		formatBool(crawl.SitemapExists),
		formatBool(crawl.SitemapIsBlocked),
		time.Now().Format("2006-01-02 15:04:05"),
	)

	return text
}

// sendMessage sends the message to DingTalk webhook
func (s *DingTalkService) sendMessage(message DingTalkMessage) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequest("POST", s.config.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add signature if secret is configured
	if s.config.Secret != "" {
		timestamp := strconv.FormatInt(time.Now().UnixNano()/1e6, 10)
		signature := s.generateSignature(s.config.Secret, timestamp)

		req.Header.Set("timestamp", timestamp)
		req.Header.Set("sign", signature)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DingTalk API returned status %d", resp.StatusCode)
	}

	log.Printf("DingTalk notification sent successfully for project %s", message.Markdown.Title)
	return nil
}

// generateSignature generates HMAC-SHA256 signature for DingTalk webhook
func (s *DingTalkService) generateSignature(secret, timestamp string) string {
	stringToSign := timestamp + "\n" + secret
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// formatDuration formats duration to human readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f秒", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1f分钟", d.Minutes())
	} else {
		return fmt.Sprintf("%.1f小时", d.Hours())
	}
}

// formatBool formats boolean to Chinese string
func formatBool(b bool) string {
	if b {
		return "是"
	}
	return "否"
}
