package services

import (
	"github.com/stjudewashere/seonaut/internal/models"
)

const (
	chartLimit = 4
)

type (
	DashboardServiceRepository interface {
		CountByMediaType(int64) *models.CountList
		CountByStatusCode(int64) *models.CountList

		CountByCanonical(int64) int
		CountImagesAlt(int64) *models.AltCount
		CountScheme(int64) *models.SchemeCount
		CountByNonCanonical(int64) int
		GetStatusCodeByDepth(crawlId int64) []models.StatusCodeByDepth
		GetTitleStats(crawlId int64) *models.TitleStats
		GetDescriptionStats(crawlId int64) *models.DescriptionStats
		GetInlinkStats(crawlId int64) *models.InlinkStats
		GetMediaTypeDetails(crawlId int64) []models.MediaTypeDetail
		GetPageURLsByScheme(crawlId int64, scheme string) []string
		GetPageURLsByStatusCodeRange(crawlId int64, left int64, right int64) []string
		GetWithoutTitlePageURLs(crawlId int64) []string
		GetPageURLsByTitleLength(crawlId int64, minLength int64, maxLength int64) []string
		GetMultipleTitlesPageURLs(crawlId int64) []string
		GetDuplicateTitle(crawlId int64) []string
		GetDuplicateTitleURLsByTitle(crawlId int64, title string) []string
		GetWithoutDescriptionPageURLs(crawlId int64) []string
		GetPageURLsByDescriptionLength(crawlId int64, minLength int64, maxLength int64) []string
		GetMultipleDescriptionPageURLs(crawlId int64) []string
		GetDuplicateDescription(crawlId int64) []string
		GetDuplicateDescriptionURLsByDescription(crawlId int64, description string) []string
		GetURLsByMediaType(crawlId int64, mediaType string) []string
		GetNonHTMLMediaURLs(crawlId int64) []string
		GetURLsByDepthRange(crawlId int64, left int, right int) []string
	}

	DashboardService struct {
		repository DashboardServiceRepository
	}
)

func NewDashboardService(r DashboardServiceRepository) *DashboardService {
	return &DashboardService{repository: r}
}

// Returns a Chart with the PageReport's media type chart data.
func (s *DashboardService) GetMediaCount(crawlId int64) *models.Chart {
	v := s.repository.CountByMediaType(crawlId)
	return newChart(v)
}

// Returns a Chart with the PageReport's status code chart data.
func (s *DashboardService) GetStatusCount(crawlId int64) *models.Chart {
	v := s.repository.CountByStatusCode(crawlId)
	return newChart(v)
}

// Returns the count Images with and without the alt attribute.
func (s *DashboardService) GetImageAltCount(crawlId int64) *models.AltCount {
	return s.repository.CountImagesAlt(crawlId)
}

// Returns the count of PageReports with and without https.
func (s *DashboardService) GetSchemeCount(crawlId int64) *models.SchemeCount {
	return s.repository.CountScheme(crawlId)
}

// Returns a count of PageReports that are canonical or not.
func (s *DashboardService) GetCanonicalCount(crawlId int64) *models.CanonicalCount {
	return &models.CanonicalCount{
		Canonical:    s.repository.CountByCanonical(crawlId),
		NonCanonical: s.repository.CountByNonCanonical(crawlId),
	}
}

// GetStatusCodeByDepth returns a slice of StatusCodeByDepth models with the total number of
// pagereports by depth and status code.
func (s *DashboardService) GetStatusCodeByDepth(crawlId int64) []models.StatusCodeByDepth {
	return s.repository.GetStatusCodeByDepth(crawlId)
}

// GetTitleStats returns title statistics for a crawl
func (s *DashboardService) GetTitleStats(crawlId int64) *models.TitleStats {
	return s.repository.GetTitleStats(crawlId)
}

// GetDescriptionStats returns description statistics for a crawl
func (s *DashboardService) GetDescriptionStats(crawlId int64) *models.DescriptionStats {
	return s.repository.GetDescriptionStats(crawlId)
}

// GetInlinkStats returns statistics about pages with zero or low inlinks
func (s *DashboardService) GetInlinkStats(crawlId int64) *models.InlinkStats {
	return s.repository.GetInlinkStats(crawlId)
}

// GetMediaTypeDetails returns all media types with their counts (not limited by chartLimit)
func (s *DashboardService) GetMediaTypeDetails(crawlId int64) []models.MediaTypeDetail {
	return s.repository.GetMediaTypeDetails(crawlId)
}

// Returns a Chart containing the keys and values from the CountList.
// It limits the slice to the chartLimit value.
func newChart(c *models.CountList) *models.Chart {
	chart := models.Chart{}
	total := 0

	for _, i := range *c {
		total = total + i.Value
	}

	for _, i := range *c {
		ci := models.ChartItem(i)
		chart = append(chart, ci)
	}

	if len(chart) > chartLimit {
		chart[chartLimit-1].Key = "Other"
		for _, v := range chart[chartLimit:] {
			chart[chartLimit-1].Value += v.Value
		}

		chart = chart[:chartLimit]
	}

	return &chart
}

//func (s *DashboardService) GetWithoutAltImageURLs(crawlId int64) []string {
//	return s.repository.GetWithoutAltImageURLs(crawlId)
//}

func (s *DashboardService) GetPageURLsByScheme(crawlId int64, scheme string) []string {
	return s.repository.GetPageURLsByScheme(crawlId, scheme)
}

func (s *DashboardService) GetPageURLsByStatusCodeRange(crawlId int64, left int64, right int64) []string {
	return s.repository.GetPageURLsByStatusCodeRange(crawlId, left, right)
}

func (s *DashboardService) GetWithoutTitlePageURLs(crawlId int64) []string {
	return s.repository.GetWithoutTitlePageURLs(crawlId)
}

func (s *DashboardService) GetPageURLsByTitleLength(crawlId int64, minLength int64, maxLength int64) []string {
	return s.repository.GetPageURLsByTitleLength(crawlId, minLength, maxLength)
}

func (s *DashboardService) GetMultipleTitlesPageURLs(crawlId int64) []string {
	return s.repository.GetMultipleTitlesPageURLs(crawlId)
}

func (s *DashboardService) GetDuplicateTitle(crawlId int64) []string {
	return s.repository.GetDuplicateTitle(crawlId)
}

func (s *DashboardService) GetDuplicateTitleURLsByTitle(crawlId int64, title string) []string {
	return s.repository.GetDuplicateTitleURLsByTitle(crawlId, title)
}

func (s *DashboardService) GetWithoutDescriptionPageURLs(crawlId int64) []string {
	return s.repository.GetWithoutDescriptionPageURLs(crawlId)
}

func (s *DashboardService) GetPageURLsByDescriptionLength(crawlId int64, minLength int64, maxLength int64) []string {
	return s.repository.GetPageURLsByDescriptionLength(crawlId, minLength, maxLength)
}

func (s *DashboardService) GetMultipleDescriptionPageURLs(crawlId int64) []string {
	return s.repository.GetMultipleDescriptionPageURLs(crawlId)
}

func (s *DashboardService) GetDuplicateDescription(crawlId int64) []string {
	return s.repository.GetDuplicateDescription(crawlId)
}

func (s *DashboardService) GetDuplicateDescriptionURLsByDescription(crawlId int64, description string) []string {
	return s.repository.GetDuplicateDescriptionURLsByDescription(crawlId, description)
}

func (s *DashboardService) GetURLsByMediaType(crawlId int64, mediaType string) []string {
	return s.repository.GetURLsByMediaType(crawlId, mediaType)
}

func (s *DashboardService) GetNonHTMLMediaURLs(crawlId int64) []string {
	return s.repository.GetNonHTMLMediaURLs(crawlId)
}

func (s *DashboardService) GetURLsByDepthRange(crawlId int64, left int, right int) []string {
	return s.repository.GetURLsByDepthRange(crawlId, left, right)
}
