package services

type (
	LinkServiceRepository interface {
		GetLinkURLsByNofollow(crawlId int64, nofollow int) []string
	}

	LinkService struct {
		repository LinkServiceRepository
	}
)

func NewLinkService(r LinkServiceRepository) *LinkService {
	return &LinkService{repository: r}
}

func (s *LinkService) GetLinkURLsByNofollow(crawlId int64, nofollow int) []string {
	return s.repository.GetLinkURLsByNofollow(crawlId, nofollow)

}
