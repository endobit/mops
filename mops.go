// Package ops implements the metal ops service.
package mops

import (
	"net/http"
	"strings"
)

// DefaultPort is the default port to listen on. It can be overridden with the
// --port flag.
const DefaultPort = 8888

// ReportScope represents the scope of a report, which can be filtered by
// zone, cluster, and host. It provides a way to build a query string for
// generating reports based on these parameters.
type ReportScope struct {
	Zone    string
	Cluster string
	Host    string
}

// Query returns the query string for the report scope. If no parameters are
// set, it returns an empty string.
func (r ReportScope) Query() string {
	query := []string{}

	if r.Zone != "" {
		query = append(query, "zone="+r.Zone)
	}

	if r.Cluster != "" {
		query = append(query, "cluster="+r.Cluster)
	}

	if r.Host != "" {
		query = append(query, "host="+r.Host)
	}

	if len(query) == 0 {
		return ""
	}

	return "?" + strings.Join(query, "&")
}

// From populates the ReportScope from an HTTP request. It extracts the "zone",
// "cluster", and "host" parameters from the query string of the request URL and
// assigns them to the corresponding fields in the ReportScope.
func (r *ReportScope) From(req *http.Request) {
	params := req.URL.Query()

	r.Zone = params.Get("zone")
	r.Cluster = params.Get("cluster")
	r.Host = params.Get("host")
}
