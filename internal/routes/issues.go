package routes

import (
	"net/http"
	"strconv"

	"github.com/stjudewashere/seonaut/internal/models"
	"github.com/stjudewashere/seonaut/internal/services"
)

type issueHandler struct {
	*services.Container
}

// indexHandler handles the issues view of a project.
// It expects a query parameter "pid" containing the project id.
func (h *issueHandler) indexHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := h.CookieSession.GetUser(r.Context())
	if !ok {
		redirect(w, r, http.StatusSeeOther, h.Container, "/signout")
		return
	}

	pid, err := strconv.Atoi(r.URL.Query().Get("pid"))
	if err != nil {
		redirect(w, r, http.StatusSeeOther, h.Container, "/")
		return
	}

	pv, err := h.ProjectViewService.GetProjectView(pid, user.Id)
	if err != nil {
		redirect(w, r, http.StatusSeeOther, h.Container, "/")
		return
	}

	if pv.Crawl.TotalURLs == 0 {
		redirect(w, r, http.StatusSeeOther, h.Container, "/")
		return
	}

	ig := models.IssuesGroupView{
		ProjectView: pv,
		IssueCount:  h.IssueService.GetIssuesCount(pv.Crawl.Id),
	}

	v := &PageView{
		Lang:      user.Lang,
		Theme:     user.Theme,
		Data:      ig,
		User:      *user,
		PageTitle: "ISSUES_VIEW_PAGE_TITLE",
	}

	h.Renderer.RenderTemplate(w, "issues", v, user.Lang)
}

// viewHandler handles the view of the project's issues by an specific type.
// It expects a query parameter "pid" containing the project id and an "eid" parameter
// containing the issue type.
func (h *issueHandler) viewHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := h.CookieSession.GetUser(r.Context())
	if !ok {
		redirect(w, r, http.StatusSeeOther, h.Container, "/signout")
		return
	}

	eid := r.URL.Query().Get("eid")
	if eid == "" {
		redirect(w, r, http.StatusSeeOther, h.Container, "/")
		return
	}

	pid, err := strconv.Atoi(r.URL.Query().Get("pid"))
	if err != nil {
		redirect(w, r, http.StatusSeeOther, h.Container, "/")
		return
	}

	page, err := strconv.Atoi(r.URL.Query().Get("p"))
	if err != nil {
		page = 1
	}

	pv, err := h.ProjectViewService.GetProjectView(pid, user.Id)
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
		Lang:      user.Lang,
		Theme:     user.Theme,
		Data:      data,
		User:      *user,
		PageTitle: "ISSUES_DETAIL_PAGE_TITLE",
	}

	h.Renderer.RenderTemplate(w, "issues_view", v, user.Lang)
}
