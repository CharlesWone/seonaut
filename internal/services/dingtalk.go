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
	// 校验项目的推送地址
	if !s.config.Enabled || project.DingtalkWebhookUrl == "" {
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

	return s.sendMessage(message, project.DingtalkWebhookUrl)
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

	// Build report sections with abnormal indicators highlighted
	text := fmt.Sprintf(`
### %s SEO审计报告完成
- **网站：** %s
- **审计时间：** %s
- **审计耗时：** %s

### 📊 爬取统计
- **总URL数：** %d
%s
%s

### 🔗 链接统计
- **内部链接：** %d
- **外部链接：** %d

### 🖼️ 图片Alt属性统计
- **有Alt属性：** %d
%s

### 🔒 HTTP/HTTPS统计
%s
- **HTTPS页面：** %d

### 📄 媒体类型统计%s

### 📊 状态码统计%s

### 📈 页面深度分布%s

### 📝 标题统计
- **总页面数：** %d
%s
%s
%s
%s
%s

### 📄 描述统计
- **总页面数：** %d
%s
%s
%s
%s
%s

### 🤖 技术信息
%s
- **Sitemap存在：** %s
%s

---
*报告生成时间：%s*`,
		statusEmoji,
		project.URL,
		crawl.Start.Format("2006-01-02 15:04:05"),
		formatDuration(duration),
		crawl.TotalURLs,
		formatIntMetric("被robots.txt阻止", crawl.BlockedByRobotstxt, crawl.BlockedByRobotstxt > 0),
		formatIntMetric("Noindex页面", crawl.Noindex, crawl.Noindex > 0),
		crawl.InternalFollowLinks+crawl.InternalNoFollowLinks,
		crawl.ExternalFollowLinks+crawl.ExternalNoFollowLinks,
		imageAltCount.Alt,
		formatIntMetric("无Alt属性", imageAltCount.NonAlt, imageAltCount.NonAlt > 0),
		formatIntMetric("HTTP页面", schemeCount.HTTP, schemeCount.HTTP > 0),
		schemeCount.HTTPS,
		s.formatMediaTypeStats(mediaTypeCount),
		s.formatStatusCodeStats(statusCodeCount),
		s.formatStatusCodeByDepthStats(statusCodeByDepth),
		titleStats.TotalPages,
		formatIntMetric("空标题", titleStats.EmptyTitle, titleStats.EmptyTitle > 0),
		formatIntMetric("短标题(<20字符)", titleStats.ShortTitle, titleStats.ShortTitle > 0),
		formatIntMetric("长标题(>60字符)", titleStats.LongTitle, titleStats.LongTitle > 0),
		formatIntMetric("多标题标签", titleStats.MultipleTitles, titleStats.MultipleTitles > 0),
		formatIntMetric("重复标题", titleStats.DuplicateTitle, titleStats.DuplicateTitle > 0),
		descriptionStats.TotalPages,
		formatIntMetric("空描述", descriptionStats.EmptyDescription, descriptionStats.EmptyDescription > 0),
		formatIntMetric("短描述(<80字符)", descriptionStats.ShortDescription, descriptionStats.ShortDescription > 0),
		formatIntMetric("长描述(>160字符)", descriptionStats.LongDescription, descriptionStats.LongDescription > 0),
		formatIntMetric("多描述标签", descriptionStats.MultipleDescriptions, descriptionStats.MultipleDescriptions > 0),
		formatIntMetric("重复描述", descriptionStats.DuplicateDescription, descriptionStats.DuplicateDescription > 0),
		formatStringMetric("Robots.txt存在", formatBool(crawl.RobotstxtExists), !crawl.RobotstxtExists),
		formatBool(crawl.SitemapExists),
		formatStringMetric("Sitemap被阻止", formatBool(crawl.SitemapIsBlocked), crawl.SitemapIsBlocked),
		time.Now().Format("2006-01-02 15:04:05"),
	)

	// Add optimization suggestions
	suggestions := s.generateOptimizationSuggestions(crawl, imageAltCount, schemeCount, statusCodeCount, titleStats, descriptionStats)
	if suggestions != "" {
		text += "\n\n" + suggestions
	}

	return text
}

// sendMessage sends the message to DingTalk webhook
func (s *DingTalkService) sendMessage(message DingTalkMessage, webhookUrl string) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// 使用项目的webhook地址
	req, err := http.NewRequest("POST", webhookUrl, bytes.NewBuffer(jsonData))

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

	// 设置超时时间
	s.client.Timeout = 60 * time.Second
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
		// Check if status code is 4xx or 5xx (error codes)
		isAbnormal := false
		if len(item.Key) >= 1 {
			firstDigit := item.Key[0]
			if firstDigit == '4' || firstDigit == '5' {
				isAbnormal = true
			}
		}

		if isAbnormal {
			result += fmt.Sprintf("\n- **%s：** <font color=\"red\">%d</font>", item.Key, item.Value)
		} else {
			result += fmt.Sprintf("\n- **%s：** %d", item.Key, item.Value)
		}
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

// formatMetric formats a metric with optional red highlighting if abnormal
func formatMetric(name string, value interface{}, isAbnormal bool) string {
	valueStr := fmt.Sprintf("%v", value)
	if isAbnormal {
		return fmt.Sprintf("- <font color=\"red\">**%s：** %s</font>", name, valueStr)
	}
	return fmt.Sprintf("- **%s：** %s", name, valueStr)
}

// formatIntMetric formats an integer metric with optional red highlighting
func formatIntMetric(name string, value int, isAbnormal bool) string {
	return formatMetric(name, value, isAbnormal)
}

// formatStringMetric formats a string metric with optional red highlighting
func formatStringMetric(name string, value string, isAbnormal bool) string {
	return formatMetric(name, value, isAbnormal)
}

// generateOptimizationSuggestions generates optimization suggestions based on abnormal indicators
func (s *DingTalkService) generateOptimizationSuggestions(
	crawl *models.Crawl,
	imageAltCount *models.AltCount,
	schemeCount *models.SchemeCount,
	statusCodeCount *models.Chart,
	titleStats *models.TitleStats,
	descriptionStats *models.DescriptionStats,
) string {
	var suggestions []string

	// Check for robots.txt blocking
	if crawl.BlockedByRobotstxt > 0 {
		suggestions = append(suggestions, fmt.Sprintf("- **被robots.txt阻止：** 发现 %d 个URL被robots.txt阻止。请检查robots.txt配置，确保重要页面未被意外阻止。", crawl.BlockedByRobotstxt))
	}

	// Check for noindex pages
	if crawl.Noindex > 0 {
		suggestions = append(suggestions, fmt.Sprintf("- **Noindex页面：** 发现 %d 个页面设置了noindex标签。请确认这些页面是否需要被搜索引擎索引，如不需要请忽略。", crawl.Noindex))
	}

	// Check for images without alt text
	if imageAltCount.NonAlt > 0 {
		totalImages := imageAltCount.Alt + imageAltCount.NonAlt
		if totalImages > 0 {
			percentage := float64(imageAltCount.NonAlt) / float64(totalImages) * 100
			suggestions = append(suggestions, fmt.Sprintf("- **图片Alt属性：** 发现 %d 张图片（%.1f%%）缺少Alt属性。建议为所有图片添加描述性的Alt文本，提升可访问性和SEO效果。", imageAltCount.NonAlt, percentage))
		} else {
			suggestions = append(suggestions, fmt.Sprintf("- **图片Alt属性：** 发现 %d 张图片缺少Alt属性。建议为所有图片添加描述性的Alt文本，提升可访问性和SEO效果。", imageAltCount.NonAlt))
		}
	}

	// Check for HTTP pages
	if schemeCount.HTTP > 0 {
		totalPages := schemeCount.HTTP + schemeCount.HTTPS
		if totalPages > 0 {
			percentage := float64(schemeCount.HTTP) / float64(totalPages) * 100
			suggestions = append(suggestions, fmt.Sprintf("- **HTTPS迁移：** 发现 %d 个页面（%.1f%%）仍使用HTTP协议。建议将全部页面迁移至HTTPS，提升安全性和SEO排名。", schemeCount.HTTP, percentage))
		} else {
			suggestions = append(suggestions, fmt.Sprintf("- **HTTPS迁移：** 发现 %d 个页面仍使用HTTP协议。建议将全部页面迁移至HTTPS，提升安全性和SEO排名。", schemeCount.HTTP))
		}
	}

	// Check for error status codes
	if statusCodeCount != nil {
		var error4xx, error5xx int
		for _, item := range *statusCodeCount {
			if len(item.Key) >= 1 {
				firstDigit := item.Key[0]
				if firstDigit == '4' {
					error4xx += item.Value
				} else if firstDigit == '5' {
					error5xx += item.Value
				}
			}
		}
		if error4xx > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **4xx错误：** 发现 %d 个页面返回4xx客户端错误。请检查链接是否正确，修复或删除无效页面，避免影响用户体验和SEO。", error4xx))
		}
		if error5xx > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **5xx错误：** 发现 %d 个页面返回5xx服务器错误。请立即检查服务器配置和代码，修复服务器端问题。", error5xx))
		}
	}

	// Check for title issues
	if titleStats.TotalPages > 0 {
		if titleStats.EmptyTitle > 0 {
			percentage := float64(titleStats.EmptyTitle) / float64(titleStats.TotalPages) * 100
			suggestions = append(suggestions, fmt.Sprintf("- **空标题：** 发现 %d 个页面（%.1f%%）缺少标题标签。建议为所有页面添加唯一且描述性的标题，长度控制在20-60字符之间。", titleStats.EmptyTitle, percentage))
		}
		if titleStats.ShortTitle > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **短标题：** 发现 %d 个页面标题过短（<20字符）。建议优化标题，使其更具体、更具描述性，以提升SEO效果。", titleStats.ShortTitle))
		}
		if titleStats.LongTitle > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **长标题：** 发现 %d 个页面标题过长（>60字符）。建议将标题长度控制在60字符以内，避免在搜索结果中被截断。", titleStats.LongTitle))
		}
		if titleStats.MultipleTitles > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **多标题标签：** 发现 %d 个页面存在多个标题标签。每个页面应只有一个标题标签，请删除多余的标题标签。", titleStats.MultipleTitles))
		}
		if titleStats.DuplicateTitle > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **重复标题：** 发现 %d 个页面使用重复标题。建议为每个页面创建唯一标题，提升页面区分度和SEO效果。", titleStats.DuplicateTitle))
		}
	}

	// Check for description issues
	if descriptionStats.TotalPages > 0 {
		if descriptionStats.EmptyDescription > 0 {
			percentage := float64(descriptionStats.EmptyDescription) / float64(descriptionStats.TotalPages) * 100
			suggestions = append(suggestions, fmt.Sprintf("- **空描述：** 发现 %d 个页面（%.1f%%）缺少meta描述。建议为所有页面添加吸引人的描述，长度控制在80-160字符之间。", descriptionStats.EmptyDescription, percentage))
		}
		if descriptionStats.ShortDescription > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **短描述：** 发现 %d 个页面描述过短（<80字符）。建议优化描述内容，使其更详细、更具吸引力。", descriptionStats.ShortDescription))
		}
		if descriptionStats.LongDescription > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **长描述：** 发现 %d 个页面描述过长（>160字符）。建议将描述长度控制在160字符以内，避免在搜索结果中被截断。", descriptionStats.LongDescription))
		}
		if descriptionStats.MultipleDescriptions > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **多描述标签：** 发现 %d 个页面存在多个meta描述标签。每个页面应只有一个描述标签，请删除多余的描述标签。", descriptionStats.MultipleDescriptions))
		}
		if descriptionStats.DuplicateDescription > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **重复描述：** 发现 %d 个页面使用重复描述。建议为每个页面创建唯一描述，提升页面吸引力。", descriptionStats.DuplicateDescription))
		}
	}

	// Check for robots.txt and sitemap issues
	if !crawl.RobotstxtExists {
		suggestions = append(suggestions, "- **Robots.txt：** 网站未配置robots.txt文件。建议创建robots.txt文件，指导搜索引擎爬虫的访问行为。")
	}
	if crawl.SitemapIsBlocked {
		suggestions = append(suggestions, "- **Sitemap被阻止：** Sitemap被robots.txt阻止。建议修改robots.txt配置，允许搜索引擎访问sitemap。")
	}

	// If no suggestions, return empty string
	if len(suggestions) == 0 {
		return ""
	}

	// Format suggestions section
	result := "### 💡 优化建议\n\n"
	for _, suggestion := range suggestions {
		result += suggestion + "\n"
	}

	return result
}
