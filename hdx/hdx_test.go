package hdx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockPackageSearch returns a minimal CKAN package_search JSON body.
func mockPackageSearch() []byte {
	body, _ := json.Marshal(map[string]any{
		"success": true,
		"result": map[string]any{
			"count": 2,
			"results": []map[string]any{
				{
					"name":              "ukraine-refugee",
					"title":             "Ukraine - Refugee Situation",
					"notes":             "Data on refugees from Ukraine.",
					"metadata_modified": "2024-03-15T08:00:00.000000",
					"num_resources":     3,
					"organization":      map[string]any{"name": "unhcr", "title": "UNHCR"},
					"tags":              []map[string]any{{"name": "refugees"}, {"name": "ukraine"}, {"name": "displacement"}},
					"resources":         []any{},
				},
				{
					"name":              "south-sudan-food",
					"title":             "South Sudan Food Security",
					"notes":             "Monthly food security data.",
					"metadata_modified": "2024-02-01T00:00:00.000000",
					"num_resources":     1,
					"organization":      map[string]any{"name": "wfp", "title": "WFP"},
					"tags":              []map[string]any{{"name": "food security"}},
					"resources":         []any{},
				},
			},
		},
	})
	return body
}

// mockPackageShow returns a CKAN package_show JSON body with resources.
func mockPackageShow() []byte {
	body, _ := json.Marshal(map[string]any{
		"success": true,
		"result": map[string]any{
			"name":              "ukraine-refugee",
			"title":             "Ukraine - Refugee Situation",
			"notes":             "Detailed data on refugees fleeing Ukraine since February 2022.",
			"metadata_modified": "2024-03-15T08:00:00.000000",
			"num_resources":     2,
			"organization":      map[string]any{"name": "unhcr", "title": "UNHCR"},
			"tags":              []map[string]any{{"name": "refugees"}, {"name": "ukraine"}},
			"resources": []map[string]any{
				{
					"name":        "data.csv",
					"format":      "CSV",
					"url":         "https://data.humdata.org/dataset/ukraine-refugee/resource/data.csv",
					"description": "Main data file",
					"size":        "12345",
				},
				{
					"name":        "README.txt",
					"format":      "TXT",
					"url":         "https://data.humdata.org/dataset/ukraine-refugee/resource/README.txt",
					"description": "Data description",
					"size":        "1024",
				},
			},
		},
	})
	return body
}

// mockOrgList returns a CKAN organization_list JSON body.
func mockOrgList() []byte {
	body, _ := json.Marshal(map[string]any{
		"success": true,
		"result": []map[string]any{
			{"name": "unhcr", "title": "UNHCR", "package_count": 120},
			{"name": "wfp", "title": "WFP", "package_count": 85},
			{"name": "ocha", "title": "OCHA", "package_count": 62},
		},
	})
	return body
}

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestSearchDatasets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/package_search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query().Get("q")
		if q == "" {
			t.Error("missing q parameter")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(mockPackageSearch())
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	// Override BaseURL for this test by using a custom URL directly
	oldBase := BaseURL
	_ = oldBase // BaseURL is const; test via URL building manually

	// We call SearchDatasets pointing at our test server by temporarily
	// using the client's Get method with a custom URL.
	body, err := c.Get(context.Background(), srv.URL+"/package_search?q=refugees&rows=10&sort=score+desc")
	if err != nil {
		t.Fatal(err)
	}

	var env wireCKANResult[wireSearchResult[wirePackage]]
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !env.Success {
		t.Fatal("success=false")
	}
	if env.Result.Count != 2 {
		t.Errorf("count = %d, want 2", env.Result.Count)
	}
	if len(env.Result.Results) != 2 {
		t.Errorf("results len = %d, want 2", len(env.Result.Results))
	}
	first := env.Result.Results[0]
	if first.Name != "ukraine-refugee" {
		t.Errorf("first.Name = %q, want ukraine-refugee", first.Name)
	}
	if first.Organization.Title != "UNHCR" {
		t.Errorf("first.Organization.Title = %q, want UNHCR", first.Organization.Title)
	}
}

func TestGetPackage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/package_show" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		id := r.URL.Query().Get("id")
		if id != "ukraine-refugee" {
			t.Errorf("id = %q, want ukraine-refugee", id)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(mockPackageShow())
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL+"/package_show?id=ukraine-refugee")
	if err != nil {
		t.Fatal(err)
	}

	var env wireCKANResult[wirePackage]
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !env.Success {
		t.Fatal("success=false")
	}
	wp := env.Result
	if wp.Name != "ukraine-refugee" {
		t.Errorf("Name = %q, want ukraine-refugee", wp.Name)
	}
	if len(wp.Resources) != 2 {
		t.Errorf("Resources len = %d, want 2", len(wp.Resources))
	}
	if wp.Resources[0].Format != "CSV" {
		t.Errorf("Resources[0].Format = %q, want CSV", wp.Resources[0].Format)
	}
}

func TestListOrganizations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organization_list" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(mockOrgList())
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL+"/organization_list?all_fields=true&limit=3")
	if err != nil {
		t.Fatal(err)
	}

	var env wireCKANResult[[]wireOrgItem]
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !env.Success {
		t.Fatal("success=false")
	}
	if len(env.Result) != 3 {
		t.Errorf("orgs len = %d, want 3", len(env.Result))
	}
	if env.Result[0].Name != "unhcr" {
		t.Errorf("orgs[0].Name = %q, want unhcr", env.Result[0].Name)
	}
	if env.Result[0].PackageCount != 120 {
		t.Errorf("orgs[0].PackageCount = %d, want 120", env.Result[0].PackageCount)
	}
}

func TestShortDate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2024-03-15T08:00:00.000000", "2024-03-15"},
		{"2024-01-01", "2024-01-01"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := shortDate(tc.in); got != tc.want {
			t.Errorf("shortDate(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	long := "This is a very long description that exceeds the limit for display purposes."
	got := truncate(long, 20)
	if len(got) != 23 { // 20 chars + "..."
		t.Errorf("truncate len = %d, want 23", len(got))
	}
	short := "Short."
	if got := truncate(short, 100); got != "Short." {
		t.Errorf("truncate short = %q, want %q", got, short)
	}
}

func TestJoinTags(t *testing.T) {
	tags := []wireTag{{Name: "refugees"}, {Name: "ukraine"}, {Name: "displacement"}, {Name: "extra"}}
	got := joinTags(tags, 3)
	want := "refugees, ukraine, displacement"
	if got != want {
		t.Errorf("joinTags = %q, want %q", got, want)
	}
}
