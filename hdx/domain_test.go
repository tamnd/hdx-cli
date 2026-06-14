package hdx

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// Offline tests for the URI driver's pure string functions and host wiring.
// Client HTTP behavior is in hdx_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "hdx" {
		t.Errorf("Scheme = %q, want hdx", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "hdx" {
		t.Errorf("Identity.Binary = %q, want hdx", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"ukraine-refugee", "package", "ukraine-refugee"},
		{"south-sudan-food-security", "package", "south-sudan-food-security"},
		{"https://data.humdata.org/dataset/ukraine-refugee", "package", "ukraine-refugee"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("package", "ukraine-refugee")
	want := "https://data.humdata.org/dataset/ukraine-refugee"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	p := &Dataset{
		Name:         "ukraine-refugee",
		Title:        "Ukraine - Refugee Situation",
		Organization: "UNHCR",
		Resources:    3,
		Modified:     "2024-03-15",
		Tags:         "refugees, ukraine",
	}
	u, err := h.Mint(p)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	// kit derives the URI authority from the lowercase type name: Dataset -> "dataset"
	if want := "hdx://dataset/ukraine-refugee"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("hdx", "ukraine-refugee")
	if err != nil || got.String() != "hdx://package/ukraine-refugee" {
		t.Errorf("ResolveOn = (%q, %v), want hdx://package/ukraine-refugee", got.String(), err)
	}
}
