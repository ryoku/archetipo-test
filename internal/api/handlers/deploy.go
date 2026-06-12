package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryoku/kubegate/internal/api/middleware"
	"github.com/ryoku/kubegate/internal/domain"
	"github.com/ryoku/kubegate/internal/gitops"
	"github.com/ryoku/kubegate/internal/store"
)

type GitOpsApplier interface {
	Apply(ctx context.Context, p gitops.ApplyParams) error
}

type DeploymentHandlers struct {
	productStore         store.ProductStore
	envStore             store.EnvironmentStore
	lockStore            store.DeploymentLockStore
	applier              GitOpsApplier
	lockTimeout          time.Duration
	defaultTagConvention string
}

func NewDeploymentHandlers(
	productStore store.ProductStore,
	envStore store.EnvironmentStore,
	lockStore store.DeploymentLockStore,
	applier GitOpsApplier,
	defaultTagConvention string,
) *DeploymentHandlers {
	return &DeploymentHandlers{
		productStore:         productStore,
		envStore:             envStore,
		lockStore:            lockStore,
		applier:              applier,
		lockTimeout:          deploymentLockTimeout(),
		defaultTagConvention: defaultTagConvention,
	}
}

func deploymentLockTimeout() time.Duration {
	n, err := strconv.Atoi(os.Getenv("DEPLOYMENT_LOCK_TIMEOUT_SECONDS"))
	if err != nil || n <= 0 {
		return 5 * time.Second
	}
	return time.Duration(n) * time.Second
}

type deployRequest struct {
	Workload string `json:"workload"`
	Tag      string `json:"tag"`
}

type deploymentLockResponse struct {
	Error       string `json:"error"`
	LockHolder  string `json:"lock_holder"`
	LockedSince string `json:"locked_since"`
}

type tagConventionRejectionResponse struct {
	RejectedTag  string `json:"rejected_tag"`
	AppliedRegex string `json:"applied_regex"`
	Message      string `json:"message"`
}

func (h *DeploymentHandlers) Deploy(c *gin.Context) {
	productSlug := c.Param("productSlug")
	environmentID := c.Param("environmentID")

	if !checkProductAccess(c, productSlug) || !validateURLSlug(c, productSlug) {
		return
	}

	identity, ok := middleware.IdentityFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	req, ok := bindDeployRequest(c)
	if !ok {
		return
	}

	product, env, ok := h.resolveDeployTargets(c, productSlug, environmentID)
	if !ok {
		return
	}

	if !h.checkTagConvention(c, product, env, req.Tag) {
		return
	}

	actor := actorName(identity)
	lock, ok := h.acquireLock(c, product.ID, env.ID, actor)
	if !ok {
		return
	}
	defer func() {
		if err := lock.Release(context.Background()); err != nil {
			log.Printf("deploy lock release: product=%s env=%s: %v", productSlug, environmentID, err)
		}
	}()

	if !h.applyGitOps(c, product, env, req.Workload, req.Tag, actor) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"product":     product.Slug,
		"workload":    req.Workload,
		"env":         env.Name,
		"tag":         req.Tag,
		"deployed_by": actor,
	})
}

func bindDeployRequest(c *gin.Context) (deployRequest, bool) {
	var req deployRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return req, false
	}
	if req.Workload == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "workload is required"})
		return req, false
	}
	if req.Tag == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "tag is required"})
		return req, false
	}
	return req, true
}

func (h *DeploymentHandlers) resolveDeployTargets(c *gin.Context, productSlug, environmentID string) (*domain.Product, *domain.Environment, bool) {
	product, ok := resolveProduct(c, h.productStore, productSlug)
	if !ok {
		return nil, nil, false
	}
	env, err := h.envStore.GetByID(c.Request.Context(), product.ID, environmentID)
	if err != nil {
		if errors.Is(err, store.ErrEnvironmentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsgNotFound})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		}
		return nil, nil, false
	}
	return product, env, true
}

func (h *DeploymentHandlers) acquireLock(c *gin.Context, productID, envID, actor string) (store.AcquiredLock, bool) {
	lock, holder, err := h.lockStore.TryAcquire(c.Request.Context(), productID, envID, actor, h.lockTimeout)
	if err != nil {
		log.Printf("acquireLock product=%s env=%s: %v", productID, envID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return nil, false
	}
	if lock == nil {
		resp := deploymentLockResponse{Error: "deployment in progress"}
		if holder != nil {
			resp.LockHolder = holder.LockHolder
			resp.LockedSince = holder.LockedSince.UTC().Format(time.RFC3339)
		}
		c.JSON(http.StatusConflict, resp)
		return nil, false
	}
	return lock, true
}

func (h *DeploymentHandlers) applyGitOps(c *gin.Context, product *domain.Product, env *domain.Environment, workload, tag, actor string) bool {
	helmReleasePath := gitops.HelmReleasePath(env.Slug, product.Slug)
	err := h.applier.Apply(c.Request.Context(), gitops.ApplyParams{
		HelmReleasePath: helmReleasePath,
		Workload:        workload,
		NewTag:          tag,
		ProductSlug:     product.Slug,
		EnvName:         env.Name,
		Actor:           actor,
	})
	if err == nil {
		return true
	}
	if errors.Is(err, gitops.ErrGitOpsNotConfigured) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "deployments are not available on this server"})
		return false
	}
	var notFound *gitops.HelmReleaseNotFoundError
	if errors.As(err, &notFound) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("HelmRelease not found: %s", notFound.Path)})
		return false
	}
	var pathErr *gitops.HelmReleasePathError
	if errors.As(err, &pathErr) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("workload not found in HelmRelease: %s", pathErr.Path)})
		return false
	}
	var inputErr *gitops.PatchInputError
	if errors.As(err, &inputErr) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("invalid %s: %s", inputErr.Field, inputErr.Reason)})
		return false
	}
	log.Printf("applyGitOps product=%s workload=%s env=%s tag=%s: %v", product.Slug, workload, env.Name, tag, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
	return false
}

func (h *DeploymentHandlers) checkTagConvention(c *gin.Context, product *domain.Product, env *domain.Environment, tag string) bool {
	violation, err := domain.CheckTagConvention(tag, env.Type, product.TagConventionRegex, h.defaultTagConvention)
	if err != nil {
		log.Printf("checkTagConvention product=%s env=%s tag=%s: %v", product.Slug, env.Name, tag, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return false
	}
	if violation != nil {
		c.JSON(http.StatusUnprocessableEntity, tagConventionRejectionResponse{
			RejectedTag:  violation.RejectedTag,
			AppliedRegex: violation.AppliedRegex,
			Message:      fmt.Sprintf("tag '%s' does not match the required production tag convention (%s)", violation.RejectedTag, violation.AppliedRegex),
		})
		return false
	}
	return true
}

func actorName(identity *domain.UserIdentity) string {
	if identity.Name != "" {
		return identity.Name
	}
	return identity.Email
}
