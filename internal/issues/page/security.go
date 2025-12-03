package page

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/stjudewashere/seonaut/internal/issues/errors"
	"github.com/stjudewashere/seonaut/internal/models"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// Returns a report_manager.PageIssueReporter with a callback function that
// reports if the page's HSTS header is missing. The callback returns true if the Strict-Transport-Security,
// header does not exist or is not valid.
func NewMissingHSTSHeaderReporter() *models.PageIssueReporter {
	c := func(pageReport *models.PageReport, htmlNode *html.Node, header *http.Header) bool {
		hstsHeader := header.Get("Strict-Transport-Security")
		if hstsHeader == "" {
			return true
		}

		directives := strings.Split(hstsHeader, ";")
		for _, directive := range directives {
			if strings.HasPrefix(directive, "max-age=") {
				maxAge := strings.TrimPrefix(directive, "max-age=")
				_, err := strconv.Atoi(maxAge)
				if err != nil {
					return true
				}
			}
		}

		return false
	}

	return &models.PageIssueReporter{
		ErrorType: errors.ErrorMissingHSTSHeader,
		Callback:  c,
	}
}

// Returns a report_manager.PageIssueReporter with a callback function that
// reports if the page's CSP (Content Security Policy) is missing by looking both in the Headers and meta tags.
// The callback returns true if the CSP does not exist.
func NewMissingCSPReporter() *models.PageIssueReporter {
	c := func(pageReport *models.PageReport, htmlNode *html.Node, header *http.Header) bool {
		if pageReport.MediaType != "text/html" {
			return false
		}

		cspTag, err := htmlquery.QueryAll(htmlNode, "//head/meta[@http-equiv=\"Content-Security-Policy\"]")
		if err != nil {
			return false
		}

		CSPHeader := header.Get("Content-Security-Policy")

		return cspTag == nil && CSPHeader == ""
	}

	return &models.PageIssueReporter{
		ErrorType: errors.ErrorMissingCSP,
		Callback:  c,
	}
}

// Returns a report_manager.PageIssueReporter with a callback function that
// reports if the page's X-Content-Type-Options header is missing.
// The callback returns true if the header does not exist.
func NewMissingContentTypeOptionsReporter() *models.PageIssueReporter {
	c := func(pageReport *models.PageReport, htmlNode *html.Node, header *http.Header) bool {
		if pageReport.MediaType != "text/html" {
			return false
		}

		contentTypeOptions := header.Get("X-Content-Type-Options")

		return contentTypeOptions != "nosniff"
	}

	return &models.PageIssueReporter{
		ErrorType: errors.ErrorContentTypeOptions,
		Callback:  c,
	}
}

// NewMissingXFrameOptionsReporter
// X-Frame-Options HTTP 响应头是用来给浏览器指示允许一个页面可否在 <frame>, </iframe> 或者 <object> 中展现的标记。
// 网站可以使用此功能，来确保自己网站的内容没有被嵌套到别人的网站中去，也从而避免了点击劫持 (clickjacking) 的攻击。
// DENY 表示该页面不允许在frame中展示，即便是在相同域名的页面中嵌套也不允许。
// SAMEORIGIN 表示该页面可以在相同域名页面的frame中展示。通常使用此项。
// ALLOW-FROM uri 表示该页面可以在指定来源的frame中展示。
func NewMissingXFrameOptionsReporter() *models.PageIssueReporter {
	return &models.PageIssueReporter{
		ErrorType: errors.ErrorMissingXFrameOptions,
		Callback: func(p *models.PageReport, node *html.Node, header *http.Header) bool {
			if p.MediaType != "text/html" {
				return false // 没问题
			}
			xfo := header.Get("X-Frame-Options")
			// 设置了 DENY 或 SAMEORIGIN 就算合规（不区分大小写）
			if xfo != "" && (strings.EqualFold(xfo, "DENY") || strings.EqualFold(xfo, "SAMEORIGIN")) {
				return false // 没问题
			}
			return true // 有问题
		},
	}
}

// NewMissingReferrerPolicyReporter
// 检查Referrer-Policy头需要存在
func NewMissingReferrerPolicyReporter() *models.PageIssueReporter {
	return &models.PageIssueReporter{
		ErrorType: errors.ErrorMissingReferrerPolicy,
		Callback: func(p *models.PageReport, node *html.Node, header *http.Header) bool {
			// 检查 Referrer-Policy 这个头是否存在且不为空
			referrerPolicy := header.Get("Referrer-Policy")
			if referrerPolicy != "" {
				return false // 没问题
			}
			return true // 有问题
		},
	}
}

// NewServerVersionLeakReporter 检查 Server 头是否泄露版本（Cloudflare 除外）
func NewServerVersionLeakReporter() *models.PageIssueReporter {
	return &models.PageIssueReporter{
		ErrorType: errors.ErrorServerVersionLeak,
		Callback: func(p *models.PageReport, node *html.Node, header *http.Header) bool {
			if header == nil {
				return false
			}
			server := header.Get("Server")
			if server == "" {
				return false // 没问题
			}
			// Cloudflare 返回的通常是 "cloudflare" 或泛化值，允许
			if strings.Contains(strings.ToLower(server), "cloudflare") {
				return false // 没问题
			}
			// 包含类似 1.18.0、2.4.41、10.0 这种版本号的都算泄露
			return regexp.MustCompile(`[0-9]+\.[0-9]+`).MatchString(server)
		},
	}
}

// NewXPoweredByLeakReporter 检查 X-Powered-By 是否存在
func NewXPoweredByLeakReporter() *models.PageIssueReporter {
	return &models.PageIssueReporter{
		ErrorType: errors.ErrorXPoweredByLeak,
		Callback: func(p *models.PageReport, node *html.Node, header *http.Header) bool {
			return header.Get("X-Powered-By") != ""
		},
	}
}

// NewAspNetVersionLeakReporter 检查 ASP.NET 版本泄露（X-AspNet-Version 和 X-AspNetMvc-Version）
func NewAspNetVersionLeakReporter() *models.PageIssueReporter {
	return &models.PageIssueReporter{
		ErrorType: errors.ErrorAspNetVersionLeak,
		Callback: func(p *models.PageReport, node *html.Node, header *http.Header) bool {
			asp1 := header.Get("X-AspNet-Version")
			asp2 := header.Get("X-AspNetMvc-Version")
			return asp1 != "" || asp2 != ""
		},
	}
}
