package crawler

import (
	"errors"
	"net/url"
	"sync"

	sitemap "github.com/oxffaa/gopher-parse-sitemap"
)

type SitemapChecker struct {
	limit  int
	client Client
}

func NewSitemapChecker(client Client, limit int) *SitemapChecker {
	return &SitemapChecker{
		limit:  limit,
		client: client,
	}
}

// Check if any of the sitemap URLs provided exist
func (sc *SitemapChecker) SitemapExists(URLs []string) bool {
	for _, s := range URLs {
		if sc.urlExists(s) {
			return true
		}
	}

	return false
}

// Check if a URL exists by checking its status code
func (sc *SitemapChecker) urlExists(URL string) bool {
	return sc.urlExistsWithRedirect(URL, 0, make(map[string]bool))
}

// urlExistsWithRedirect checks if a URL exists, following redirects up to maxRedirects times
func (sc *SitemapChecker) urlExistsWithRedirect(URL string, redirectCount int, visited map[string]bool) bool {
	// Prevent infinite redirect loops
	if redirectCount >= 10 {
		return false
	}

	// Check if we've already visited this URL (prevent redirect loops)
	if visited[URL] {
		return false
	}
	visited[URL] = true

	resp, err := sc.client.Head(URL)
	if err != nil {
		return false
	}

	statusCode := resp.Response.StatusCode

	// Check if it's a redirect status code (301, 302, 307, 308)
	if statusCode == 301 || statusCode == 302 || statusCode == 307 || statusCode == 308 {
		location := resp.Response.Header.Get("Location")
		if location == "" {
			return false
		}

		// Parse the redirect target URL
		redirectURL, err := url.Parse(location)
		if err != nil {
			return false
		}

		// Resolve relative URLs against the original request URL
		baseURL, err := url.Parse(URL)
		if err != nil {
			return false
		}

		// If the redirect URL is relative, resolve it against the base URL
		if !redirectURL.IsAbs() {
			redirectURL = baseURL.ResolveReference(redirectURL)
		}

		// Recursively follow the redirect
		return sc.urlExistsWithRedirect(redirectURL.String(), redirectCount+1, visited)
	}

	// Check if the final status code indicates success
	return statusCode >= 200 && statusCode < 300
}

// Parse the sitemaps using a callback function on each entry
// For each URL provided check if it's an index sitemap
func (sc *SitemapChecker) ParseSitemaps(URLs []string, callback func(u string)) {
	c := 0
	wg := new(sync.WaitGroup)
	lock := sync.RWMutex{}

	for _, l := range URLs {
		sitemaps := sc.checkIndex(l)
		for _, s := range sitemaps {
			wg.Add(1)

			// Each sitemap is parsed in its own Go routine
			// If the sitemap limit is hit the parser function returns an error to stop the process
			go func(s string) {
				// Follow redirects to get the final response
				_, resp, err := sc.followRedirect(s, 0, make(map[string]bool))
				if err != nil {
					wg.Done()
					return
				}
				defer resp.Response.Body.Close()

				// Use the final URL for parsing
				sitemap.Parse(resp.Response.Body, func(e sitemap.Entry) error {
					callback(e.GetLocation())

					lock.Lock()
					defer lock.Unlock()

					c++
					if c >= sc.limit {
						return errors.New("URL limit hit")
					}

					return nil
				})

				wg.Done()
			}(s)
		}
	}

	wg.Wait()
}

// followRedirect follows redirects and returns the final URL and response
func (sc *SitemapChecker) followRedirect(URL string, redirectCount int, visited map[string]bool) (string, *ClientResponse, error) {
	// Prevent infinite redirect loops
	if redirectCount >= 5 {
		return "", nil, errors.New("redirect limit exceeded")
	}

	// Check if we've already visited this URL (prevent redirect loops)
	if visited[URL] {
		return "", nil, errors.New("redirect loop detected")
	}
	visited[URL] = true

	resp, err := sc.client.Get(URL)
	if err != nil {
		return "", nil, err
	}

	statusCode := resp.Response.StatusCode

	// Check if it's a redirect status code (301, 302, 307, 308)
	if statusCode == 301 || statusCode == 302 || statusCode == 307 || statusCode == 308 {
		location := resp.Response.Header.Get("Location")
		if location == "" {
			// No Location header, return current response
			return URL, resp, nil
		}

		// Close the redirect response body
		resp.Response.Body.Close()

		// Parse the redirect target URL
		redirectURL, err := url.Parse(location)
		if err != nil {
			return "", nil, err
		}

		// Resolve relative URLs against the original request URL
		baseURL, err := url.Parse(URL)
		if err != nil {
			return "", nil, err
		}

		// If the redirect URL is relative, resolve it against the base URL
		if !redirectURL.IsAbs() {
			redirectURL = baseURL.ResolveReference(redirectURL)
		}

		// Recursively follow the redirect
		return sc.followRedirect(redirectURL.String(), redirectCount+1, visited)
	}

	// Not a redirect, return the response
	return URL, resp, nil
}

// Returns a slice of strings with sitemap URLs
// If URL is a sitemap index the slice will contain all the sitemaps found
// Otherwise it will return an slice containing only the original URL
func (sc *SitemapChecker) checkIndex(URL string) []string {
	sitemaps := []string{}

	sitemap.ParseIndexFromSite(URL, func(e sitemap.IndexEntry) error {
		l := e.GetLocation()
		sitemaps = append(sitemaps, l)
		return nil
	})

	if len(sitemaps) == 0 {
		sitemaps = append(sitemaps, URL)
	}

	return sitemaps
}
