package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// environment is a minimal representation of the API response for a single environment.
type environment struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	GitopsPath string `json:"gitops_path"`
}

func listEnvironments(cmd *cobra.Command, configDir, apiURL, outputFmt, productSlug string) error {
	if outputFmt != "" && outputFmt != "json" {
		return fmt.Errorf("unsupported output format %q: supported values: json", outputFmt)
	}

	tok, err := ReadToken(configDir)
	if err != nil {
		return fmt.Errorf("reading stored token: %w", err)
	}

	path := "/api/v1/products/" + url.PathEscape(productSlug) + "/environments"
	client := NewAPIClient(apiURL, tok)
	resp, err := client.Get(cmdContext(cmd), path)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusNotFound:
		return fmt.Errorf("product not found: %s", productSlug)
	case http.StatusUnauthorized:
		return fmt.Errorf("session expired, please run `kubegate login`")
	case http.StatusForbidden:
		return fmt.Errorf("access denied: you do not have permission to access this product")
	default:
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if outputFmt == "json" {
		_, err = fmt.Fprint(cmd.OutOrStdout(), string(body))
		return err
	}

	var envs []environment
	if err := json.Unmarshal(body, &envs); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tTYPE\tGITOPS PATH"); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	for _, e := range envs {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", e.Name, e.Type, e.GitopsPath); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}
	return w.Flush()
}

// NewEnvListCmd returns the "kubegate env list" command.
func NewEnvListCmd(configDir string) *cobra.Command {
	var (
		apiURL      string
		outputFmt   string
		productSlug string
	)

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List environments for a product",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listEnvironments(cmd, configDir, apiURL, outputFmt, productSlug)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", envOrDefault("KUBEGATE_API_URL", "http://localhost:8081"), "KubeGate API base URL")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "", "Output format (json)")
	cmd.Flags().StringVar(&productSlug, "product", "", "Product slug")
	if err := cmd.MarkFlagRequired("product"); err != nil {
		panic(fmt.Sprintf("env list: MarkFlagRequired product: %v", err))
	}

	return cmd
}
