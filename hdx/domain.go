package hdx

import (
	"context"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes hdx as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/hdx-cli/hdx"
//
// The init below registers it; the host then dereferences hdx:// URIs by
// routing to the operations Register installs. The same Domain also builds
// the standalone hdx binary (see cli.NewApp), so the binary and a host share
// one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the hdx driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "hdx",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "hdx",
			Short:  "A command line for the Humanitarian Data Exchange (HDX).",
			Long: `A command line for the Humanitarian Data Exchange (HDX).

hdx reads public humanitarian data from data.humdata.org over plain HTTPS,
shapes it into clean records, and prints output that pipes into the rest of
your tools. No API key, nothing to run alongside it.`,
			Site: Host,
			Repo: "https://github.com/tamnd/hdx-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// search op: keyword search over HDX datasets.
	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "read",
		Summary: "Search HDX datasets by keyword",
		Args:    []kit.Arg{{Name: "query", Help: "search keyword"}},
	}, searchDatasets)

	// package op: show a dataset and all its resources.
	kit.Handle(app, kit.OpMeta{
		Name:    "package",
		Group:   "read",
		List:    true,
		Summary: "Show a dataset and its resources",
		Args:    []kit.Arg{{Name: "name", Help: "dataset name/slug (e.g. ukraine-refugee)"}},
	}, getPackage)

	// organization op: list organizations on HDX.
	kit.Handle(app, kit.OpMeta{
		Name:    "organization",
		Group:   "read",
		Summary: "List organizations on HDX",
	}, listOrganizations)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type searchInput struct {
	Query  string  `kit:"arg" help:"search keyword"`
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Org    string  `kit:"flag" help:"filter by organization name slug"`
	Client *Client `kit:"inject"`
}

type packageInput struct {
	Name   string  `kit:"arg" help:"dataset name/slug"`
	Client *Client `kit:"inject"`
}

type organizationInput struct {
	Limit  int     `kit:"flag,inherit" help:"max results"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchDatasets(ctx context.Context, in searchInput, emit func(*Dataset) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}
	datasets, err := in.Client.SearchDatasets(ctx, in.Query, limit, in.Org)
	if err != nil {
		return mapErr(err)
	}
	for _, d := range datasets {
		if err := emit(d); err != nil {
			return err
		}
	}
	return nil
}

func getPackage(ctx context.Context, in packageInput, emit func(any) error) error {
	pkg, resources, err := in.Client.GetPackage(ctx, in.Name)
	if err != nil {
		return mapErr(err)
	}
	if err := emit(pkg); err != nil {
		return err
	}
	for _, r := range resources {
		if err := emit(r); err != nil {
			return err
		}
	}
	return nil
}

func listOrganizations(ctx context.Context, in organizationInput, emit func(*Organization) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	orgs, err := in.Client.ListOrganizations(ctx, limit)
	if err != nil {
		return mapErr(err)
	}
	for _, o := range orgs {
		if err := emit(o); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver: pure string functions, no network ---

// Classify turns a dataset name or HDX URL into a (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if u, e := parseHDXURL(input); e == nil && u != "" {
		return "package", u, nil
	}
	if input != "" {
		return "package", input, nil
	}
	return "", "", errs.Usage("unrecognized hdx reference: %q", input)
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "package":
		return "https://" + Host + "/dataset/" + id, nil
	default:
		return "", errs.Usage("hdx has no resource type %q", uriType)
	}
}

// --- helpers ---

// parseHDXURL extracts a dataset name from a full data.humdata.org URL.
// e.g. https://data.humdata.org/dataset/ukraine-refugee -> "ukraine-refugee"
func parseHDXURL(input string) (string, error) {
	if !strings.Contains(input, "humdata.org") {
		return "", errs.Usage("not an HDX URL")
	}
	parts := strings.Split(strings.TrimSuffix(input, "/"), "/dataset/")
	if len(parts) == 2 && parts[1] != "" {
		return parts[1], nil
	}
	return "", errs.Usage("cannot extract dataset from URL: %q", input)
}

// mapErr converts a library error into the kit error kind that carries the
// right exit code.
func mapErr(err error) error {
	return err
}
