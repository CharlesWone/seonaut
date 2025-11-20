package routes

import (
	"github.com/stjudewashere/seonaut/internal/utils"
	"net/http"
	"strconv"
	"strings"

	"github.com/stjudewashere/seonaut/internal/models"
	"github.com/stjudewashere/seonaut/internal/services"
)

type issuesReportHandler struct {
	*services.Container
}

func (h *issuesReportHandler) indexHandler(w http.ResponseWriter, r *http.Request) {
	// 提取路径参数
	pathParam := strings.TrimPrefix(r.URL.Path, h.Config.HTTPServer.ContextPath+"/issuesReport/")
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

	//pid, err := strconv.Atoi(r.URL.Query().Get("pid"))
	//if err != nil {
	//	redirect(w, r, http.StatusSeeOther, h.Container, "/")
	//	return
	//}

	pv, err := h.ProjectViewService.GetProjectViewNoAuth(pid)
	if err != nil {
		redirect(w, r, http.StatusSeeOther, h.Container, "/")
		return
	}

	// 爬虫中？？？
	if pv.Crawl.TotalURLs == 0 {
		redirect(w, r, http.StatusSeeOther, h.Container, "/")
		return
	}

	ig := models.IssuesGroupView{
		ProjectView: pv,
		IssueCount:  h.IssueService.GetIssuesCount(pv.Crawl.Id),
	}

	v := &PageView{
		Lang:      "en",
		Theme:     "dart",
		Data:      ig,
		User:      models.User{},
		PageTitle: "ISSUES_VIEW_PAGE_TITLE",
	}

	h.Renderer.RenderTemplate(w, "issues_report", v, "en")
}

func (h *issuesReportHandler) viewHandler(w http.ResponseWriter, r *http.Request) {
	// 提取路径参数
	pathParam := strings.TrimPrefix(r.URL.Path, h.Config.HTTPServer.ContextPath+"/issuesReport/view/")
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

	eid := r.URL.Query().Get("eid")
	if eid == "" {
		redirect(w, r, http.StatusSeeOther, h.Container, "/")
		return
	}

	page, err := strconv.Atoi(r.URL.Query().Get("p"))
	if err != nil {
		page = 1
	}

	pv, err := h.ProjectViewService.GetProjectViewNoAuth(pid)
	if err != nil {
		redirect(w, r, http.StatusSeeOther, h.Container, "/")
		return
	}

	paginatorView, err := h.IssueService.GetPaginatedReportsByIssue(pv.Crawl.Id, page, eid)
	if err != nil {
		redirect(w, r, http.StatusSeeOther, h.Container, "/")
		return
	}

	data := models.IssuesView{
		ProjectView:   pv,
		Eid:           eid,
		PaginatorView: paginatorView,
	}

	v := &PageView{
		Lang:      "en",
		Theme:     "dart",
		Data:      data,
		User:      models.User{},
		PageTitle: "ISSUES_DETAIL_PAGE_TITLE",
	}

	h.Renderer.RenderTemplate(w, "issues_report_view", v, "en")
}
