package routes

import (
	"github.com/stjudewashere/seonaut/internal/models"
	"github.com/stjudewashere/seonaut/internal/services"
	"html/template"
	"net/http"
	"strconv"
)

type crawlReportHandler struct {
	*services.Container
}

func (h *crawlReportHandler) indexHandler(w http.ResponseWriter, r *http.Request) {
	// 解析请求参数
	pid, err := strconv.Atoi(r.URL.Query().Get("pid"))
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// 获取项目视图：项目和项目最后一次的爬虫信息
	pv, err := h.ProjectViewService.GetProjectViewNoAuth(pid)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// 获取爬虫报告 markdown格式
	markdown := h.CrawlReportService.GetCrawlReportMarkdown(&pv.Crawl, &pv.Project)
	// 转换为HTML
	html := h.CrawlReportService.MarkdownToHTML(markdown)

	archiveExists := h.Container.ArchiveService.ArchiveExists(&pv.Project)

	// 渲染模板视图
	h.Renderer.RenderTemplate(w, "crawl_report", &PageView{
		Lang:      "en",
		Theme:     "dart",
		PageTitle: "CRAWL_REPORT_VIEW_PAGE_TITLE", //  国际化 CRAWL_REPORT_VIEW_PAGE_TITLE 作为key
		Data: struct {
			Project       models.Project
			ArchiveExists bool
			Content       template.HTML
		}{
			Project:       pv.Project,
			ArchiveExists: archiveExists,
			Content:       html,
		},
	}, "en")
}
