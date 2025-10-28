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
	config           *config.DingTalkConfig
	client           *http.Client
	dashboardService *DashboardService
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
func NewDingTalkService(config *config.DingTalkConfig, dashboardService *DashboardService) *DingTalkService {
	return &DingTalkService{
		config:           config,
		client:           &http.Client{Timeout: 10 * time.Second},
		dashboardService: dashboardService,
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

	// Get additional statistics from dashboard service
	imageAltCount := s.dashboardService.GetImageAltCount(crawl.Id)
	schemeCount := s.dashboardService.GetSchemeCount(crawl.Id)
	mediaTypeCount := s.dashboardService.GetMediaCount(crawl.Id)
	statusCodeCount := s.dashboardService.GetStatusCount(crawl.Id)
	statusCodeByDepth := s.dashboardService.GetStatusCodeByDepth(crawl.Id)
	titleStats := s.dashboardService.GetTitleStats(crawl.Id)
	descriptionStats := s.dashboardService.GetDescriptionStats(crawl.Id)

	text := fmt.Sprintf(`
### %s SEO审计报告完成
- **网站：** %s
- **审计时间：** %s
- **审计耗时：** %s

### 📊 爬取统计
- **总URL数：** %d
- **被robots.txt阻止：** %d
- **Noindex页面：** %d

### 🔗 链接统计
- **内部链接：** %d
- **外部链接：** %d

### 🖼️ 图片Alt属性统计
- **有Alt属性：** %d
- **无Alt属性：** %d

### 🔒 HTTP/HTTPS统计
- **HTTP页面：** %d
- **HTTPS页面：** %d

### 📄 媒体类型统计%s

### 📊 状态码统计%s

### 📈 页面深度分布%s

### 📝 标题统计
- **总页面数：** %d
- **空标题：** %d
- **短标题(<20字符)：** %d
- **长标题(>60字符)：** %d
- **多标题标签：** %d
- **重复标题：** %d

### 📄 描述统计
- **总页面数：** %d
- **空描述：** %d
- **短描述(<80字符)：** %d
- **长描述(>160字符)：** %d
- **多描述标签：** %d
- **重复描述：** %d

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
		// crawl.CriticalIssues,
		// crawl.AlertIssues,
		// crawl.WarningIssues,
		// crawl.TotalIssues,
		crawl.InternalFollowLinks+crawl.InternalNoFollowLinks,
		crawl.ExternalFollowLinks+crawl.ExternalNoFollowLinks,
		// crawl.SponsoredLinks,
		// crawl.UGCLinks,
		imageAltCount.Alt,
		imageAltCount.NonAlt,
		schemeCount.HTTP,
		schemeCount.HTTPS,
		s.formatMediaTypeStats(mediaTypeCount),
		s.formatStatusCodeStats(statusCodeCount),
		s.formatStatusCodeByDepthStats(statusCodeByDepth),
		titleStats.TotalPages,
		titleStats.EmptyTitle,
		titleStats.ShortTitle,
		titleStats.LongTitle,
		titleStats.MultipleTitles,
		titleStats.DuplicateTitle,
		descriptionStats.TotalPages,
		descriptionStats.EmptyDescription,
		descriptionStats.ShortDescription,
		descriptionStats.LongDescription,
		descriptionStats.MultipleDescriptions,
		descriptionStats.DuplicateDescription,
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

// formatMediaTypeStats formats media type statistics
func (s *DingTalkService) formatMediaTypeStats(chart *models.Chart) string {
	if chart == nil || len(*chart) == 0 {
		return "\n- 无数据"
	}

	var result string
	for _, item := range *chart {
		result += fmt.Sprintf("\n- **%s：** %d", item.Key, item.Value)
	}
	return result
}

// formatStatusCodeStats formats status code statistics
func (s *DingTalkService) formatStatusCodeStats(chart *models.Chart) string {
	if chart == nil || len(*chart) == 0 {
		return "\n- 无数据"
	}

	var result string
	for _, item := range *chart {
		result += fmt.Sprintf("\n- **%s：** %d", item.Key, item.Value)
	}
	return result
}

// formatStatusCodeByDepthStats formats status code by depth statistics
func (s *DingTalkService) formatStatusCodeByDepthStats(statusCodes []models.StatusCodeByDepth) string {
	if len(statusCodes) == 0 {
		return "\n- 无数据"
	}

	// Calculate totals
	var totalPages int
	var depth1_2, depth3_4, depth5_6, depth7_8 int

	for _, sc := range statusCodes {
		depthTotal := sc.StatusCode100 + sc.StatusCode200 + sc.StatusCode300 + sc.StatusCode400 + sc.StatusCode500
		if depthTotal > 0 {
			totalPages += depthTotal

			// Group by depth ranges
			switch {
			case sc.Depth <= 2:
				depth1_2 += depthTotal
			case sc.Depth <= 4:
				depth3_4 += depthTotal
			case sc.Depth <= 6:
				depth5_6 += depthTotal
			case sc.Depth <= 8:
				depth7_8 += depthTotal
			}
		}
	}

	if totalPages == 0 {
		return "\n- 无数据"
	}

	// Calculate percentages
	depth1_2Percent := float64(depth1_2) / float64(totalPages) * 100
	depth3_4Percent := float64(depth3_4) / float64(totalPages) * 100
	depth5_6Percent := float64(depth5_6) / float64(totalPages) * 100
	depth7_8Percent := float64(depth7_8) / float64(totalPages) * 100

	result := fmt.Sprintf(`
- **总页面数：** %d
- **深度1-2：** %d页 (%.1f%%)
- **深度3-4：** %d页 (%.1f%%) 
- **深度5-6：** %d页 (%.1f%%)
- **深度7-8：** %d页 (%.1f%%)
`,
		totalPages,
		depth1_2, depth1_2Percent,
		depth3_4, depth3_4Percent,
		depth5_6, depth5_6Percent,
		depth7_8, depth7_8Percent,
	)

	return result
}

// formatBool formats boolean to Chinese string
func formatBool(b bool) string {
	if b {
		return "是"
	}
	return "否"
}
