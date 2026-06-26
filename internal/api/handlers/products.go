package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/store"
)

const (
	errMsgInternal = "internal error"
	errMsgNotFound = "not found"
)

// ProductHandlers bundles the HTTP handlers for the /products resource.
type ProductHandlers struct {
	store store.ProductStore
}

// NewProductHandlers returns a ProductHandlers wired to the given store.
func NewProductHandlers(s store.ProductStore) *ProductHandlers {
	return &ProductHandlers{store: s}
}

// createProductRequest is the body for POST /api/v1/products.
type createProductRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

// updateProductRequest is the body for PUT /api/v1/products/:productSlug.
type updateProductRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// productResponse is the API representation of a product.
type productResponse struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Slug             string  `json:"slug"`
	Description      string  `json:"description"`
	ArchivedAt       *string `json:"archived_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
	MyRole           string  `json:"my_role,omitempty"` // caller's effective role on this product
	LastDeployedAt   *string `json:"last_deployed_at"`
	HasProductionEnv bool    `json:"has_production_env"`
}

func toProductResponse(p *domain.Product) productResponse {
	r := productResponse{
		ID:               p.ID,
		Name:             p.Name,
		Slug:             p.Slug,
		Description:      p.Description,
		CreatedAt:        p.CreatedAt.Format(time.RFC3339),
		HasProductionEnv: p.HasProductionEnv,
	}
	if p.ArchivedAt != nil {
		s := p.ArchivedAt.Format(time.RFC3339)
		r.ArchivedAt = &s
	}
	if p.LastDeployedAt != nil {
		s := p.LastDeployedAt.Format(time.RFC3339)
		r.LastDeployedAt = &s
	}
	return r
}

// checkProductAccess verifies that a valid identity exists in the context and
// that the caller has at least some role on slug (or is a DevOps Admin).
// It writes the appropriate error response and returns false when access is
// denied, so the caller can return immediately.
func checkProductAccess(c *gin.Context, slug string) bool {
	identity, ok := middleware.IdentityFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return false
	}
	if !identity.IsDevOpsAdmin {
		if _, hasRole := identity.ProductRoles[slug]; !hasRole {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return false
		}
	}
	return true
}

// validateURLSlug checks that slug is a valid product slug coming from a URL
// parameter. It writes a 400 and returns false when validation fails.
func validateURLSlug(c *gin.Context, slug string) bool {
	if err := domain.ValidateSlug(slug); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product slug in URL"})
		return false
	}
	return true
}

// resolveProduct fetches the product by slug and writes an appropriate error
// response if not found, inaccessible, or archived. Returns (nil, false) when
// a response has already been written, (*Product, true) on success.
func resolveProduct(c *gin.Context, ps store.ProductStore, slug string) (*domain.Product, bool) {
	product, err := ps.GetBySlug(c.Request.Context(), slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return nil, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return nil, false
	}
	if product.ArchivedAt != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
		return nil, false
	}
	return product, true
}

// CreateProduct handles POST /api/v1/products.
// Reserved for DevOps Admin — enforced by RequireAdmin middleware upstream.
func (h *ProductHandlers) CreateProduct(c *gin.Context) {
	var req createProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "name is required"})
		return
	}
	if err := domain.ValidateSlug(req.Slug); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	p := &domain.Product{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
	}
	if err := h.store.Create(c.Request.Context(), p); err != nil {
		if errors.Is(err, store.ErrSlugConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "slug already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	c.JSON(http.StatusCreated, toProductResponse(p))
}

// ListProducts handles GET /api/v1/products.
// DevOps Admin receives all products; other users receive only products they have a role on.
func (h *ProductHandlers) ListProducts(c *gin.Context) {
	identity, ok := middleware.IdentityFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	includeArchived := c.Query("include_archived") == "true"

	opts := store.ListOptions{IncludeArchived: includeArchived}
	if !identity.IsDevOpsAdmin {
		allowlist := make(map[string]struct{}, len(identity.ProductRoles))
		for slug := range identity.ProductRoles {
			allowlist[slug] = struct{}{}
		}
		opts.SlugAllowlist = allowlist
	}

	products, err := h.store.List(c.Request.Context(), opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	resp := make([]productResponse, len(products))
	for i := range products {
		resp[i] = toProductResponse(&products[i])
		if identity.IsDevOpsAdmin {
			resp[i].MyRole = "admin"
		} else if role, ok := identity.ProductRoles[products[i].Slug]; ok {
			resp[i].MyRole = role
		}
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateProduct handles PUT /api/v1/products/:productSlug.
// Allows updating name and description; slug is immutable.
// Access controlled by RequireRole(RoleEditor) middleware upstream.
func (h *ProductHandlers) UpdateProduct(c *gin.Context) {
	slug := c.Param("productSlug")
	if !checkProductAccess(c, slug) || !validateURLSlug(c, slug) {
		return
	}

	var req updateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "name is required"})
		return
	}

	updated, err := h.store.Update(c.Request.Context(), slug, req.Name, req.Description)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	c.JSON(http.StatusOK, toProductResponse(updated))
}

// ArchiveProduct handles DELETE /api/v1/products/:productSlug.
// Performs a soft delete by setting archived_at.
// Access controlled by RequireRole(RoleEditor) middleware upstream.
func (h *ProductHandlers) ArchiveProduct(c *gin.Context) {
	slug := c.Param("productSlug")
	if !checkProductAccess(c, slug) || !validateURLSlug(c, slug) {
		return
	}

	if err := h.store.Archive(c.Request.Context(), slug); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	c.Status(http.StatusNoContent)
}
