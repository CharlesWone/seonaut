package repository

import (
	"database/sql"
	"log"
	"sort"

	"github.com/stjudewashere/seonaut/internal/models"
)

type DashboardRepository struct {
	DB *sql.DB
}

// CountByCanonical returns the number of pagereports in a crawl that are of type "text/html"
// and have an empty canonical or a canonical pointing to its own url.
func (ds *DashboardRepository) CountByCanonical(cid int64) int {
	query := `
		SELECT
			count(id)
		FROM pagereports 
		WHERE crawl_id = ? AND media_type = "text/html" AND (canonical = "" OR canonical = url)
			AND status_code >= 200 AND status_code < 300
	`

	row := ds.DB.QueryRow(query, cid)
	var c int
	if err := row.Scan(&c); err != nil {
		log.Printf("CountByCanonical: %v\n", err)
	}

	return c
}

// CountByNonCanonical returns the number of pagereports in a crawl that are of type "text/html"
// and have a non empty canonical or a canonical pointing to a different url.
func (ds *DashboardRepository) CountByNonCanonical(cid int64) int {
	query := `
		SELECT
			count(id)
		FROM pagereports 
		WHERE crawl_id = ? AND media_type = "text/html" AND canonical != "" AND canonical != url
			AND status_code >= 200 AND status_code < 300
	`

	row := ds.DB.QueryRow(query, cid)
	var c int
	if err := row.Scan(&c); err != nil {
		log.Printf("CountByNonCanonical: %v\n", err)
	}

	return c
}

// CountImagesAlt returns an AltCount model with the total number of images that have an alt attribute
// and the total number of images that don't.
func (ds *DashboardRepository) CountImagesAlt(cid int64) *models.AltCount {
	query := `
		SELECT 
			if(alt = "", "no alt", "alt") as a,
			count(*)
		FROM images
		WHERE crawl_id = ?
		GROUP BY a
	`

	c := &models.AltCount{}

	rows, err := ds.DB.Query(query, cid)
	if err != nil {
		log.Println(err)
		return c
	}

	for rows.Next() {
		var v int
		var a string

		err := rows.Scan(&a, &v)
		if err != nil {
			continue
		}

		if a == "alt" {
			c.Alt = v
		} else {
			c.NonAlt = v
		}
	}

	return c
}

// CountScheme returns an SchemeCount model with the total number of pagereports that use the
// http and the total number of pagereports that use https.
func (ds *DashboardRepository) CountScheme(cid int64) *models.SchemeCount {
	query := `
		SELECT
			scheme,
			count(*)
		FROM pagereports
		WHERE crawl_id = ?
		GROUP BY scheme
	`

	c := &models.SchemeCount{}

	rows, err := ds.DB.Query(query, cid)
	if err != nil {
		log.Println(err)
		return c
	}

	for rows.Next() {
		var v int
		var a string

		err := rows.Scan(&a, &v)
		if err != nil {
			continue
		}

		if a == "https" {
			c.HTTPS = v
		} else {
			c.HTTP = v
		}
	}

	return c
}

// CountByMediaType returns a CountList model with the total number of pagereports by media type.
func (ds *DashboardRepository) CountByMediaType(cid int64) *models.CountList {
	query := `
		SELECT media_type, count(*)
		FROM pagereports
		WHERE crawl_id = ? AND crawled = 1
		GROUP BY media_type`

	return ds.countListQuery(query, cid)
}

// CountByStatusCode returns a CountList model with the total number of pagereports by status code.
func (ds *DashboardRepository) CountByStatusCode(cid int64) *models.CountList {
	query := `
		SELECT
			status_code,
			count(*)
		FROM pagereports
		WHERE crawl_id = ? AND crawled = 1
		GROUP BY status_code`

	return ds.countListQuery(query, cid)
}

// countListQuery is a helper function used to build the CountList model.
func (ds *DashboardRepository) countListQuery(query string, cid int64) *models.CountList {
	m := models.CountList{}
	rows, err := ds.DB.Query(query, cid)
	if err != nil {
		log.Println(err)
		return &m
	}

	for rows.Next() {
		c := models.CountItem{}
		err := rows.Scan(&c.Key, &c.Value)
		if err != nil {
			log.Println(err)
			continue
		}
		m = append(m, c)
	}

	sort.Sort(sort.Reverse(m))

	return &m
}

// GetStatusCodeByDepth returns a slice of StatusCodeByDepth models with the total number of
// pagereports by depth and status code.
func (ds *DashboardRepository) GetStatusCodeByDepth(cid int64) []models.StatusCodeByDepth {
	query := `
	SELECT
		d.depth,
		COALESCE(SUM(CASE WHEN pr.status_code BETWEEN 0 AND 199 THEN 1 ELSE 0 END), 0) AS status_0_to_199,
		COALESCE(SUM(CASE WHEN pr.status_code BETWEEN 200 AND 299 THEN 1 ELSE 0 END), 0) AS status_200_to_299,
		COALESCE(SUM(CASE WHEN pr.status_code BETWEEN 300 AND 399 THEN 1 ELSE 0 END), 0) AS status_300_to_399,
		COALESCE(SUM(CASE WHEN pr.status_code BETWEEN 400 AND 499 THEN 1 ELSE 0 END), 0) AS status_400_to_499,
		COALESCE(SUM(CASE WHEN pr.status_code >= 500 THEN 1 ELSE 0 END), 0) AS status_500_and_above
	FROM
		(SELECT 1 AS depth
		UNION SELECT 2
		UNION SELECT 3
		UNION SELECT 4
		UNION SELECT 5
		UNION SELECT 6
		UNION SELECT 7
		UNION SELECT 8) d
	LEFT JOIN pagereports pr ON pr.depth = d.depth AND pr.crawl_id = ?
	GROUP BY d.depth
	ORDER BY d.depth;
	`

	s := []models.StatusCodeByDepth{}

	rows, err := ds.DB.Query(query, cid)
	if err != nil {
		log.Println(err)
		return s
	}

	for rows.Next() {
		c := models.StatusCodeByDepth{}
		err := rows.Scan(&c.Depth, &c.StatusCode100, &c.StatusCode200, &c.StatusCode300, &c.StatusCode400, &c.StatusCode500)
		if err != nil {
			log.Println(err)
			continue
		}
		s = append(s, c)
	}

	return s
}

// GetTitleStats returns title statistics for a crawl
func (ds *DashboardRepository) GetTitleStats(cid int64) *models.TitleStats {
	query := `
		SELECT
			COUNT(*) as total_pages,
			SUM(CASE WHEN title = '' OR title IS NULL THEN 1 ELSE 0 END) as empty_title,
			SUM(CASE WHEN LENGTH(title) > 0 AND LENGTH(title) < 20 THEN 1 ELSE 0 END) as short_title,
			SUM(CASE WHEN LENGTH(title) > 60 THEN 1 ELSE 0 END) as long_title,
			SUM(CASE WHEN title_count > 1 THEN 1 ELSE 0 END) as multiple_titles
		FROM (
			SELECT 
				pr.*,
				(SELECT COUNT(*) FROM pagereports pr2 
				 WHERE pr2.crawl_id = pr.crawl_id 
				 AND pr2.title = pr.title 
				 AND pr2.media_type = 'text/html' 
				 AND pr2.status_code >= 200 
				 AND pr2.status_code < 300 
				 AND (pr2.canonical = '' OR pr2.canonical = pr2.url) 
				 AND pr2.crawled = 1) as title_count
			FROM pagereports pr
			WHERE pr.crawl_id = ? 
			AND pr.media_type = 'text/html' 
			AND pr.status_code >= 200 
			AND pr.status_code < 300 
			AND (pr.canonical = '' OR pr.canonical = pr.url) 
			AND pr.crawled = 1
		) as pr_with_counts
	`

	stats := &models.TitleStats{}
	row := ds.DB.QueryRow(query, cid)

	var duplicateTitle int
	err := row.Scan(&stats.TotalPages, &stats.EmptyTitle, &stats.ShortTitle, &stats.LongTitle, &stats.MultipleTitles)
	if err != nil {
		log.Printf("GetTitleStats: %v\n", err)
		return stats
	}

	// Get duplicate title count separately
	duplicateQuery := `
		SELECT COUNT(*) as duplicate_count
		FROM (
			SELECT title, COUNT(*) as cnt
			FROM pagereports
			WHERE crawl_id = ? 
			AND media_type = 'text/html' 
			AND status_code >= 200 
			AND status_code < 300 
			AND (canonical = '' OR canonical = url) 
			AND crawled = 1
			AND title != ''
			GROUP BY title
			HAVING cnt > 1
		) as duplicates
	`

	row = ds.DB.QueryRow(duplicateQuery, cid)
	err = row.Scan(&duplicateTitle)
	if err != nil {
		log.Printf("GetTitleStats duplicate count: %v\n", err)
	} else {
		stats.DuplicateTitle = duplicateTitle
	}

	return stats
}

// GetDescriptionStats returns description statistics for a crawl
func (ds *DashboardRepository) GetDescriptionStats(cid int64) *models.DescriptionStats {
	query := `
		SELECT
			COUNT(*) as total_pages,
			SUM(CASE WHEN description = '' OR description IS NULL THEN 1 ELSE 0 END) as empty_description,
			SUM(CASE WHEN LENGTH(description) > 0 AND LENGTH(description) < 80 THEN 1 ELSE 0 END) as short_description,
			SUM(CASE WHEN LENGTH(description) > 160 THEN 1 ELSE 0 END) as long_description,
			SUM(CASE WHEN description_count > 1 THEN 1 ELSE 0 END) as multiple_descriptions
		FROM (
			SELECT 
				pr.*,
				(SELECT COUNT(*) FROM pagereports pr2 
				 WHERE pr2.crawl_id = pr.crawl_id 
				 AND pr2.description = pr.description 
				 AND pr2.media_type = 'text/html' 
				 AND pr2.status_code >= 200 
				 AND pr2.status_code < 300 
				 AND (pr2.canonical = '' OR pr2.canonical = pr2.url) 
				 AND pr2.crawled = 1) as description_count
			FROM pagereports pr
			WHERE pr.crawl_id = ? 
			AND pr.media_type = 'text/html' 
			AND pr.status_code >= 200 
			AND pr.status_code < 300 
			AND (pr.canonical = '' OR pr.canonical = pr.url) 
			AND pr.crawled = 1
		) as pr_with_counts
	`

	stats := &models.DescriptionStats{}
	row := ds.DB.QueryRow(query, cid)

	var duplicateDescription int
	err := row.Scan(&stats.TotalPages, &stats.EmptyDescription, &stats.ShortDescription, &stats.LongDescription, &stats.MultipleDescriptions)
	if err != nil {
		log.Printf("GetDescriptionStats: %v\n", err)
		return stats
	}

	// Get duplicate description count separately
	duplicateQuery := `
		SELECT COUNT(*) as duplicate_count
		FROM (
			SELECT description, COUNT(*) as cnt
			FROM pagereports
			WHERE crawl_id = ? 
			AND media_type = 'text/html' 
			AND status_code >= 200 
			AND status_code < 300 
			AND (canonical = '' OR canonical = url) 
			AND crawled = 1
			AND description != ''
			GROUP BY description
			HAVING cnt > 1
		) as duplicates
	`

	row = ds.DB.QueryRow(duplicateQuery, cid)
	err = row.Scan(&duplicateDescription)
	if err != nil {
		log.Printf("GetDescriptionStats duplicate count: %v\n", err)
	} else {
		stats.DuplicateDescription = duplicateDescription
	}

	return stats
}

// GetInlinkStats returns statistics about pages with zero or low inlinks
func (ds *DashboardRepository) GetInlinkStats(cid int64) *models.InlinkStats {
	stats := &models.InlinkStats{}

	// First, get total pages that should be indexed (HTML pages with 200-299 status code)
	totalQuery := `
		SELECT COUNT(*)
		FROM pagereports
		WHERE crawl_id = ? 
		AND media_type = 'text/html' 
		AND status_code >= 200 
		AND status_code < 300 
		AND (canonical = '' OR canonical = url) 
		AND crawled = 1
	`

	row := ds.DB.QueryRow(totalQuery, cid)
	err := row.Scan(&stats.TotalPages)
	if err != nil {
		log.Printf("GetInlinkStats total pages: %v\n", err)
		return stats
	}

	if stats.TotalPages == 0 {
		return stats
	}

	// Count pages with zero inlinks (isolated pages)
	zeroInlinksQuery := `
		SELECT COUNT(*)
		FROM pagereports pr
		WHERE pr.crawl_id = ? 
		AND pr.media_type = 'text/html' 
		AND pr.status_code >= 200 
		AND pr.status_code < 300 
		AND (pr.canonical = '' OR pr.canonical = pr.url) 
		AND pr.crawled = 1
		AND NOT EXISTS (
			SELECT 1 
			FROM links l
			INNER JOIN pagereports pr2 ON pr2.id = l.pagereport_id
			WHERE l.url_hash = pr.url_hash 
			AND pr2.crawl_id = ? 
			AND pr2.crawled = 1
		)
	`

	row = ds.DB.QueryRow(zeroInlinksQuery, cid, cid)
	err = row.Scan(&stats.ZeroInlinks)
	if err != nil {
		log.Printf("GetInlinkStats zero inlinks: %v\n", err)
	}

	// Count pages with <= 1 inlinks (low value entry pages)
	lowValueInlinksQuery := `
		SELECT COUNT(*)
		FROM pagereports pr
		WHERE pr.crawl_id = ? 
		AND pr.media_type = 'text/html' 
		AND pr.status_code >= 200 
		AND pr.status_code < 300 
		AND (pr.canonical = '' OR pr.canonical = pr.url) 
		AND pr.crawled = 1
		AND (
			SELECT COUNT(*)
			FROM links l
			INNER JOIN pagereports pr2 ON pr2.id = l.pagereport_id
			WHERE l.url_hash = pr.url_hash 
			AND pr2.crawl_id = ? 
			AND pr2.crawled = 1
		) <= 1
	`

	row = ds.DB.QueryRow(lowValueInlinksQuery, cid, cid)
	err = row.Scan(&stats.LowValueInlinks)
	if err != nil {
		log.Printf("GetInlinkStats low value inlinks: %v\n", err)
	}

	return stats
}

// GetMediaTypeDetails returns all media types with their counts (not limited by chartLimit)
func (ds *DashboardRepository) GetMediaTypeDetails(cid int64) []models.MediaTypeDetail {
	query := `
		SELECT media_type, count(*)
		FROM pagereports
		WHERE crawl_id = ? AND crawled = 1
		GROUP BY media_type
		ORDER BY count(*) DESC
	`

	var details []models.MediaTypeDetail
	rows, err := ds.DB.Query(query, cid)
	if err != nil {
		log.Printf("GetMediaTypeDetails: %v\n", err)
		return details
	}
	defer rows.Close()

	for rows.Next() {
		var detail models.MediaTypeDetail
		err := rows.Scan(&detail.MediaType, &detail.Count)
		if err != nil {
			log.Printf("GetMediaTypeDetails scan: %v\n", err)
			continue
		}
		details = append(details, detail)
	}

	return details
}
