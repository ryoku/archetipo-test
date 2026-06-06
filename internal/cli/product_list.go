package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// product is a minimal representation of the API response for a single product.
type product struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

func listProducts(cmd *cobra.Command, configDir, apiURL, outputFmt string) error {
	tok, err := ReadToken(configDir)
	if err != nil {
		return fmt.Errorf("reading stored token: %w", err)
	}

	client := NewAPIClient(apiURL, tok)
	resp, err := client.Get(cmdContext(cmd), "/api/v1/products")
	if err != nil {
		return fmt.Errorf("GET /api/v1/products: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// ok
	case http.StatusUnauthorized:
		return fmt.Errorf("session expired, please run `kubegate login`")
	case http.StatusForbidden:
		return fmt.Errorf("access denied: you do not have permission to list products")
	default:
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if outputFmt != "" && outputFmt != "json" {
		return fmt.Errorf("unsupported output format %q: supported values: json", outputFmt)
	}
	if outputFmt == "json" {
		_, err = fmt.Fprint(cmd.OutOrStdout(), string(body))
		return err
	}

	var products []product
	if err := json.Unmarshal(body, &products); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tSLUG\tDESCRIPTION"); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}
	for _, p := range products {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.Slug, p.Description); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}
	return w.Flush()
}

// NewProductListCmd returns the "kubegate product list" command.
func NewProductListCmd(configDir string) *cobra.Command {
	var (
		apiURL    string
		outputFmt string
	)

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List products",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listProducts(cmd, configDir, apiURL, outputFmt)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", envOrDefault("KUBEGATE_API_URL", "http://localhost:8081"), "KubeGate API base URL")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "", "Output format (json)")

	return cmd
}
