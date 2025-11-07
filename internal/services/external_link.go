package services

type (
	ExternalLinkServiceRepository interface {
		GetExternalLinkURLsByNofollow(crawlId int64, nofollow int) []string
	}

	ExternalLinkService struct {
		repository ExternalLinkServiceRepository
	}
)

func NewExternalLinkService(r ExternalLinkServiceRepository) *ExternalLinkService {
	return &ExternalLinkService{repository: r}
}

func (s *ExternalLinkService) GetExternalLinkURLsByNofollow(crawlId int64, nofollow int) []string {
	return s.repository.GetExternalLinkURLsByNofollow(crawlId, nofollow)
}
