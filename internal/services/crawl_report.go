package services

import (
	"bytes"
	"fmt"
	"github.com/microcosm-cc/bluemonday"
	"github.com/stjudewashere/seonaut/internal/models"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"html/template"
	"math"
	"regexp"
	"strings"
	"time"
)

type CrawlReportService struct {
	dashboardService    *DashboardService
	imageService        *ImageService
	linkService         *LinkService
	externalLinkService *ExternalLinkService
	markdownRender      goldmark.Markdown // ← 接口，不是指针！
}

func NewCrawlReportService(dashboardService *DashboardService, imageService *ImageService, linkService *LinkService, externalLinkService *ExternalLinkService) *CrawlReportService {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // 表格、任务列表、删除线
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // <h1 id="xxx">
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(), // 换行 → <br>
			html.WithUnsafe(),    // 需要嵌入 HTML 时打开
		),
	)

	return &CrawlReportService{
		dashboardService:    dashboardService,
		imageService:        imageService,
		linkService:         linkService,
		externalLinkService: externalLinkService,
		markdownRender:      md, // ← 直接赋值，类型匹配
	}
}

/*
MarkdownToHTML 将 Markdown 文本渲染为 **安全的 HTML**
流程：
 1. goldmark 解析 Markdown → HTML（默认安全，不渲染 <script>）
 2. bluemonday 清洗 HTML → 移除危险标签/属性（如 script, onclick, onerror）

为什么要清洗？

即使 goldmark 默认安全，但：
  - 未来可能开启 html.WithUnsafe() 允许嵌入 HTML
  - 内容来自爬虫、数据库、用户输入（不可信）
  - 防止 XSS 攻击：如 <img src=x onerror=alert(1)>、<script>alert(1)</script>

bluemonday 是“零信任”原则：**任何 HTML 输出都必须清洗**

安全策略：
  - 使用 UGCPolicy()：允许常见标签（p, h1-h6, ul, code, a, img, table 等）
  - 允许代码高亮 class（如 language-go）
  - 允许外部链接安全打开（target="_blank" + rel="noopener noreferrer"）
*/
func (s *CrawlReportService) MarkdownToHTML(md string) template.HTML {
	var buf bytes.Buffer

	// Step 1: 渲染 Markdown → HTML
	// goldmark 默认不渲染原始 HTML（如 <script>），但仍需清洗
	if err := s.markdownRender.Convert([]byte(md), &buf); err != nil {
		// 渲染失败返回空，避免模板崩溃
		return template.HTML("")
	}

	// Step 2: bluemonday 严格清洗，防止 XSS
	p := bluemonday.UGCPolicy() // 预设安全策略：只允许用户生成内容（UGC）常见标签
	// 允许代码高亮使用的 class（如 class="language-go"）
	// goldmark 生成单个 class，格式为 "language-xxx"
	// 精确匹配 "_blank"
	p.AllowAttrs("target").Matching(regexp.MustCompile(`^_blank$`)).OnElements("a")

	// 精确匹配 "noopener noreferrer"
	p.AllowAttrs("rel").Matching(regexp.MustCompile(`^noopener noreferrer$`)).OnElements("a")

	// 允许 class="language-xxx"
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^language-[a-zA-Z0-9+.#*-]+$`)).OnElements("code", "pre")

	// Step 3: 清洗后返回 template.HTML（Go 模板不会二次转义）
	safeHTML := p.SanitizeBytes(buf.Bytes())
	return template.HTML(safeHTML)
}

func (s *CrawlReportService) GetCrawlReportMarkdown(crawl *models.Crawl, project *models.Project) string {
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
	allMediaTypes := s.dashboardService.GetMediaTypeDetails(crawl.Id)
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

// formatLinkStats formats link statistics with abnormal indicators
func (s *CrawlReportService) formatLinkStats(crawl *models.Crawl) string {
	internalTotal := crawl.InternalFollowLinks + crawl.InternalNoFollowLinks
	externalTotal := crawl.ExternalFollowLinks + crawl.ExternalNoFollowLinks

	result := fmt.Sprintf("- **内部链接：** %d", internalTotal)

	// Check internal nofollow ratio
	if internalTotal > 0 {
		internalNoFollowRatio := float64(crawl.InternalNoFollowLinks) / float64(internalTotal)
		if internalNoFollowRatio > 0.3 {
			result += fmt.Sprintf("\n- <font color=\"red\">**内部nofollow链接：** %d (%.1f%%)</font>", crawl.InternalNoFollowLinks, internalNoFollowRatio*100)
		} else {
			result += fmt.Sprintf("\n- **内部nofollow链接：** %d (%.1f%%)", crawl.InternalNoFollowLinks, internalNoFollowRatio*100)
		}
	}

	result += fmt.Sprintf("\n- **外部链接：** %d", externalTotal)

	// Check external follow ratio
	if externalTotal > 0 {
		externalFollowRatio := float64(crawl.ExternalFollowLinks) / float64(externalTotal)
		if externalFollowRatio > 0.5 {
			result += fmt.Sprintf("\n- <font color=\"red\">**外部follow链接：** %d (%.1f%%)</font>", crawl.ExternalFollowLinks, externalFollowRatio*100)
		} else {
			result += fmt.Sprintf("\n- **外部follow链接：** %d (%.1f%%)", crawl.ExternalFollowLinks, externalFollowRatio*100)
		}
	}

	return result
}

// formatInlinkStats formats inlink statistics with abnormal indicators
func (s *CrawlReportService) formatInlinkStats(stats *models.InlinkStats) string {
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
		result += fmt.Sprintf("\n- <font color=\"red\">**孤岛页面（入链数=0）：** %d (%.1f%%)</font>", stats.ZeroInlinks, zeroInlinksPercent)
	} else if zeroInlinksPercent > 5 {
		result += fmt.Sprintf("\n- <font color=\"red\">**孤岛页面（入链数=0）：** %d (%.1f%%)</font>", stats.ZeroInlinks, zeroInlinksPercent)
	} else {
		result += fmt.Sprintf("\n- **孤岛页面（入链数=0）：** %d (%.1f%%)", stats.ZeroInlinks, zeroInlinksPercent)
	}

	// Low value inlinks (<= 1)
	if lowValueInlinksPercent > 30 {
		result += fmt.Sprintf("\n- <font color=\"red\">**低价值入口页面（入链数<=1）：** %d (%.1f%%)</font>", stats.LowValueInlinks, lowValueInlinksPercent)
	} else if lowValueInlinksPercent > 15 {
		result += fmt.Sprintf("\n- <font color=\"red\">**低价值入口页面（入链数<=1）：** %d (%.1f%%)</font>", stats.LowValueInlinks, lowValueInlinksPercent)
	} else {
		result += fmt.Sprintf("\n- **低价值入口页面（入链数<=1）：** %d (%.1f%%)", stats.LowValueInlinks, lowValueInlinksPercent)
	}

	return result
}

// generateOptimizationSuggestions generates optimization suggestions based on abnormal indicators
func (s *CrawlReportService) generateOptimizationSuggestions(
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

			// 获取没有alt属性的图片url
			urls := s.imageService.GetWithoutAltImageURLs(crawl.Id)
			// 图片 URL 列表（用代码块包裹，保持整齐）
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
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
			// 获取http协议的页面
			urls := s.dashboardService.GetPageURLsByScheme(crawl.Id, "http")
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
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
			// 获取状态码4开头的页面
			urls := s.dashboardService.GetPageURLsByStatusCodeRange(crawl.Id, 400, 499)
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
		}
		if error5xx > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **5xx错误：** 发现 %d 个页面返回5xx服务器错误。请立即检查服务器配置和代码，修复服务器端问题。", error5xx))
			// 获取状态码5开头的页面
			urls := s.dashboardService.GetPageURLsByStatusCodeRange(crawl.Id, 500, 599)
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
		}
	}

	// Check for title issues
	if titleStats.TotalPages > 0 {
		if titleStats.EmptyTitle > 0 {
			percentage := float64(titleStats.EmptyTitle) / float64(titleStats.TotalPages) * 100
			suggestions = append(suggestions, fmt.Sprintf("- **空标题：** 发现 %d 个页面（%.1f%%）缺少标题标签。建议为所有页面添加唯一且描述性的标题，长度控制在20-60字符之间。", titleStats.EmptyTitle, percentage))
			// 获取空标题的页面url
			urls := s.dashboardService.GetWithoutTitlePageURLs(crawl.Id)
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
		}
		if titleStats.ShortTitle > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **短标题：** 发现 %d 个页面标题过短（<20字符）。建议优化标题，使其更具体、更具描述性，以提升SEO效果。", titleStats.ShortTitle))
			// 获取短标题的页面url
			urls := s.dashboardService.GetPageURLsByTitleLength(crawl.Id, 1, 19)
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
		}
		if titleStats.LongTitle > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **长标题：** 发现 %d 个页面标题过长（>60字符）。建议将标题长度控制在60字符以内，避免在搜索结果中被截断。", titleStats.LongTitle))
			// 获取长标题的页面url
			urls := s.dashboardService.GetPageURLsByTitleLength(crawl.Id, 60, 1000)
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
		}
		if titleStats.MultipleTitles > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **多标题标签：** 发现 %d 个页面存在多个标题标签。每个页面应只有一个标题标签，请删除多余的标题标签。", titleStats.MultipleTitles))
			// 获取多标题的页面url
			urls := s.dashboardService.GetMultipleTitlesPageURLs(crawl.Id)
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
		}
		if titleStats.DuplicateTitle > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **重复标题：** 发现 %d 个页面使用重复标题。建议为每个页面创建唯一标题，提升页面区分度和SEO效果。", titleStats.DuplicateTitle))
			// 后续优化为一条sql
			// 获取重复标题
			titles := s.dashboardService.GetDuplicateTitle(crawl.Id)
			for _, title := range titles {
				// 根据标题查url
				urls := s.dashboardService.GetDuplicateTitleURLsByTitle(crawl.Id, title)
				if len(urls) > 0 {
					var lines []string
					for _, url := range urls {
						lines = append(lines, "  🚩 "+title+" "+url)
					}
					suggestions = append(suggestions,
						"```text\n"+
							strings.Join(lines, "\n")+"\n"+
							"```",
					)
				}
			}
		}
	}

	// Check for description issues
	if descriptionStats.TotalPages > 0 {
		if descriptionStats.EmptyDescription > 0 {
			percentage := float64(descriptionStats.EmptyDescription) / float64(descriptionStats.TotalPages) * 100
			suggestions = append(suggestions, fmt.Sprintf("- **空描述：** 发现 %d 个页面（%.1f%%）缺少meta描述。建议为所有页面添加吸引人的描述，长度控制在80-160字符之间。", descriptionStats.EmptyDescription, percentage))
			// 获取空描述的页面url
			urls := s.dashboardService.GetWithoutDescriptionPageURLs(crawl.Id)
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
		}
		if descriptionStats.ShortDescription > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **短描述：** 发现 %d 个页面描述过短（<80字符）。建议优化描述内容，使其更详细、更具吸引力。", descriptionStats.ShortDescription))
			// 获取短描述的页面url
			urls := s.dashboardService.GetPageURLsByDescriptionLength(crawl.Id, 1, 79)
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
		}
		if descriptionStats.LongDescription > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **长描述：** 发现 %d 个页面描述过长（>160字符）。建议将描述长度控制在160字符以内，避免在搜索结果中被截断。", descriptionStats.LongDescription))
			// 获取长描述的页面url
			urls := s.dashboardService.GetPageURLsByDescriptionLength(crawl.Id, 161, math.MaxInt64)
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
		}
		if descriptionStats.MultipleDescriptions > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **多描述标签：** 发现 %d 个页面存在多个meta描述标签。每个页面应只有一个描述标签，请删除多余的描述标签。", descriptionStats.MultipleDescriptions))
			// 获取多描述的页面url
			urls := s.dashboardService.GetMultipleDescriptionPageURLs(crawl.Id)
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
		}
		if descriptionStats.DuplicateDescription > 0 {
			suggestions = append(suggestions, fmt.Sprintf("- **重复描述：** 发现 %d 个页面使用重复描述。建议为每个页面创建唯一描述，提升页面吸引力。", descriptionStats.DuplicateDescription))
			// 后续优化为一条sql
			// 获取重复的描述
			descriptions := s.dashboardService.GetDuplicateDescription(crawl.Id)
			for _, description := range descriptions {
				// 根据描述查url
				urls := s.dashboardService.GetDuplicateDescriptionURLsByDescription(crawl.Id, description)
				if len(urls) > 0 {
					var lines []string
					for _, url := range urls {
						lines = append(lines, "  🚩 "+description+" "+url)
					}
					suggestions = append(suggestions,
						"```text\n"+
							strings.Join(lines, "\n")+"\n"+
							"```",
					)
				}
			}
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
			// 查询使用了nofollow的内部链接
			urls := s.linkService.GetLinkURLsByNofollow(crawl.Id, 1)
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
		}
	}
	if externalTotal > 0 {
		externalFollowRatio := float64(crawl.ExternalFollowLinks) / float64(externalTotal)
		if externalFollowRatio > 0.5 {
			suggestions = append(suggestions, fmt.Sprintf("- **外部follow链接：** 发现 %.1f%%%% 的外部链接使用了follow属性，超过50%%%%的阈值。这可能导致SEO权重外流，建议为外部链接添加nofollow属性。", externalFollowRatio*100))
			// 查询nofollow为 0 的外部链接
			urls := s.externalLinkService.GetExternalLinkURLsByNofollow(crawl.Id, 0)
			if len(urls) > 0 {
				var lines []string
				for _, url := range urls {
					lines = append(lines, "  🚩 "+url)
				}
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(lines, "\n")+"\n"+
						"```",
				)
			}
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
					// 查询 媒体类型为 image/png 的url
					urls := s.dashboardService.GetURLsByMediaType(crawl.Id, mt.MediaType)
					if len(urls) > 0 {
						var lines []string
						for _, url := range urls {
							lines = append(lines, "  🚩 "+url)
						}
						suggestions = append(suggestions,
							"```text\n"+
								strings.Join(lines, "\n")+"\n"+
								"```",
						)
					}
				} else {
					suggestions = append(suggestions, fmt.Sprintf("- **PNG格式图片：** 发现 %d 个PNG格式页面（%.1f%%）。照片应该使用JPEG或WebP格式，PNG会带来不必要的体积膨胀。建议将PNG格式的照片转换为JPEG或WebP格式，以减小文件体积，提升页面加载速度。", mt.Count, percentage))
					// 查询 媒体类型为 image/png 的url
					urls := s.dashboardService.GetURLsByMediaType(crawl.Id, mt.MediaType)
					if len(urls) > 0 {
						var lines []string
						for _, url := range urls {
							lines = append(lines, "  🚩 "+url)
						}
						suggestions = append(suggestions,
							"```text\n"+
								strings.Join(lines, "\n")+"\n"+
								"```",
						)
					}
				}
			} else if mt.MediaType == "image/gif" && percentage > 5 {
				suggestions = append(suggestions, fmt.Sprintf("- **GIF格式图片：** 发现 %d 个GIF格式页面（%.1f%%）。建议使用WebP或视频格式替代GIF，以获得更好的压缩效果和性能。", mt.Count, percentage))
				// 查询 媒体类型为 image/gif 的url
				urls := s.dashboardService.GetURLsByMediaType(crawl.Id, mt.MediaType)
				if len(urls) > 0 {
					var lines []string
					for _, url := range urls {
						lines = append(lines, "  🚩 "+url)
					}
					suggestions = append(suggestions,
						"```text\n"+
							strings.Join(lines, "\n")+"\n"+
							"```",
					)
				}
			} else if mt.MediaType != "text/html" && mt.MediaType != "image/jpeg" && mt.MediaType != "image/jpg" && mt.MediaType != "image/webp" && mt.MediaType != "image/png" && mt.MediaType != "image/gif" && percentage > 10 {
				suggestions = append(suggestions, fmt.Sprintf("- **非HTML媒体类型：** 发现 %d 个%s类型页面（%.1f%%）。请检查这些页面是否应该被搜索引擎索引，如果不需要，建议在robots.txt中阻止或使用noindex标签。", mt.Count, mt.MediaType, percentage))
				// 根据媒体类型查询
				urls := s.dashboardService.GetURLsByMediaType(crawl.Id, mt.MediaType)
				if len(urls) > 0 {
					var lines []string
					for _, url := range urls {
						lines = append(lines, "  🚩 "+url)
					}
					suggestions = append(suggestions,
						"```text\n"+
							strings.Join(lines, "\n")+"\n"+
							"```",
					)
				}
			}
		}

		// Check for "Other" types (if there are more than 4 media types)
		if len(allMediaTypes) > 4 {
			var otherCount int
			for i := 4; i < len(allMediaTypes); i++ {
				otherCount += allMediaTypes[i].Count
			}
			otherPercentage := float64(otherCount) / float64(totalMediaCount) * 100

			// 超过10%
			if otherPercentage > 10 {
				otherTypes := ""
				// 拼接媒体类型，注意只到了8
				for i := 4; i < len(allMediaTypes) && i < 8; i++ {
					if i > 4 {
						otherTypes += "、"
					}
					otherTypes += allMediaTypes[i].MediaType
				}
				if len(allMediaTypes) > 8 {
					otherTypes += "等"
				}

				//  url拼接
				var allOtherUrls []string
				for i := 4; i < len(allMediaTypes); i++ {
					// 根据媒体类型查询
					urls := s.dashboardService.GetURLsByMediaType(crawl.Id, allMediaTypes[i].MediaType)
					if len(urls) > 0 {
						var lines []string
						for _, url := range urls {
							lines = append(lines, "  🚩 "+url)
						}
						allOtherUrls = append(allOtherUrls, lines...)
					}
				}

				suggestions = append(suggestions, fmt.Sprintf("- **其他媒体类型：** 发现 %d 个其他类型页面（%.1f%%），包含：%s。请检查这些媒体类型是否合理，必要时进行优化。", otherCount, otherPercentage, otherTypes))
				suggestions = append(suggestions,
					"```text\n"+
						strings.Join(allOtherUrls, "\n")+"\n"+
						"```",
				)
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
				// 根据深度查询url
				urls := s.dashboardService.GetURLsByDepthRange(crawl.Id, 7, 8)
				if len(urls) > 0 {
					var lines []string
					for _, url := range urls {
						lines = append(lines, "  🚩 "+url)
					}
					suggestions = append(suggestions,
						"```text\n"+
							strings.Join(lines, "\n")+"\n"+
							"```",
					)
				}
			} else if depth5_8Percent > 20 {
				suggestions = append(suggestions, fmt.Sprintf("- **页面深度：** 发现深度5-8层的页面占比为 %.1f%%%%，超过20%%%%的阈值。建议优化网站导航结构，尽量将重要页面控制在3-4层以内。", depth5_8Percent))
				// 根据深度查询url
				urls := s.dashboardService.GetURLsByDepthRange(crawl.Id, 5, 8)
				if len(urls) > 0 {
					var lines []string
					for _, url := range urls {
						lines = append(lines, "  🚩 "+url)
					}
					suggestions = append(suggestions,
						"```text\n"+
							strings.Join(lines, "\n")+"\n"+
							"```",
					)
				}
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

// formatMediaTypeStats formats media type statistics with abnormal indicators
func (s *CrawlReportService) formatMediaTypeStats(chart *models.Chart, allMediaTypes []models.MediaTypeDetail) string {
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
			result += fmt.Sprintf("\n- <font color=\"red\">**%s：** %d (%.1f%%)</font>", item.Key, item.Value, percentage)
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
func (s *CrawlReportService) formatStatusCodeStats(chart *models.Chart) string {
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
func (s *CrawlReportService) formatStatusCodeByDepthStats(statusCodes []models.StatusCodeByDepth) string {
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
		result += fmt.Sprintf("\n- **深度5-6：** <font color=\"red\">%d页 (%.1f%%)</font>", depth5_6, depth5_6Percent)
	} else {
		result += fmt.Sprintf("\n- **深度5-6：** %d页 (%.1f%%)", depth5_6, depth5_6Percent)
	}

	// Format depth 7-8 with red if abnormal
	if depth7_8Abnormal {
		result += fmt.Sprintf("\n- **深度7-8：** <font color=\"red\">%d页 (%.1f%%)</font>", depth7_8, depth7_8Percent)
	} else {
		result += fmt.Sprintf("\n- **深度7-8：** %d页 (%.1f%%)", depth7_8, depth7_8Percent)
	}

	return result
}
