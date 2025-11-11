package routes

import (
	"github.com/stjudewashere/seonaut/internal/models"
	"github.com/stjudewashere/seonaut/internal/services"
	"github.com/stjudewashere/seonaut/internal/utils"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

type crawlReportHandler struct {
	*services.Container
}

func (h *crawlReportHandler) indexHandler(w http.ResponseWriter, r *http.Request) {
	// 提取路径参数
	pathParam := strings.TrimPrefix(r.URL.Path, h.Config.HTTPServer.ContextPath+"/crawlReport/")
	if pathParam == "" || pathParam == "/" {
		http.Error(w, "missing param", http.StatusBadRequest)
		return
	}

	// 解密参数
	idStr, err := utils.DecryptParam(pathParam)
	if err != nil {
		http.Error(w, "invalid param", http.StatusBadRequest)
		return
	}

	// 转为int64
	pid, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid param", http.StatusBadRequest)
		return
	}

	// 获取项目视图：项目和项目最后一次的爬虫信息
	pv, err := h.ProjectViewService.GetProjectViewNoAuth(pid)
	if err != nil {
		http.Error(w, "invalid param", http.StatusBadRequest)
		return
	}

	var html template.HTML
	var archiveExists bool
	if pv.Crawl.Crawling {
		html = "<h2>爬取中...</h2>"
		archiveExists = false
	} else {
		pv.Project.EncryptedId = pathParam
		// 获取爬虫报告 markdown格式
		markdown := h.CrawlReportService.GetCrawlReportMarkdown(&pv.Crawl, &pv.Project)
		// 转换为HTML
		html = h.CrawlReportService.MarkdownToHTML(markdown)
		archiveExists = h.Container.ArchiveService.ArchiveExists(&pv.Project)
	}

	// 渲染模板视图
	h.Renderer.RenderTemplate(w, "crawl_report", &PageView{
		Lang:      "en",
		Theme:     "dart",
		PageTitle: "CRAWL_REPORT_VIEW_PAGE_TITLE", //  国际化 CRAWL_REPORT_VIEW_PAGE_TITLE 作为key
		Data: struct {
			Project       models.Project
			ArchiveExists bool
			Content       template.HTML
			Crawling      bool
		}{
			Project:       pv.Project,
			ArchiveExists: archiveExists,
			Content:       html,
			Crawling:      pv.Crawl.Crawling,
		},
	}, "en")
}
