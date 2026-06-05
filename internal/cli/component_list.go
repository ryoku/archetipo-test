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

// component is a minimal representation of the API response for a single component.
type component struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	GCRImagePath string `json:"gcr_image_path"`
}

// NewComponentListCmd returns the "kubegate component list" command.
func NewComponentListCmd(configDir string) *cobra.Command {
	var (
		apiURL       string
		outputFmt    string
		productSlug  string
	)

	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List components for a product",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			tok, err := ReadToken(configDir)
			if err != nil {
				return fmt.Errorf("reading stored token: %w", err)
			}

			path := "/api/v1/products/" + url.PathEscape(productSlug) + "/components"
			client := NewAPIClient(apiURL, tok)
			resp, err := client.Get(cmdContext(cmd), path)
			if err != nil {
				return fmt.Errorf("GET %s: %w", path, err)
			}
			defer resp.Body.Close()

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

			var components []component
			if err := json.Unmarshal(body, &components); err != nil {
				return fmt.Errorf("parsing response: %w", err)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NAME\tSLUG\tGCR IMAGE PATH")
			for _, c := range components {
				fmt.Fprintf(w, "%s\t%s\t%s\n", c.Name, c.Slug, c.GCRImagePath)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", envOrDefault("KUBEGATE_API_URL", "http://localhost:8081"), "KubeGate API base URL")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "", "Output format (json)")
	cmd.Flags().StringVar(&productSlug, "product", "", "Product slug")
	_ = cmd.MarkFlagRequired("product")

	return cmd
}
