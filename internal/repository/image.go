package repository

import (
	"database/sql"
	"log"
)

type ImageRepository struct {
	DB *sql.DB
}

func (ds *ImageRepository) GetWithoutAltImageURLs(crawlId int64) []string {
	query := `SELECT url FROM images WHERE crawl_id = ? AND alt = ""`

	var result []string

	rows, err := ds.DB.Query(query, crawlId)
	if err != nil {
		log.Printf("GetWithoutAltImageUrl: %v\n", err)
		return result
	}

	defer rows.Close()

	for rows.Next() {
		var item string
		err := rows.Scan(&item)
		if err != nil {
			log.Printf("GetWithoutAltImageUrl scan: %v\n", err)
			continue
		}
		result = append(result, item)
	}

	return result
}
