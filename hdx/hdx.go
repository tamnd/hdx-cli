// Package hdx is the library behind the hdx command line:
// the HTTP client, request shaping, and the typed data models for the
// Humanitarian Data Exchange (HDX) API.
//
// HDX (data.humdata.org) is a CKAN-based open humanitarian data platform run
// by OCHA. Every action endpoint lives under /api/action/<name> and returns
// {"success": true, "result": <payload>}. No API key or cookie required.
//
// The Client sets a real User-Agent, paces requests so a busy session stays
// polite, and retries transient failures (429 and 5xx).
package hdx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to HDX.
const DefaultUserAgent = "hdx-cli/0.1 (tamnd87@gmail.com)"

// Host is the HDX site hostname.
const Host = "data.humdata.org"

// BaseURL is the CKAN API root every request is built from.
const BaseURL = "https://data.humdata.org/api/action"

// --- wire types ---

type wireCKANResult[T any] struct {
	Success bool `json:"success"`
	Result  T    `json:"result"`
}

type wireSearchResult[T any] struct {
	Count   int `json:"count"`
	Results []T `json:"results"`
}

type wirePackage struct {
	Name             string         `json:"name"`
	Title            string         `json:"title"`
	Notes            string         `json:"notes"`
	Organization     wireOrg        `json:"organization"`
	Resources        []wireResource `json:"resources"`
	Tags             []wireTag      `json:"tags"`
	MetadataModified string         `json:"metadata_modified"`
	NumResources     int            `json:"num_resources"`
}

type wireOrg struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

type wireResource struct {
	Name        string          `json:"name"`
	Format      string          `json:"format"`
	URL         string          `json:"url"`
	Description string          `json:"description"`
	Size        json.RawMessage `json:"size"` // number or string in the wild
}

type wireTag struct {
	Name string `json:"name"`
}

type wireOrgItem struct {
	Name         string `json:"name"`
	Title        string `json:"title"`
	PackageCount int    `json:"package_count"`
}

// --- output record types ---

// Dataset is one search result record.
type Dataset struct {
	Name         string `kit:"id" json:"name"`
	Title        string `json:"title"`
	Organization string `json:"organization"`
	Resources    int    `json:"resources"`
	Modified     string `json:"modified"`
	Tags         string `json:"tags"` // comma-joined first 3 tags
}

// Package is the summary record for a dataset.
type Package struct {
	Name         string `kit:"id" json:"name"`
	Title        string `json:"title"`
	Organization string `json:"organization"`
	Description  string `json:"description"` // truncated notes
	Modified     string `json:"modified"`
	Resources    int    `json:"resources"`
}

// Resource is one file attached to a dataset.
type Resource struct {
	Name        string `kit:"id" json:"name"`
	Format      string `json:"format"`
	URL         string `json:"url" table:"url,url"`
	Description string `json:"description"`
}

// Organization is one HDX contributing organization.
type Organization struct {
	Name         string `kit:"id" json:"name"`
	Title        string `json:"title"`
	PackageCount int    `json:"package_count"`
}

// --- client ---

// Client talks to the HDX CKAN API over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults: 30s timeout, 300ms
// minimum gap between requests, and 3 retries on transient errors.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: DefaultUserAgent,
		Rate:      300 * time.Millisecond,
		Retries:   3,
	}
}

// SearchDatasets searches HDX datasets by keyword. org filters by organization
// name slug if non-empty.
func (c *Client) SearchDatasets(ctx context.Context, query string, limit int, org string) ([]*Dataset, error) {
	if limit <= 0 {
		limit = 10
	}
	u := BaseURL + "/package_search?q=" + url.QueryEscape(query) +
		fmt.Sprintf("&rows=%d", limit) +
		"&sort=score+desc"
	if org != "" {
		u += "&fq=organization:" + url.QueryEscape(org)
	}

	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}

	var env wireCKANResult[wireSearchResult[wirePackage]]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode package_search: %w", err)
	}
	if !env.Success {
		return nil, fmt.Errorf("package_search returned success=false")
	}

	var out []*Dataset
	for _, wp := range env.Result.Results {
		out = append(out, &Dataset{
			Name:         wp.Name,
			Title:        wp.Title,
			Organization: wp.Organization.Title,
			Resources:    wp.NumResources,
			Modified:     shortDate(wp.MetadataModified),
			Tags:         joinTags(wp.Tags, 3),
		})
	}
	return out, nil
}

// GetPackage fetches a single dataset by name/slug and returns the summary
// record plus all resource records.
func (c *Client) GetPackage(ctx context.Context, name string) (*Package, []*Resource, error) {
	u := BaseURL + "/package_show?id=" + url.QueryEscape(name)
	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, nil, err
	}

	var env wireCKANResult[wirePackage]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, nil, fmt.Errorf("decode package_show: %w", err)
	}
	if !env.Success {
		return nil, nil, fmt.Errorf("package_show returned success=false")
	}

	wp := env.Result
	pkg := &Package{
		Name:         wp.Name,
		Title:        wp.Title,
		Organization: wp.Organization.Title,
		Description:  truncate(wp.Notes, 200),
		Modified:     shortDate(wp.MetadataModified),
		Resources:    wp.NumResources,
	}

	var resources []*Resource
	for _, wr := range wp.Resources {
		resources = append(resources, &Resource{
			Name:        wr.Name,
			Format:      wr.Format,
			URL:         wr.URL,
			Description: wr.Description,
		})
	}
	return pkg, resources, nil
}

// ListOrganizations returns HDX organizations.
func (c *Client) ListOrganizations(ctx context.Context, limit int) ([]*Organization, error) {
	if limit <= 0 {
		limit = 20
	}
	u := fmt.Sprintf("%s/organization_list?all_fields=true&limit=%d", BaseURL, limit)
	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}

	var env wireCKANResult[[]wireOrgItem]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode organization_list: %w", err)
	}
	if !env.Success {
		return nil, fmt.Errorf("organization_list returned success=false")
	}

	var out []*Organization
	for _, wo := range env.Result {
		out = append(out, &Organization{
			Name:         wo.Name,
			Title:        wo.Title,
			PackageCount: wo.PackageCount,
		})
	}
	return out, nil
}

// Get fetches rawURL and returns the response body. It paces and retries
// according to the client's settings.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- helpers ---

// shortDate trims a datetime string like "2024-01-15T10:00:00.000000" to
// just the date "2024-01-15".
func shortDate(s string) string {
	if i := strings.Index(s, "T"); i > 0 {
		return s[:i]
	}
	return s
}

// truncate shortens s to at most n bytes, appending "..." if it was cut.
func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// joinTags returns up to max tags comma-joined.
func joinTags(tags []wireTag, max int) string {
	var names []string
	for i, t := range tags {
		if i >= max {
			break
		}
		names = append(names, t.Name)
	}
	return strings.Join(names, ", ")
}
