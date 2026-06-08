package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

type createEnvRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	OverlayPath string `json:"overlay_path"`
}

func createEnvironment(cmd *cobra.Command, configDir, apiURL, outputFmt, productSlug, name, envType, overlay string) error {
	if outputFmt != "" && outputFmt != "json" {
		return fmt.Errorf("unsupported output format %q: supported values: json", outputFmt)
	}

	tok, err := ReadToken(configDir)
	if err != nil {
		return fmt.Errorf("reading stored token: %w", err)
	}

	payload, err := json.Marshal(createEnvRequest{Name: name, Type: envType, OverlayPath: overlay})
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	path := "/api/v1/products/" + url.PathEscape(productSlug) + "/environments"
	client := NewAPIClient(apiURL, tok)
	resp, err := client.Post(cmdContext(cmd), path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		// ok
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		var apiErr struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error != "" {
			return fmt.Errorf("validation error: %s", apiErr.Error)
		}
		return fmt.Errorf("validation error: %s", string(body))
	case http.StatusConflict:
		return fmt.Errorf("environment name already exists for product %q", productSlug)
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

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "environment %q created for product %q\n", name, productSlug)
	return err
}

// NewEnvCreateCmd returns the "kubegate env create" command.
func NewEnvCreateCmd(configDir string) *cobra.Command {
	var (
		apiURL      string
		outputFmt   string
		productSlug string
		name        string
		envType     string
		overlay     string
	)

	cmd := &cobra.Command{
		Use:          "create",
		Short:        "Create an environment for a product",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return createEnvironment(cmd, configDir, apiURL, outputFmt, productSlug, name, envType, overlay)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", envOrDefault("KUBEGATE_API_URL", "http://localhost:8081"), "KubeGate API base URL")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "", "Output format (json)")
	cmd.Flags().StringVar(&productSlug, "product", "", "Product slug")
	cmd.Flags().StringVar(&name, "name", "", "Environment name")
	cmd.Flags().StringVar(&envType, "type", "", "Environment type (dev, integration, production)")
	cmd.Flags().StringVar(&overlay, "overlay", "", "Gitops overlay path (relative)")
	_ = cmd.MarkFlagRequired("product")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("overlay")

	return cmd
}
