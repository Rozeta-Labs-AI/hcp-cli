package hcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultUserAgent = "hcp-cli/dev"

type Client struct {
	baseURL    *url.URL
	apiKey     string
	authMode   string
	companyID  string
	httpClient *http.Client
	userAgent  string
}

type Options struct {
	BaseURL    string
	APIKey     string
	AuthMode   string
	CompanyID  string
	HTTPClient *http.Client
	UserAgent  string
}

type StatusError struct {
	Method string
	Path   string
	Status string
	Code   int
	Body   string
}

type PageRequest struct {
	Page        int
	PageSize    int
	LocationIDs []string
	Expand      []string
	Since       string
	Extra       url.Values
}

type CollectionPage struct {
	Resource   string
	Items      []map[string]any
	Page       int
	PageSize   int
	TotalPages int
	Raw        json.RawMessage
}

type FilePart struct {
	FieldName   string
	Path        string
	ContentType string
}

func (e *StatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("housecall pro api %s %s failed: %s", e.Method, e.Path, e.Status)
	}
	return fmt.Sprintf("housecall pro api %s %s failed: %s: %s", e.Method, e.Path, e.Status, e.Body)
}

func New(opts Options) (*Client, error) {
	base := strings.TrimSpace(opts.BaseURL)
	if base == "" {
		base = "https://api.housecallpro.com"
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("base url must include scheme and host")
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	userAgent := strings.TrimSpace(opts.UserAgent)
	if userAgent == "" {
		userAgent = defaultUserAgent
	}

	return &Client{
		baseURL:    parsed,
		apiKey:     strings.TrimSpace(opts.APIKey),
		authMode:   strings.ToLower(strings.TrimSpace(opts.AuthMode)),
		companyID:  strings.TrimSpace(opts.CompanyID),
		httpClient: httpClient,
		userAgent:  userAgent,
	}, nil
}

func (c *Client) GetRaw(ctx context.Context, path string, query url.Values) (json.RawMessage, error) {
	return c.DoRaw(ctx, http.MethodGet, path, query, nil)
}

func (c *Client) ListCustomers(ctx context.Context, req PageRequest) (CollectionPage, error) {
	return c.ListResource(ctx, "customers", "customer", "/customers", req)
}

func (c *Client) ListJobs(ctx context.Context, req PageRequest) (CollectionPage, error) {
	return c.ListResource(ctx, "jobs", "job", "/jobs", req)
}

func (c *Client) ListEstimates(ctx context.Context, req PageRequest) (CollectionPage, error) {
	return c.ListResource(ctx, "estimates", "estimate", "/estimates", req)
}

func (c *Client) ListLeads(ctx context.Context, req PageRequest) (CollectionPage, error) {
	return c.ListResource(ctx, "leads", "lead", "/leads", req)
}

func (c *Client) ListEmployees(ctx context.Context, req PageRequest) (CollectionPage, error) {
	return c.ListResource(ctx, "employees", "employee", "/employees", req)
}

func (c *Client) ListInvoices(ctx context.Context, req PageRequest) (CollectionPage, error) {
	return c.ListResource(ctx, "invoices", "invoice", "/invoices", req)
}

func (c *Client) ListJobAppointments(ctx context.Context, jobID string, req PageRequest) (CollectionPage, error) {
	return c.ListResource(ctx, "appointments", "appointment", "/jobs/"+url.PathEscape(jobID)+"/appointments", req)
}

func (c *Client) ListResource(ctx context.Context, resource string, singular string, path string, req PageRequest) (CollectionPage, error) {
	query := req.values()
	raw, err := c.GetRaw(ctx, path, query)
	if err != nil {
		return CollectionPage{}, err
	}
	items, totalPages, err := collection(resource, singular, raw)
	if err != nil {
		return CollectionPage{}, err
	}
	return CollectionPage{
		Resource:   resource,
		Items:      items,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
		Raw:        raw,
	}, nil
}

func (c *Client) DoRaw(ctx context.Context, method string, path string, query url.Values, body any) (json.RawMessage, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	reqURL := c.resolve(path, query)
	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.companyID != "" {
		req.Header.Set("X-Company-Id", c.companyID)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", c.authHeader())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &StatusError{
			Method: method,
			Path:   path,
			Status: resp.Status,
			Code:   resp.StatusCode,
			Body:   truncate(strings.TrimSpace(string(data)), 700),
		}
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return json.RawMessage("null"), nil
	}

	return json.RawMessage(data), nil
}

func (c *Client) DoMultipart(ctx context.Context, method string, path string, query url.Values, fields map[string]string, files []FilePart) (json.RawMessage, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, fmt.Errorf("write multipart field: %w", err)
		}
	}
	for _, filePart := range files {
		if err := addFilePart(writer, filePart); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart body: %w", err)
	}

	reqURL := c.resolve(path, query)
	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), &buffer)
	if err != nil {
		return nil, fmt.Errorf("build multipart request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if c.companyID != "" {
		req.Header.Set("X-Company-Id", c.companyID)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", c.authHeader())
	}
	return c.do(req, method, path)
}

func addFilePart(writer *multipart.Writer, filePart FilePart) error {
	field := strings.TrimSpace(filePart.FieldName)
	if field == "" {
		field = "file"
	}
	file, err := os.Open(filePart.Path)
	if err != nil {
		return fmt.Errorf("open file %s: %w", filePart.Path, err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile(field, filepath.Base(filePart.Path))
	if err != nil {
		return fmt.Errorf("create multipart file part: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("write multipart file part: %w", err)
	}
	return nil
}

func (c *Client) do(req *http.Request, method string, path string) (json.RawMessage, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &StatusError{
			Method: method,
			Path:   path,
			Status: resp.Status,
			Code:   resp.StatusCode,
			Body:   truncate(strings.TrimSpace(string(data)), 700),
		}
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return json.RawMessage("null"), nil
	}

	return json.RawMessage(data), nil
}

func (r PageRequest) values() url.Values {
	query := url.Values{}
	if r.Extra != nil {
		for key, values := range r.Extra {
			for _, value := range values {
				query.Add(key, value)
			}
		}
	}
	if r.Page > 0 {
		query.Set("page", fmt.Sprintf("%d", r.Page))
	}
	if r.PageSize > 0 {
		query.Set("page_size", fmt.Sprintf("%d", r.PageSize))
	}
	if r.Since != "" {
		query.Set("updated_at_min", r.Since)
	}
	for _, id := range r.LocationIDs {
		if strings.TrimSpace(id) != "" {
			query.Add("location_ids[]", strings.TrimSpace(id))
		}
	}
	for _, expand := range r.Expand {
		if strings.TrimSpace(expand) != "" {
			query.Add("expand[]", strings.TrimSpace(expand))
		}
	}
	return query
}

func collection(resource string, singular string, raw json.RawMessage) ([]map[string]any, int, error) {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, 0, fmt.Errorf("decode response json: %w", err)
	}
	totalPages := 0
	if object, ok := payload.(map[string]any); ok {
		totalPages = intValue(object["total_pages"])
		for _, key := range []string{resource, singular, "data", "items", "results"} {
			if arr, ok := object[key].([]any); ok {
				return mapItems(arr), totalPages, nil
			}
		}
		if _, hasPage := object["page"]; !hasPage {
			return []map[string]any{object}, totalPages, nil
		}
	}
	if arr, ok := payload.([]any); ok {
		return mapItems(arr), totalPages, nil
	}
	return nil, totalPages, nil
}

func mapItems(values []any) []map[string]any {
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]any); ok {
			out = append(out, item)
		}
	}
	return out
}

func intValue(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case json.Number:
		i, _ := typed.Int64()
		return int(i)
	default:
		return 0
	}
}

func (c *Client) resolve(path string, query url.Values) *url.URL {
	u := *c.baseURL
	basePath := strings.TrimRight(u.Path, "/")
	reqPath := "/" + strings.TrimLeft(path, "/")
	u.Path = basePath + reqPath
	u.RawQuery = query.Encode()
	return &u
}

func (c *Client) authHeader() string {
	if c.authMode == "oauth" {
		return "Bearer " + c.apiKey
	}
	return "Token " + c.apiKey
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}
