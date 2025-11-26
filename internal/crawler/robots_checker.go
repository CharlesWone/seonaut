package crawler

import (
	"errors"
	"github.com/temoto/robotstxt"
	"net/url"
	"sync"
	"unsafe"
)

type RobotsChecker struct {
	robotsMap map[string]*robotstxt.RobotsData
	rlock     *sync.RWMutex
	client    Client
}

type RobotsData struct {
	// private
	groups      map[string]*robotstxt.Group
	allowAll    bool
	disallowAll bool
	Host        string
	Sitemaps    []string
}

func NewRobotsChecker(client Client) *RobotsChecker {
	return &RobotsChecker{
		robotsMap: make(map[string]*robotstxt.RobotsData),
		rlock:     &sync.RWMutex{},
		client:    client,
	}
}

// Returns true if the URL is blocked by robots.txt
func (r *RobotsChecker) IsBlocked(u *url.URL) bool {
	robot, err := r.getRobotsMap(u)
	if err != nil || robot == nil {
		return false
	}

	path := u.EscapedPath()
	if u.RawQuery != "" {
		path += "?" + u.Query().Encode()
	}

	return !robot.TestAgent(path, r.client.GetUAName())
}

// Returns true if the robots.txt file exists and is valid
func (r *RobotsChecker) Exists(u *url.URL) bool {
	robot, err := r.getRobotsMap(u)
	if err != nil {
		return false
	}

	if robot == nil {
		return false
	}

	return true
}

// Returns a list of sitemaps found in the robots.txt file
func (r *RobotsChecker) GetSitemaps(u *url.URL) []string {
	robot, err := r.getRobotsMap(u)
	if err != nil || robot == nil {
		return []string{}
	}

	return robot.Sitemaps
}

// Returns a RobotsData checking if it has already been created and stored in the robotsMap
func (r *RobotsChecker) getRobotsMap(u *url.URL) (*robotstxt.RobotsData, error) {
	r.rlock.Lock()
	defer r.rlock.Unlock()

	robot, ok := r.robotsMap[u.Host]
	if ok {
		return robot, nil
	}

	robotsURL := u.Scheme + "://" + u.Host + "/robots.txt"

	// Follow redirects to get the final response
	_, resp, err := r.followRedirect(robotsURL, 0, make(map[string]bool))
	if err != nil {
		r.robotsMap[u.Host] = nil
		return nil, err
	}
	defer resp.Response.Body.Close()

	if resp.Response.StatusCode != 200 {
		r.robotsMap[u.Host] = nil
		return nil, errors.New("robots.txt file does not exist")
	}

	robot, err = robotstxt.FromResponse(resp.Response)
	if err != nil {
		r.robotsMap[u.Host] = nil
		return nil, err
	}

	// 用指针映射访问未公开字段
	shadow := (*RobotsData)(unsafe.Pointer(robot))
	if len(shadow.groups) <= 0 {
		return nil, errors.New("robots.txt file does not exist")
	}

	r.robotsMap[u.Host] = robot

	return robot, nil
}

// followRedirect follows redirects and returns the final URL and response
func (r *RobotsChecker) followRedirect(URL string, redirectCount int, visited map[string]bool) (string, *ClientResponse, error) {
	// Prevent infinite redirect loops
	if redirectCount >= 5 {
		return "", nil, errors.New("redirect limit exceeded")
	}

	// Check if we've already visited this URL (prevent redirect loops)
	if visited[URL] {
		return "", nil, errors.New("redirect loop detected")
	}
	visited[URL] = true

	resp, err := r.client.Get(URL)
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
		return r.followRedirect(redirectURL.String(), redirectCount+1, visited)
	}

	// Not a redirect, return the response
	return URL, resp, nil
}
