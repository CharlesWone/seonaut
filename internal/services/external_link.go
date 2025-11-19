package services

type (
	ExternalLinkServiceRepository interface {
		GetExternalLinkURLsByNofollow(crawlId int64, nofollow int, deduplication bool) []string
	}

	ExternalLinkService struct {
		repository ExternalLinkServiceRepository
	}
)

func NewExternalLinkService(r ExternalLinkServiceRepository) *ExternalLinkService {
	return &ExternalLinkService{repository: r}
}

func (s *ExternalLinkService) GetExternalLinkURLsByNofollow(crawlId int64, nofollow int, deduplication bool) []string {
	return s.repository.GetExternalLinkURLsByNofollow(crawlId, nofollow, deduplication)
}
