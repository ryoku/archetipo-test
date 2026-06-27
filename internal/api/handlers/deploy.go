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
	Apply(ctx context.Context, p gitops.ApplyParams) (string, error)
}

type DeploymentHandlers struct {
	productStore         store.ProductStore
	envStore             store.EnvironmentStore
	lockStore            store.DeploymentLockStore
	deploymentStore      store.DeploymentStore
	applier              GitOpsApplier
	lockTimeout          time.Duration
	defaultTagConvention string
}

func NewDeploymentHandlers(
	productStore store.ProductStore,
	envStore store.EnvironmentStore,
	lockStore store.DeploymentLockStore,
	deploymentStore store.DeploymentStore,
	applier GitOpsApplier,
	defaultTagConvention string,
) *DeploymentHandlers {
	if deploymentStore == nil {
		panic("NewDeploymentHandlers: deploymentStore must not be nil")
	}
	return &DeploymentHandlers{
		productStore:         productStore,
		envStore:             envStore,
		lockStore:            lockStore,
		deploymentStore:      deploymentStore,
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

	inProgressRecord := &domain.Deployment{
		ActorDisplayName: actor,
		ProductID:        product.ID,
		EnvironmentID:    env.ID,
		ComponentName:    req.Workload,
		EnvironmentName:  env.Name,
		Tag:              req.Tag,
		Outcome:          domain.OutcomeInProgress,
	}
	var inProgressID string
	if err := h.deploymentStore.Create(c.Request.Context(), inProgressRecord); err != nil {
		log.Printf("deploy: create in_progress record product=%s env=%s: %v", productSlug, environmentID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}
	inProgressID = inProgressRecord.ID

	commitSHA, applyErrMsg, ok := h.applyGitOps(c, product, env, req.Workload, req.Tag, actor)

	if applyErrMsg != "" {
		// Internal gitops failure: update in_progress record to failure and return.
		// applyGitOps has already written the HTTP 500 response.
		h.updateOutcome(c.Request.Context(), inProgressID, domain.OutcomeFailure, nil, &applyErrMsg)
		return
	}
	if !ok {
		// Non-recordable failure (server not configured, HelmRelease/workload not found,
		// invalid patch input): the deploy never actually started, so remove the
		// in_progress record rather than persisting a spurious failure that would
		// pollute the activity feed and deployment history.
		h.deleteInProgress(c.Request.Context(), inProgressID)
		return
	}

	h.updateOutcome(c.Request.Context(), inProgressID, domain.OutcomeSuccess, &commitSHA, nil)

	c.JSON(http.StatusAccepted, gin.H{"deployment_id": inProgressID})
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

// applyGitOps clones, patches, and pushes the gitops repo. It always writes the HTTP error
// response when returning ok=false. errMsg is non-empty only for internal (recordable) failures;
// non-recordable failures (misconfigured server, HelmRelease not found) return errMsg="".
func (h *DeploymentHandlers) applyGitOps(c *gin.Context, product *domain.Product, env *domain.Environment, workload, tag, actor string) (commitSHA, errMsg string, ok bool) {
	helmReleasePath := gitops.HelmReleasePath(env.Slug, product.Slug)
	sha, err := h.applier.Apply(c.Request.Context(), gitops.ApplyParams{
		HelmReleasePath: helmReleasePath,
		Workload:        workload,
		NewTag:          tag,
		ProductSlug:     product.Slug,
		EnvName:         env.Name,
		Actor:           actor,
	})
	if err == nil {
		return sha, "", true
	}
	if errors.Is(err, gitops.ErrGitOpsNotConfigured) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "deployments are not available on this server"})
		return "", "", false
	}
	var notFound *gitops.HelmReleaseNotFoundError
	if errors.As(err, &notFound) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("HelmRelease not found: %s", notFound.Path)})
		return "", "", false
	}
	var pathErr *gitops.HelmReleasePathError
	if errors.As(err, &pathErr) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("workload not found in HelmRelease: %s", pathErr.Path)})
		return "", "", false
	}
	var inputErr *gitops.PatchInputError
	if errors.As(err, &inputErr) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("invalid %s: %s", inputErr.Field, inputErr.Reason)})
		return "", "", false
	}
	log.Printf("applyGitOps product=%s workload=%s env=%s tag=%s: %v", product.Slug, workload, env.Name, tag, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
	// Store a generic failure category rather than the raw error, which may contain
	// internal details such as repo URLs or auth scheme from go-git.
	return "", "gitops apply failed", false
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

// updateOutcome updates the outcome of an existing deployment record best-effort.
// If id is empty (record was never created) the call is a no-op.
func (h *DeploymentHandlers) updateOutcome(ctx context.Context, id, outcome string, commitSHA *string, errorMessage *string) {
	if id == "" {
		return
	}
	if err := h.deploymentStore.UpdateOutcome(ctx, id, outcome, commitSHA, errorMessage); err != nil {
		if outcome == domain.OutcomeSuccess {
			log.Printf("updateOutcome: gitops succeeded but outcome record stuck as in_progress id=%s (will be swept by stale cleaner): %v", id, err)
		} else {
			log.Printf("updateOutcome id=%s outcome=%s: %v", id, outcome, err)
		}
	}
}

// deleteInProgress removes a previously created in_progress record best-effort.
// If id is empty (record was never created) the call is a no-op.
func (h *DeploymentHandlers) deleteInProgress(ctx context.Context, id string) {
	if id == "" {
		return
	}
	if err := h.deploymentStore.Delete(ctx, id); err != nil {
		log.Printf("deleteInProgress id=%s: %v", id, err)
	}
}

// GetDeployment handles GET /api/v1/deployments/:deploymentID.
func (h *DeploymentHandlers) GetDeployment(c *gin.Context) {
	deploymentID := c.Param("deploymentID")

	d, err := h.deploymentStore.GetByID(c.Request.Context(), deploymentID)
	if err != nil {
		if errors.Is(err, store.ErrDeploymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
			return
		}
		log.Printf("GetDeployment id=%s: %v", deploymentID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	product, err := h.productStore.GetByID(c.Request.Context(), d.ProductID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			log.Printf("GetDeployment id=%s: orphaned deployment — product id=%s not found", deploymentID, d.ProductID)
		} else {
			log.Printf("GetDeployment id=%s resolve product id=%s: %v", deploymentID, d.ProductID, err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsgInternal})
		return
	}

	if !checkProductAccess(c, product.Slug) {
		return
	}

	c.JSON(http.StatusOK, toDeploymentResponse(*d))
}
