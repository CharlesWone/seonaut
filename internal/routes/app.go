package routes

import (
	"fmt"
	"github.com/stjudewashere/seonaut/internal/models"
	"github.com/stjudewashere/seonaut/internal/services"
	"log"
	"net/http"
)

// PageView is the data structure used to render the html templates.
type PageView struct {
	Lang      string
	Theme     string
	PageTitle string
	User      models.User
	Data      interface{}
	Refresh   bool
}

// 重定向函数
func redirect(w http.ResponseWriter, r *http.Request, code int, container *services.Container, path string) {
	contextPath := container.Config.HTTPServer.ContextPath
	http.Redirect(w, r, contextPath+path, code)
}

// NewServer sets up the HTTP server routes and starts the HTTP server.
func NewServer(container *services.Container) {
	contextPath := container.Config.HTTPServer.ContextPath

	// Handle static files
	fileServer := http.FileServer(http.Dir("./web/static"))
	cssFileServer := http.FileServer(http.Dir("./web/css"))
	fontsFileServer := http.FileServer(http.Dir("./web/fonts"))
	//http.Handle("GET /resources/", http.StripPrefix("/resources", fileServer))
	http.Handle("GET /resources/", http.StripPrefix("/resources", cssFileServer))
	http.Handle("GET /fonts/", http.StripPrefix("/fonts", fontsFileServer))
	http.Handle("GET /resources/echarts.min.js", http.StripPrefix("/resources", fileServer))
	http.Handle("GET /robots.txt", fileServer)
	http.Handle("GET /favicon.ico", fileServer)

	// Crawler routes
	crawlHandler := crawlHandler{container}
	http.HandleFunc("GET "+contextPath+"/crawl/start", container.CookieSession.Auth(crawlHandler.startHandler))
	http.HandleFunc("GET "+contextPath+"/crawl/stop", container.CookieSession.Auth(crawlHandler.stopHandler))
	http.HandleFunc("GET "+contextPath+"/crawl/live", container.CookieSession.Auth(crawlHandler.liveCrawlHandler))
	http.HandleFunc("GET "+contextPath+"/crawl/auth", container.CookieSession.Auth(crawlHandler.authGetHandler))
	http.HandleFunc("POST "+contextPath+"/crawl/auth", container.CookieSession.Auth(crawlHandler.authPostHandler))
	http.HandleFunc("GET "+contextPath+"/crawl/ws", container.CookieSession.Auth(crawlHandler.wsHandler))

	// Dashboard route
	dashboardHandler := dashboardHandler{container}
	http.HandleFunc("GET "+contextPath+"/dashboard", container.CookieSession.Auth(dashboardHandler.indexHandler))

	// URL explorer route
	explorerHandler := explorerHandler{container}
	http.HandleFunc("GET "+contextPath+"/explorer", container.CookieSession.Auth(explorerHandler.indexHandler))

	// Data export routes
	exportHandler := exportHandler{container}
	http.HandleFunc("GET "+contextPath+"/export", container.CookieSession.Auth(exportHandler.indexHandler))
	http.HandleFunc("GET "+contextPath+"/export/csv", container.CookieSession.Auth(exportHandler.csvHandler))
	http.HandleFunc("GET "+contextPath+"/export/sitemap", container.CookieSession.Auth(exportHandler.sitemapHandler))
	http.HandleFunc("GET "+contextPath+"/export/resources", container.CookieSession.Auth(exportHandler.resourcesHandler))
	http.HandleFunc("GET "+contextPath+"/export/wazc", container.CookieSession.Auth(exportHandler.waczHandler))

	// Crawl Report routes
	crawlReportHandler := crawlReportHandler{container}
	// Crawl Report route 路劲参数传参
	http.HandleFunc("GET "+contextPath+"/crawlReport/", crawlReportHandler.indexHandler)

	// Issues routes
	issueHandler := issueHandler{container}
	http.HandleFunc("GET "+contextPath+"/issues", container.CookieSession.Auth(issueHandler.indexHandler))
	http.HandleFunc("GET "+contextPath+"/issues/view", container.CookieSession.Auth(issueHandler.viewHandler))

	// Issues Report routes
	issuesReportHandler := issuesReportHandler{container}
	http.HandleFunc("GET "+contextPath+"/issuesReport/", issuesReportHandler.indexHandler)
	http.HandleFunc("GET "+contextPath+"/issuesReport/view/", issuesReportHandler.viewHandler)

	// Project routes
	projectHandler := projectHandler{container}
	http.HandleFunc("GET "+contextPath+"/", container.CookieSession.Auth(projectHandler.indexHandler))
	http.HandleFunc("GET "+contextPath+"/project/add", container.CookieSession.Auth(projectHandler.addGetHandler))
	http.HandleFunc("POST "+contextPath+"/project/add", container.CookieSession.Auth(projectHandler.addPostHandler))
	http.HandleFunc("GET "+contextPath+"/project/edit", container.CookieSession.Auth(projectHandler.editGetHandler))
	http.HandleFunc("POST "+contextPath+"/project/edit", container.CookieSession.Auth(projectHandler.editPostHandler))
	http.HandleFunc("GET "+contextPath+"/project/delete", container.CookieSession.Auth(projectHandler.deleteHandler))

	// Resource route
	resourceHandler := resourceHandler{container}
	http.HandleFunc("GET "+contextPath+"/resources", container.CookieSession.Auth(resourceHandler.indexHandler))

	// Archive Handler
	archiveHandler := archiveHandler{container}
	http.HandleFunc("GET "+contextPath+"/archive", container.CookieSession.Auth(archiveHandler.archiveHandler))
	http.HandleFunc("GET "+contextPath+"/archive/download", container.CookieSession.Auth(archiveHandler.downloadHandler))

	// User routes
	userHandler := userHandler{container}
	http.HandleFunc("GET "+contextPath+"/signup", userHandler.signupGetHandler)
	http.HandleFunc("POST "+contextPath+"/signup", userHandler.signupPostHandler)
	http.HandleFunc("GET "+contextPath+"/signin", userHandler.signinGetHandler)
	http.HandleFunc("POST "+contextPath+"/signin", userHandler.signinPostHandler)
	http.HandleFunc("GET "+contextPath+"/account", container.CookieSession.Auth(userHandler.editGetHandler))
	http.HandleFunc("POST "+contextPath+"/account", container.CookieSession.Auth(userHandler.editPostHandler))
	http.HandleFunc("GET "+contextPath+"/account/password", container.CookieSession.Auth(userHandler.changePasswordGetHandler))
	http.HandleFunc("POST "+contextPath+"/account/password", container.CookieSession.Auth(userHandler.changePasswordPostHandler))
	http.HandleFunc("GET "+contextPath+"/account/delete", container.CookieSession.Auth((userHandler.deleteGetHandler)))
	http.HandleFunc("POST "+contextPath+"/account/delete", container.CookieSession.Auth((userHandler.deletePostHandler)))
	http.HandleFunc("GET "+contextPath+"/signout", container.CookieSession.Auth(userHandler.signoutHandler))

	// Replay routes
	replayHandler := replayHandler{container}
	http.HandleFunc("GET "+contextPath+"/replay", container.CookieSession.Auth(replayHandler.proxyHandler))

	fmt.Printf("Starting server at %s on port %d...\n", container.Config.HTTPServer.Server, container.Config.HTTPServer.Port)
	fmt.Printf("Access url: %s\n", container.Config.HTTPServer.URL)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", container.Config.HTTPServer.Server, container.Config.HTTPServer.Port), nil)
	if err != nil {
		log.Fatalf("error starting server: %v", err)
	}
}
