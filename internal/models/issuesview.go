package models

type (
	IssuesGroupView struct {
		ProjectView *ProjectView
		IssueCount  *IssueCount
		Crawls      []Crawl
	}

	IssuesView struct {
		ProjectView   *ProjectView
		Eid           string
		PaginatorView PaginatorView
	}
)
