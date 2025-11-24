package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/stjudewashere/seonaut/internal/utils"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stjudewashere/seonaut/internal/config"
	"github.com/stjudewashere/seonaut/internal/models"
)

// DingTalkService handles sending notifications to DingTalk webhook
type DingTalkService struct {
	config           *config.DingTalkConfig
	client           *http.Client
	dashboardService *DashboardService
	serverConfig     *config.HTTPServerConfig
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
func NewDingTalkService(config *config.DingTalkConfig, dashboardService *DashboardService, serverConfig *config.HTTPServerConfig) *DingTalkService {
	return &DingTalkService{
		config:           config,
		client:           &http.Client{Timeout: 10 * time.Second},
		dashboardService: dashboardService,
		serverConfig:     serverConfig,
	}
}

// SendCrawlReport sends crawl completion report to DingTalk
func (s *DingTalkService) SendCrawlReport(crawl *models.Crawl, project *models.Project) error {
	// 校验项目的推送地址
	if !s.config.Enabled || project.DingtalkWebhookUrl == "" {
		return nil
	}

	//markdownText := s.formatCrawlReport(crawl, project)
	markdownText := s.buildMarkdownCrawlReport(crawl, project)

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

	// 报告链接
	enc, _ := utils.EncryptParam(strconv.FormatInt(project.Id, 10))
	//crawlReportLink := fmt.Sprintf("%s/crawlReport/%s", strings.TrimRight(s.serverConfig.URL, "/"), enc)
	crawlReportLink := fmt.Sprintf("%s/issuesReport/%s", strings.TrimRight(s.serverConfig.URL, "/"), enc)

	// Get additional statistics from dashboard service
	imageAltCount := s.dashboardService.GetImageAltCount(crawl.Id)
	schemeCount := s.dashboardService.GetSchemeCount(crawl.Id)
	mediaTypeCount := s.dashboardService.GetMediaCount(crawl.Id)
	allMediaTypes := s.dashboardService.GetMediaTypeDetails(crawl.Id)
	statusCodeCount := s.dashboardService.GetStatusCount(crawl.Id)
	statusCodeByDepth := s.dashboardService.GetStatusCodeByDepth(crawl.Id)
	titleStats := s.dashboardService.GetTitleStats(crawl.Id)
	descriptionStats := s.dashboardService.GetDescriptionStats(crawl.Id)

	// Build report sections with abnormal indicators highlighted
	text := fmt.Sprintf(`
### %s SEO审计报告完成
- **网站：** %s
- **报告链接：** %s
- **审计时间：** %s
- **审计耗时：** %s

### 📊 爬取统计
- **总URL数：** %d
%s
%s

### 🔗 链接统计
%s

### 🖼️ 图片Alt属性统计
%s
%s

### 🔒 HTTP/HTTPS统计
%s
%s

### 📄 媒体类型统计%s

### 📊 状态码统计%s

### 📈 页面深度分布%s

### 📝 标题统计
%s
%s
%s
%s
%s
%s

### 📄 描述统计
%s
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
		crawlReportLink,
		crawl.Start.Format("2006-01-02 15:04:05"),
		formatDuration(duration),
		crawl.TotalURLs,
		formatIntMetric("被robots.txt阻止", crawl.BlockedByRobotstxt, crawl.BlockedByRobotstxt > 0),
		formatIntMetric("Noindex页面", crawl.Noindex, crawl.Noindex > 0),
		s.formatLinkStats(crawl),
		fmt.Sprintf("- **有Alt属性：** %d", imageAltCount.Alt),
		formatIntMetric("无Alt属性", imageAltCount.NonAlt, imageAltCount.NonAlt > 0),
		formatIntMetric("HTTP页面", schemeCount.HTTP, schemeCount.HTTP > 0),
		fmt.Sprintf("- **HTTPS页面：** %d", schemeCount.HTTPS),
		s.formatMediaTypeStats(mediaTypeCount, allMediaTypes),
		s.formatStatusCodeStats(statusCodeCount),
		s.formatStatusCodeByDepthStats(statusCodeByDepth),
		fmt.Sprintf("- **总页面数：** %d", titleStats.TotalPages),
		formatIntMetric("空标题", titleStats.EmptyTitle, titleStats.EmptyTitle > 0),
		formatIntMetric("短标题(<20字符)", titleStats.ShortTitle, titleStats.ShortTitle > 0),
		formatIntMetric("长标题(>60字符)", titleStats.LongTitle, titleStats.LongTitle > 0),
		formatIntMetric("多标题标签", titleStats.MultipleTitles, titleStats.MultipleTitles > 0),
		formatIntMetric("重复标题", titleStats.DuplicateTitle, titleStats.DuplicateTitle > 0),
		fmt.Sprintf("- **总页面数：** %d", descriptionStats.TotalPages),
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
	suggestions := s.generateOptimizationSuggestions(crawl, imageAltCount, schemeCount, statusCodeCount, titleStats, descriptionStats, allMediaTypes, statusCodeByDepth)
	if suggestions != "" {
		text += "\n\n --- \n\n"
		text += "\n\n" + suggestions
	}

	return text
}

// 构建markdown格式的爬虫报告
func (s *DingTalkService) buildMarkdownCrawlReport(crawl *models.Crawl, project *models.Project) string {
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

	// 严重问题  - **严重问题：** %d
	var criticalIssuesStr string
	if crawl.CriticalIssues > 0 {
		criticalIssuesStr = fmt.Sprintf("- <font color=\"#ff0000\">**严重问题：** %d</font> \n", crawl.CriticalIssues)
	} else {
		criticalIssuesStr = fmt.Sprintf("- **严重问题：** %d \n", crawl.CriticalIssues)
	}

	// 提示问题  - **提示问题：** %d
	warningIssuesStr := fmt.Sprintf("- **提示问题：** %d \n", crawl.WarningIssues)

	// 警告问题  - **警告问题：** %d
	var alertIssuesStr string
	if crawl.AlertIssues > 0 {
		alertIssuesStr = fmt.Sprintf("- <font color=\"#f1d460\">**警告问题：** %d</font> \n", crawl.AlertIssues)
	} else {
		alertIssuesStr = fmt.Sprintf("- **警告问题：** %d \n", crawl.AlertIssues)
	}

	// 报告链接
	enc, _ := utils.EncryptParam(strconv.FormatInt(crawl.Id, 10))
	crawlReportLink := fmt.Sprintf("%s/issuesReport/%s", strings.TrimRight(s.serverConfig.URL, "/"), enc)

	data := struct {
		MediaChart        *models.Chart
		StatusChart       *models.Chart
		CanonicalCount    *models.CanonicalCount
		AltCount          *models.AltCount
		SchemeCount       *models.SchemeCount
		StatusCodeByDepth []models.StatusCodeByDepth
	}{
		MediaChart:        s.dashboardService.GetMediaCount(crawl.Id),
		StatusChart:       s.dashboardService.GetStatusCount(crawl.Id),
		CanonicalCount:    s.dashboardService.GetCanonicalCount(crawl.Id),
		AltCount:          s.dashboardService.GetImageAltCount(crawl.Id),
		SchemeCount:       s.dashboardService.GetSchemeCount(crawl.Id),
		StatusCodeByDepth: s.dashboardService.GetStatusCodeByDepth(crawl.Id),
	}

	totalInternalLinks := crawl.InternalFollowLinks + crawl.InternalNoFollowLinks
	totalExternalLinks := crawl.ExternalFollowLinks + crawl.ExternalNoFollowLinks
	totalLinks := totalInternalLinks + totalExternalLinks

	// 占比
	percent := func(a, b int) string {
		if b == 0 {
			return "0.0%"
		}
		p := float64(a) / float64(b) * 100
		// 防止出现 -0.0% 的情况
		if math.Signbit(p) && p > -0.0001 {
			p = 0
		}
		return fmt.Sprintf("%.1f%%", p)
	}

	// 媒体类型 ### 📄 媒体类型统计
	mediaTypeStr := ``
	totalMediaTypeCount := 0
	for _, item := range *data.MediaChart {
		totalMediaTypeCount += item.Value
	}
	if totalMediaTypeCount > 0 {
		mediaTypeStr += fmt.Sprintf("### 📄 媒体类型统计 \n")
		for _, i := range *data.MediaChart {
			mediaTypeStr += fmt.Sprintf("- **%s：** %d (%s) \n", i.Key, i.Value, percent(i.Value, totalMediaTypeCount))
		}
	}

	// 状态码 ### 📊 状态码统计
	statusCodeStr := fmt.Sprintf("### 📊 状态码统计\n")
	statusCodeStr += s.formatStatusCodeStats(data.StatusChart)
	//if data.StatusChart != nil && len(*data.StatusChart) > 0 {
	//	statusCodeStr += fmt.Sprintf("### 📊 状态码统计\n")
	//	for _, i := range *data.StatusChart {
	//		statusCodeStr += fmt.Sprintf("- **%s：** %d \n", i.Key, i.Value)
	//	}
	//}

	//// ### 📈 页面深度分布（推荐最终版，彻底解决所有问题）
	//depthStr := ""
	//depthTotal := 0
	//// 先计算总页面数
	//for _, i := range data.StatusCodeByDepth {
	//	depthTotal += i.StatusCode100 + i.StatusCode200 + i.StatusCode300 + i.StatusCode400 + i.StatusCode500
	//}
	//
	//if depthTotal > 0 {
	//	depthStr += "### 📈 页面深度分布\n"
	//	depthStr += fmt.Sprintf("- **总页面数：** %d\n", depthTotal)
	//
	//	// 用 map 按 depth 存数量，便于后面分组
	//	depthCount := make(map[int]int)
	//	maxDepth := 0
	//	for _, i := range data.StatusCodeByDepth {
	//		count := i.StatusCode100 + i.StatusCode200 + i.StatusCode300 + i.StatusCode400 + i.StatusCode500
	//		if count > 0 {
	//			depthCount[i.Depth] = count
	//			if i.Depth > maxDepth {
	//				maxDepth = i.Depth
	//			}
	//		}
	//	}
	//
	//	// 从 1 开始，每两层一组，直到盖过最大深度
	//	for low := 1; low <= maxDepth+2; low += 2 { // +2 确保能盖到奇数层
	//		high := low + 1
	//
	//		countLow := depthCount[low]
	//		countHigh := depthCount[high]
	//
	//		totalInGroup := countLow + countHigh
	//
	//		// 只有当这一组有页面时才显示（关键！彻底杜绝 0 页行）
	//		if totalInGroup == 0 {
	//			continue
	//		}
	//
	//		if countHigh > 0 {
	//			depthStr += fmt.Sprintf("- **深度%d-%d：** %d页 (%s)\n", low, high, totalInGroup, percent(totalInGroup, depthTotal))
	//		} else {
	//			// 只有 low 有数据（奇数层结尾）
	//			depthStr += fmt.Sprintf("- **深度%d：** %d页 (%s)\n", low, totalInGroup, percent(totalInGroup, depthTotal))
	//		}
	//	}
	//}

	// ### 📈 页面深度分布（百分比强制加起来 100.0% 版）
	depthStr := ""
	depthTotal := 0

	for _, i := range data.StatusCodeByDepth {
		depthTotal += i.StatusCode100 + i.StatusCode200 + i.StatusCode300 + i.StatusCode400 + i.StatusCode500
	}

	if depthTotal > 0 {
		depthStr += "### 📈 页面深度分布\n"
		depthStr += fmt.Sprintf("- **总页面数：** %d\n", depthTotal)

		depthCount := make(map[int]int)
		maxDepth := 0
		for _, i := range data.StatusCodeByDepth {
			count := i.StatusCode100 + i.StatusCode200 + i.StatusCode300 + i.StatusCode400 + i.StatusCode500
			if count > 0 {
				depthCount[i.Depth] = count
				if i.Depth > maxDepth {
					maxDepth = i.Depth
				}
			}
		}

		// 收集所有分组，用于最后补差
		type group struct {
			low, high  int
			count      int
			percentStr string // 先存原始百分比字符串
		}
		var groups []group

		// 第一步：正常计算每一组的原始百分比（保留原始 float）
		for low := 1; low <= maxDepth+2; low += 2 {
			high := low + 1
			countLow := depthCount[low]
			countHigh := depthCount[high]
			totalInGroup := countLow + countHigh
			if totalInGroup == 0 {
				continue
			}

			rawPercent := float64(totalInGroup) / float64(depthTotal) * 100
			percentStr := fmt.Sprintf("%.1f%%", rawPercent)

			if countHigh > 0 {
				groups = append(groups, group{low: low, high: high, count: totalInGroup, percentStr: percentStr})
			} else {
				groups = append(groups, group{low: low, high: 0, count: totalInGroup, percentStr: percentStr})
			}
		}

		// 第二步：如果有多行，把最后一个分组用来“兜底补差”
		if len(groups) > 1 {
			// 重新计算所有原始 float 值，求和
			sumDisplayed := 0.0
			for i := 0; i < len(groups)-1; i++ {
				// 重新计算前几行的精确百分比并四舍五入
				p := float64(groups[i].count) / float64(depthTotal) * 100
				displayed := math.Round(p*10) / 10 // 强制保留一位小数
				sumDisplayed += displayed
			}

			// 最后一个用 100 - 前面的和（完美 100.0%）
			lastDisplayed := 100.0 - sumDisplayed
			if lastDisplayed < 0 {
				lastDisplayed = 0 // 防止极端浮点误差
			}
			groups[len(groups)-1].percentStr = fmt.Sprintf("%.1f%%", lastDisplayed)
		}

		// 第三步：输出最终结果
		for _, g := range groups {
			if g.high > 0 {
				if g.low > 4 {
					depthStr += fmt.Sprintf("- <font color=\"#ff0000\">**深度%d-%d：** %d页 (%s)</font>\n", g.low, g.high, g.count, g.percentStr)
				} else {
					depthStr += fmt.Sprintf("- **深度%d-%d：** %d页 (%s)\n", g.low, g.high, g.count, g.percentStr)
				}
			} else {
				depthStr += fmt.Sprintf("- **深度%d：** %d页 (%s)\n", g.low, g.count, g.percentStr)
			}
		}
	}

	robotsUrlStr := ""
	sitemapUrlStr := ""
	u, _ := url.Parse(project.URL)
	if u != nil {
		if crawl.RobotstxtExists {
			robotsUrlStr = fmt.Sprintf(" | [链接>>](%s)", u.Scheme+"://"+u.Host+"/robots.txt")
		}
		if crawl.SitemapExists {
			sitemapUrlStr = fmt.Sprintf(" | [链接>>](%s)", u.Scheme+"://"+u.Host+"/sitemap.xml")
		}
	}

	text := fmt.Sprintf(`
### %s SEO审计报告完成
- **网站：** %s
- **详细报告：** %s
- **审计时间：** %s
- **审计耗时：** %s

### ⚠️ 网站问题统计
%s
%s
%s

### 📊 爬取统计
- **总URL数：** %d

### 🔗 链接统计
- **总链接数：** %d
- **内部follow链接：** %d (%s)
- **内部nofollow链接：** %d (%s)
- **外部follow链接：** %d (%s)
- **外部nofollow链接：** %d (%s)
- **Sponsored链接：** %d
- **UGC链接：** %d

### 🖼️ 图片Alt属性统计
- **有Alt属性：** %d
%s

### 🔒 HTTP/HTTPS统计
%s
- **HTTPS页面：** %d

%s

%s

%s

### 🤖 技术信息
%s
%s
%s

---
*报告生成时间：%s*`,
		statusEmoji,
		project.URL,
		fmt.Sprintf("[点击查看>>](%s)", crawlReportLink),
		crawl.Start.Format("2006-01-02 15:04:05"),
		formatDuration(duration),

		criticalIssuesStr,
		alertIssuesStr, // warningIssuesStr,
		warningIssuesStr,

		crawl.TotalURLs,

		totalLinks,
		crawl.InternalFollowLinks,
		percent(crawl.InternalFollowLinks, totalInternalLinks),
		crawl.InternalNoFollowLinks,
		percent(crawl.InternalNoFollowLinks, totalInternalLinks),
		crawl.ExternalFollowLinks,
		percent(crawl.ExternalFollowLinks, totalExternalLinks),
		crawl.ExternalNoFollowLinks,
		percent(crawl.ExternalNoFollowLinks, totalExternalLinks),
		crawl.SponsoredLinks,
		crawl.UGCLinks,

		//data.CanonicalCount.Canonical,
		//data.CanonicalCount.NonCanonical,

		data.AltCount.Alt,
		formatIntMetric("无Alt属性", data.AltCount.NonAlt, data.AltCount.NonAlt > 0),

		formatIntMetric("HTTP页面", data.SchemeCount.HTTP, data.SchemeCount.HTTP > 0),
		data.SchemeCount.HTTPS,

		mediaTypeStr,

		statusCodeStr,

		depthStr,

		formatStringMetric("Robots.txt存在", formatBool(crawl.RobotstxtExists), !crawl.RobotstxtExists)+robotsUrlStr,
		formatStringMetric("Sitemap存在", formatBool(crawl.SitemapExists), !crawl.SitemapExists)+sitemapUrlStr,
		formatStringMetric("Sitemap被阻止", formatBool(crawl.SitemapIsBlocked), crawl.SitemapIsBlocked),
		time.Now().Format("2006-01-02 15:04:05"),
	)
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

// formatMediaTypeStats formats media type statistics with abnormal indicators
func (s *DingTalkService) formatMediaTypeStats(chart *models.Chart, allMediaTypes []models.MediaTypeDetail) string {
	if chart == nil || len(*chart) == 0 {
		return "\n- 无数据"
	}

	// Calculate total count from all media types
	var totalCount int
	for _, mt := range allMediaTypes {
		totalCount += mt.Count
	}

	if totalCount == 0 {
		return "\n- 无数据"
	}

	// Build a map of media types from chart for quick lookup
	chartMap := make(map[string]int)
	for _, item := range *chart {
		chartMap[item.Key] = item.Value
	}

	var result string
	for _, item := range *chart {
		percentage := float64(item.Value) / float64(totalCount) * 100
		isAbnormal := false

		// Check if this is "Other" type
		if item.Key == "Other" {
			// For Other type, check if percentage > 10%
			if percentage > 10 {
				isAbnormal = true
			}
		} else {
			// Check for specific media types
			if item.Key == "image/png" && percentage > 5 {
				isAbnormal = true
			} else if item.Key == "image/gif" && percentage > 5 {
				isAbnormal = true
			} else if item.Key != "text/html" && percentage > 10 {
				// Other non-HTML types with > 10% are abnormal
				isAbnormal = true
			}
		}

		if isAbnormal {
			result += fmt.Sprintf("\n- <font color=\"#ff0000\">**%s：** %d (%.1f%%)</font>", item.Key, item.Value, percentage)
		} else {
			result += fmt.Sprintf("\n- **%s：** %d (%.1f%%)", item.Key, item.Value, percentage)
		}

		// If this is "Other" type, show details
		if item.Key == "Other" && len(allMediaTypes) > 4 {
			// Get the media types that are in "Other" (after the first 4)
			result += "\n  <font color=\"gray\">包含："
			for i := 4; i < len(allMediaTypes) && i < 10; i++ {
				if i > 4 {
					result += "、"
				}
				result += allMediaTypes[i].MediaType
			}
			if len(allMediaTypes) > 10 {
				result += "等"
			}
			result += "</font>"
		}
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
			result += fmt.Sprintf("\n- <font color=\"#ff0000\">**%s：** %d</font>", item.Key, item.Value)
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

	// Determine if depth 5-8 is abnormal (> 20%) or depth 7-8 is abnormal (> 10%)
	depth5_8Percent := depth5_6Percent + depth7_8Percent
	depth5_8Abnormal := depth5_8Percent > 20
	depth7_8Abnormal := depth7_8Percent > 10

	result := fmt.Sprintf(`
- **总页面数：** %d
- **深度1-2：** %d页 (%.1f%%)
- **深度3-4：** %d页 (%.1f%%)`,
		totalPages,
		depth1_2, depth1_2Percent,
		depth3_4, depth3_4Percent,
	)

	// Format depth 5-6 with red if abnormal
	if depth5_8Abnormal {
		result += fmt.Sprintf("\n- <font color=\"#ff0000\">**深度5-6：** %d页 (%.1f%%)</font>", depth5_6, depth5_6Percent)
	} else {
		result += fmt.Sprintf("\n- **深度5-6：** %d页 (%.1f%%)", depth5_6, depth5_6Percent)
	}

	// Format depth 7-8 with red if abnormal
	if depth7_8Abnormal {
		result += fmt.Sprintf("\n- <font color=\"#ff0000\">**深度7-8：** %d页 (%.1f%%)</font>", depth7_8, depth7_8Percent)
	} else {
		result += fmt.Sprintf("\n- **深度7-8：** %d页 (%.1f%%)", depth7_8, depth7_8Percent)
	}

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
		return fmt.Sprintf("- <font color=\"#ff0000\">**%s：** %s</font>", name, valueStr)
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

// formatLinkStats formats link statistics with abnormal indicators
func (s *DingTalkService) formatLinkStats(crawl *models.Crawl) string {
	internalTotal := crawl.InternalFollowLinks + crawl.InternalNoFollowLinks
	externalTotal := crawl.ExternalFollowLinks + crawl.ExternalNoFollowLinks

	result := fmt.Sprintf("- **内部链接：** %d", internalTotal)

	// Check internal nofollow ratio
	if internalTotal > 0 {
		internalNoFollowRatio := float64(crawl.InternalNoFollowLinks) / float64(internalTotal)
		if internalNoFollowRatio > 0.3 {
			result += fmt.Sprintf("\n- <font color=\"#ff0000\">**内部nofollow链接：** %d (%.1f%%)</font>", crawl.InternalNoFollowLinks, internalNoFollowRatio*100)
		} else {
			result += fmt.Sprintf("\n- **内部nofollow链接：** %d (%.1f%%)", crawl.InternalNoFollowLinks, internalNoFollowRatio*100)
		}
	}

	result += fmt.Sprintf("\n- **外部链接：** %d", externalTotal)

	// Check external follow ratio
	if externalTotal > 0 {
		externalFollowRatio := float64(crawl.ExternalFollowLinks) / float64(externalTotal)
		if externalFollowRatio > 0.5 {
			result += fmt.Sprintf("\n- <font color=\"#ff0000\">**外部follow链接：** %d (%.1f%%)</font>", crawl.ExternalFollowLinks, externalFollowRatio*100)
		} else {
			result += fmt.Sprintf("\n- **外部follow链接：** %d (%.1f%%)", crawl.ExternalFollowLinks, externalFollowRatio*100)
		}
	}

	return result
}

// formatInlinkStats formats inlink statistics with abnormal indicators
func (s *DingTalkService) formatInlinkStats(stats *models.InlinkStats) string {
	if stats == nil || stats.TotalPages == 0 {
		return ""
	}

	result := "\n### 🔗 内链结构统计\n"
	result += fmt.Sprintf("- **总页面数：** %d", stats.TotalPages)

	// Calculate percentages
	zeroInlinksPercent := float64(stats.ZeroInlinks) / float64(stats.TotalPages) * 100
	lowValueInlinksPercent := float64(stats.LowValueInlinks) / float64(stats.TotalPages) * 100

	// Zero inlinks (isolated pages)
	if zeroInlinksPercent > 10 {
		result += fmt.Sprintf("\n- <font color=\"#ff0000\">**孤岛页面（入链数=0）：** %d (%.1f%%)</font>", stats.ZeroInlinks, zeroInlinksPercent)
	} else if zeroInlinksPercent > 5 {
		result += fmt.Sprintf("\n- <font color=\"#ff0000\">**孤岛页面（入链数=0）：** %d (%.1f%%)</font>", stats.ZeroInlinks, zeroInlinksPercent)
	} else {
		result += fmt.Sprintf("\n- **孤岛页面（入链数=0）：** %d (%.1f%%)", stats.ZeroInlinks, zeroInlinksPercent)
	}

	// Low value inlinks (<= 1)
	if lowValueInlinksPercent > 30 {
		result += fmt.Sprintf("\n- <font color=\"#ff0000\">**低价值入口页面（入链数<=1）：** %d (%.1f%%)</font>", stats.LowValueInlinks, lowValueInlinksPercent)
	} else if lowValueInlinksPercent > 15 {
		result += fmt.Sprintf("\n- <font color=\"#ff0000\">**低价值入口页面（入链数<=1）：** %d (%.1f%%)</font>", stats.LowValueInlinks, lowValueInlinksPercent)
	} else {
		result += fmt.Sprintf("\n- **低价值入口页面（入链数<=1）：** %d (%.1f%%)", stats.LowValueInlinks, lowValueInlinksPercent)
	}

	return result
}

// generateOptimizationSuggestions generates optimization suggestions based on abnormal indicators
func (s *DingTalkService) generateOptimizationSuggestions(
	crawl *models.Crawl,
	imageAltCount *models.AltCount,
	schemeCount *models.SchemeCount,
	statusCodeCount *models.Chart,
	titleStats *models.TitleStats,
	descriptionStats *models.DescriptionStats,
	allMediaTypes []models.MediaTypeDetail,
	statusCodeByDepth []models.StatusCodeByDepth,
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

	// Check for link ratio issues
	internalTotal := crawl.InternalFollowLinks + crawl.InternalNoFollowLinks
	externalTotal := crawl.ExternalFollowLinks + crawl.ExternalNoFollowLinks
	if internalTotal > 0 {
		internalNoFollowRatio := float64(crawl.InternalNoFollowLinks) / float64(internalTotal)
		if internalNoFollowRatio > 0.3 {
			suggestions = append(suggestions, fmt.Sprintf("- **内部nofollow链接：** 发现 %.1f%%%% 的内部链接使用了nofollow属性，超过30%%%%的阈值。这不利于内部权重传递，建议减少内部链接的nofollow使用。", internalNoFollowRatio*100))
		}
	}
	if externalTotal > 0 {
		externalFollowRatio := float64(crawl.ExternalFollowLinks) / float64(externalTotal)
		if externalFollowRatio > 0.5 {
			suggestions = append(suggestions, fmt.Sprintf("- **外部follow链接：** 发现 %.1f%%%% 的外部链接使用了follow属性，超过50%%%%的阈值。这可能导致SEO权重外流，建议为外部链接添加nofollow属性。", externalFollowRatio*100))
		}
	}

	// Check for media type issues
	if len(allMediaTypes) > 0 {
		var totalMediaCount int
		for _, mt := range allMediaTypes {
			totalMediaCount += mt.Count
		}

		for _, mt := range allMediaTypes {
			percentage := float64(mt.Count) / float64(totalMediaCount) * 100

			if mt.MediaType == "image/png" && percentage > 5 {
				if percentage > 10 {
					suggestions = append(suggestions, fmt.Sprintf("- **PNG格式图片：** 发现 %d 个PNG格式页面（%.1f%%），属于高风险异常。照片应该使用JPEG或WebP格式，PNG会带来不必要的体积膨胀，影响页面加载速度。建议将PNG格式的照片转换为JPEG或WebP格式。", mt.Count, percentage))
				} else {
					suggestions = append(suggestions, fmt.Sprintf("- **PNG格式图片：** 发现 %d 个PNG格式页面（%.1f%%）。照片应该使用JPEG或WebP格式，PNG会带来不必要的体积膨胀。建议将PNG格式的照片转换为JPEG或WebP格式，以减小文件体积，提升页面加载速度。", mt.Count, percentage))
				}
			} else if mt.MediaType == "image/gif" && percentage > 5 {
				suggestions = append(suggestions, fmt.Sprintf("- **GIF格式图片：** 发现 %d 个GIF格式页面（%.1f%%）。建议使用WebP或视频格式替代GIF，以获得更好的压缩效果和性能。", mt.Count, percentage))
			} else if mt.MediaType != "text/html" && mt.MediaType != "image/jpeg" && mt.MediaType != "image/jpg" && mt.MediaType != "image/webp" && mt.MediaType != "image/png" && mt.MediaType != "image/gif" && percentage > 10 {
				suggestions = append(suggestions, fmt.Sprintf("- **非HTML媒体类型：** 发现 %d 个%s类型页面（%.1f%%）。请检查这些页面是否应该被搜索引擎索引，如果不需要，建议在robots.txt中阻止或使用noindex标签。", mt.Count, mt.MediaType, percentage))
			}
		}

		// Check for "Other" types (if there are more than 4 media types)
		if len(allMediaTypes) > 4 {
			var otherCount int
			for i := 4; i < len(allMediaTypes); i++ {
				otherCount += allMediaTypes[i].Count
			}
			otherPercentage := float64(otherCount) / float64(totalMediaCount) * 100
			if otherPercentage > 10 {
				otherTypes := ""
				for i := 4; i < len(allMediaTypes) && i < 8; i++ {
					if i > 4 {
						otherTypes += "、"
					}
					otherTypes += allMediaTypes[i].MediaType
				}
				if len(allMediaTypes) > 8 {
					otherTypes += "等"
				}
				suggestions = append(suggestions, fmt.Sprintf("- **其他媒体类型：** 发现 %d 个其他类型页面（%.1f%%），包含：%s。请检查这些媒体类型是否合理，必要时进行优化。", otherCount, otherPercentage, otherTypes))
			}
		}
	}

	// Check for page depth issues
	if len(statusCodeByDepth) > 0 {
		var totalPages int
		var depth5_6, depth7_8 int
		for _, sc := range statusCodeByDepth {
			depthTotal := sc.StatusCode100 + sc.StatusCode200 + sc.StatusCode300 + sc.StatusCode400 + sc.StatusCode500
			if depthTotal > 0 {
				totalPages += depthTotal
				if sc.Depth == 5 || sc.Depth == 6 {
					depth5_6 += depthTotal
				} else if sc.Depth == 7 || sc.Depth == 8 {
					depth7_8 += depthTotal
				}
			}
		}

		if totalPages > 0 {
			depth5_8Percent := float64(depth5_6+depth7_8) / float64(totalPages) * 100
			depth7_8Percent := float64(depth7_8) / float64(totalPages) * 100

			if depth7_8Percent > 10 {
				suggestions = append(suggestions, fmt.Sprintf("- **页面深度过深：** 发现深度7-8层的页面占比为 %.1f%%%%，超过10%%%%的阈值。网站结构过深，不利于搜索引擎爬取和用户体验。建议优化网站结构，减少页面深度。", depth7_8Percent))
			} else if depth5_8Percent > 20 {
				suggestions = append(suggestions, fmt.Sprintf("- **页面深度：** 发现深度5-8层的页面占比为 %.1f%%%%，超过20%%%%的阈值。建议优化网站导航结构，尽量将重要页面控制在3-4层以内。", depth5_8Percent))
			}
		}
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
