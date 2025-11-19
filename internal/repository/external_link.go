package repository

import (
	"database/sql"
	"log"
)

type ExternalLinkRepository struct {
	DB *sql.DB
}

func (ds *ExternalLinkRepository) GetExternalLinkURLsByNofollow(crawlId int64, nofollow int, deduplication bool) []string {
	query := `
		select url
		from external_links
		where crawl_id = ?
		  and nofollow = ?
	`

	if deduplication {
		query += " group by url"
	}

	var result []string

	rows, err := ds.DB.Query(query, crawlId, nofollow)
	if err != nil {
		log.Printf("GetExternalLinkURLsByNofollow: %v\n", err)
		return result
	}

	defer rows.Close()

	for rows.Next() {
		var item string
		err := rows.Scan(&item)
		if err != nil {
			log.Printf("GetExternalLinkURLsByNofollow scan: %v\n", err)
			continue
		}
		result = append(result, item)
	}

	return result
}
