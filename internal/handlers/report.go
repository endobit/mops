package handlers

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"

	"endobit.io/metal"
	pb "endobit.io/metal/gen/go/proto/metal/v1"
	"endobit.io/mops"
)

//go:embed reports/*.tmpl
var reports embed.FS

type Reporter struct {
	MetalDialer func() (*metal.Client, error)
	Logger      *slog.Logger
	initialized bool
	tmpl        *template.Template
}

// ServeHTTP implements the http.Handler interface.
//
//	@Summary		report template
//	@Description	Renders a report from the named template.
//	@Parm			zone    query string false "zone name"
//	@Parm			cluster query string false "cluster name"
//	@Parm			host    query string false "host name"
//	@Success		200	{object}	mops.GetReportResponse "report response"
//	@Router			/report/{name} [get]
func (r *Reporter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if !r.initialized {
		if err := r.init(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	var scope mops.ReportScope
	scope.From(req)

	name := req.PathValue("name")

	b, err := r.report(scope, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := mops.GetReportResponse{
		Report: string(b),
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

func (r *Reporter) report(scope mops.ReportScope, template string) ([]byte, error) {
	m, err := r.MetalDialer()
	defer func() {
		if err := m.Close(); err != nil {
			r.Logger.Error("failed to close metal client", "error", err)
		}
	}()

	if err != nil {
		return nil, fmt.Errorf("failed to dial metal client: %w", err)
	}

	ctx := m.Context() // grabs the token

	var req pb.ReadReportDataRequest

	if scope.Zone != "" {
		req.SetZone(scope.Zone)
	}
	if scope.Cluster != "" {
		req.SetCluster(scope.Cluster)
	}
	if scope.Host != "" {
		req.SetHost(scope.Host)
	}

	resp, err := m.ReadReportData(ctx, &req)
	if err != nil {
		return nil, err
	}

	var (
		report metal.Report
		buf    bytes.Buffer
	)

	if err := json.Unmarshal(resp.GetData(), &report); err != nil {
		return nil, err
	}

	if err := r.tmpl.ExecuteTemplate(&buf, template+".tmpl", report); err != nil {
		return nil, fmt.Errorf("failed to execute template %q: %w", template, err)
	}

	return buf.Bytes(), nil
}

func (r *Reporter) init() error {
	tmpl := addFunctions(template.New("mops"))

	repfs, err := fs.Sub(reports, "reports")
	if err != nil {
		return err
	}

	if err := logFiles(repfs, r.Logger.WithGroup("templates")); err != nil {
		return err
	}

	tmpl, err = tmpl.ParseFS(repfs, "*.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	r.tmpl = tmpl
	r.initialized = true

	return nil
}

// logFiles recursively logs every file and directory in the provided filesystem using the provided logger.
func logFiles(fsys fs.FS, logger *slog.Logger) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			logger.Info("found", "file", path)
		}
		return nil
	})
}

func addFunctions(tmpl *template.Template) *template.Template {
	funcs := sprig.TxtFuncMap()
	funcs["include"] = includeTemplate(tmpl)
	funcs["netmask"] = cidrNetmask
	funcs["address"] = cidrAddress

	return tmpl.Funcs(funcs)
}

// includeTemplate evaluates the named template with the provided data and returns the result as a string.
func includeTemplate(tmpl *template.Template) func(name string, data any) (string, error) {
	// The closure captures the template instance so it can be used inside the
	// template engine. I think Helm does the same thing (and also calls the
	// function include).
	return func(name string, data any) (string, error) {
		var buf strings.Builder

		err := tmpl.ExecuteTemplate(&buf, name, data)
		return buf.String(), err
	}
}

func cidrAddress(cidr string) (string, error) {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("failed to parse CIDR %q: %w", cidr, err)
	}

	return ip.String(), nil
}

func cidrNetmask(cidr string) (string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("failed to parse CIDR %q: %w", cidr, err)
	}

	mask := net.IP(ipnet.Mask).String()

	return mask, nil
}
