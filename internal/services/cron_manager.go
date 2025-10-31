package services

import (
	"github.com/robfig/cron/v3"
	"github.com/stjudewashere/seonaut/internal/models"
	"log"
)

// 管理所有定时任务
type CronManagerService struct {
	c              *cron.Cron
	projectService *ProjectService
	crawlerService *CrawlerService
	jobIDs         map[int64]cron.EntryID // projectID -> cron EntryID
}

func NewCronManagerService(projectService *ProjectService, crawlerService *CrawlerService) *CronManagerService {
	c := cron.New(cron.WithSeconds())

	s := &CronManagerService{
		c:              c,
		projectService: projectService,
		crawlerService: crawlerService,
		jobIDs:         make(map[int64]cron.EntryID),
	}

	// 启动时加载任务
	if err := s.LoadFromDB(); err != nil {
		log.Printf("[Cron] 加载任务失败: %v", err)
	} else {
		log.Printf("[Cron] 成功加载 %d 个任务", len(s.jobIDs))
	}

	// 启动
	c.Start()
	log.Println("[Cron] 定时任务已启动")
	return s
}

// 从数据库加载项目
func (s *CronManagerService) LoadFromDB() error {
	projects := s.projectService.GetAllProject()
	//projects := s.projectService.GetProjectWithCron()

	// 清空旧任务
	for _, id := range s.jobIDs {
		s.c.Remove(id)
	}
	s.jobIDs = make(map[int64]cron.EntryID)

	if projects == nil || len(projects) == 0 {
		return nil
	}
	for _, p := range projects {
		if p.CronExpr != "" {
			if err := s.AddJob(p); err != nil {
				log.Printf("[Cron] 添加任务失败 [id=%d, url=%s, cron_expr=%s]", p.Id, p.URL, p.CronExpr)
			}
		}
	}
	return nil
}

// 添加单个任务
func (s *CronManagerService) AddJob(p models.Project) error {
	if p.CronExpr == "" {
		return nil
	}

	entryID, err := s.c.AddFunc(p.CronExpr, func() {
		log.Printf("[Cron] 开始执行任务 [id=%d, url=%s, cron_expr=%s]", p.Id, p.URL, p.CronExpr)
		err := s.crawlerService.StartCrawler(p, models.BasicAuth{})
		if err != nil {
			log.Printf("[Cron] 任务执行失败 [id=%d, url=%s, cron_expr=%s]：%v", p.Id, p.URL, p.CronExpr, err)
			return
		}
	})
	if err != nil {
		return err
	}

	s.jobIDs[p.Id] = entryID
	log.Printf("[Cron] 任务添加成功 [id=%d, url=%s, cron_expr=%s]", p.Id, p.URL, p.CronExpr)

	return nil
}

// 更新任务
func (s *CronManagerService) UpdateProject(p models.Project) error {
	// 删除旧任务
	s.DeleteJob(p)
	// 添加新任务
	return s.AddJob(p)
}

// 删除任务
func (s *CronManagerService) DeleteJob(p models.Project) {
	if oldID, exists := s.jobIDs[p.Id]; exists {
		s.c.Remove(oldID)
		delete(s.jobIDs, p.Id)
		log.Printf("[Cron] 删除任务成功 [id=%d, url=%s, cron_expr=%s]", p.Id, p.URL, p.CronExpr)
	}
}

// 关闭
func (s *CronManagerService) Stop() {
	s.c.Stop()
	log.Println("[Cron] 定时任务已关闭")
}
