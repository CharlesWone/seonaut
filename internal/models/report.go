package models

type CanonicalCount struct {
	Canonical    int
	NonCanonical int
}

type SchemeCount struct {
	HTTP  int
	HTTPS int
}

type AltCount struct {
	Alt    int
	NonAlt int
}

type StatusCodeByDepth struct {
	Depth         int
	StatusCode100 int
	StatusCode200 int
	StatusCode300 int
	StatusCode400 int
	StatusCode500 int
}

type TitleStats struct {
	TotalPages     int
	EmptyTitle     int
	ShortTitle     int
	LongTitle      int
	MultipleTitles int
	DuplicateTitle int
}

type DescriptionStats struct {
	TotalPages           int
	EmptyDescription     int
	ShortDescription     int
	LongDescription      int
	MultipleDescriptions int
	DuplicateDescription int
}

type InlinkStats struct {
	TotalPages      int // 需要被收录的页面总数（HTML页面且状态码200-299）
	ZeroInlinks     int // 入链数 = 0 的页面数量（孤岛页面）
	LowValueInlinks int // 入链数 <= 1 的页面数量（低价值入口页面）
}

type MediaTypeDetail struct {
	MediaType string
	Count     int
}
