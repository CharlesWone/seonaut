package repository

import (
	"database/sql"
	"log"
)

type LinkRepository struct {
	DB *sql.DB
}

func (ds *LinkRepository) GetLinkURLsByNofollow(crawlId int64, nofollow int) []string {
	query := `
		select url
		from links
		where crawl_id = ?
		  and nofollow = ?
	`
	var result []string

	rows, err := ds.DB.Query(query, crawlId, nofollow)
	if err != nil {
		log.Printf("GetLinkURLsByNofollow: %v\n", err)
		return result
	}

	defer rows.Close()

	for rows.Next() {
		var item string
		err := rows.Scan(&item)
		if err != nil {
			log.Printf("GetLinkURLsByNofollow scan: %v\n", err)
			continue
		}
		result = append(result, item)
	}

	return result
}
