package services

type (
	ImageServiceRepository interface {
		GetWithoutAltImageURLs(crawlId int64) []string
	}

	ImageService struct {
		repository ImageServiceRepository
	}
)

func NewImageService(r ImageServiceRepository) *ImageService {
	return &ImageService{repository: r}
}

func (s *ImageService) GetWithoutAltImageURLs(crawlId int64) []string {
	return s.repository.GetWithoutAltImageURLs(crawlId)
}
