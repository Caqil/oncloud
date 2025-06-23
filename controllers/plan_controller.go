package controllers

import (
	"net/http"
	"oncloud/services"
	"oncloud/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PlanController struct {
	planService *services.PlanService
}

func NewPlanController() *PlanController {
	return &PlanController{
		planService: services.NewPlanService(),
	}
}

// GetPlans returns list of available plans
func (pc *PlanController) GetPlans(c *gin.Context) {
	includeInactive := c.Query("include_inactive") == "true"

	plans, err := pc.planService.GetAvailablePlans(includeInactive)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get plans")
		return
	}

	utils.SuccessResponse(c, "Plans retrieved successfully", plans)
}

// GetPlan returns a specific plan
func (pc *PlanController) GetPlan(c *gin.Context) {
	planID := c.Param("id")
	if !utils.IsValidObjectID(planID) {
		utils.BadRequestResponse(c, "Invalid plan ID")
		return
	}

	objID, _ := utils.StringToObjectID(planID)
	plan, err := pc.planService.GetPlan(objID)
	if err != nil {
		utils.NotFoundResponse(c, "Plan not found")
		return
	}

	utils.SuccessResponse(c, "Plan retrieved successfully", plan)
}

// ComparePlans returns plan comparison data
func (pc *PlanController) ComparePlans(c *gin.Context) {
	planIDs := c.QueryArray("plan_ids")
	if len(planIDs) == 0 {
		// Return all active plans for comparison
		comparison, err := pc.planService.GetPlanComparison(nil)
		if err != nil {
			utils.InternalServerErrorResponse(c, "Failed to get plan comparison")
			return
		}
		utils.SuccessResponse(c, "Plan comparison retrieved successfully", comparison)
		return
	}

	// Validate plan IDs
	var objIDs []primitive.ObjectID
	for _, id := range planIDs {
		if !utils.IsValidObjectID(id) {
			utils.BadRequestResponse(c, "Invalid plan ID: "+id)
			return
		}
		objID, _ := utils.StringToObjectID(id)
		objIDs = append(objIDs, objID)
	}

	comparison, err := pc.planService.GetPlanComparison(objIDs)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get plan comparison")
		return
	}

	utils.SuccessResponse(c, "Plan comparison retrieved successfully", comparison)
}

// GetPricing returns pricing information
func (pc *PlanController) GetPricing(c *gin.Context) {
	// currency := c.DefaultQuery("currency", "USD")
	// billingCycle := c.DefaultQuery("billing_cycle", "monthly")

	pricing, err := pc.planService.GetPricing()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get pricing")
		return
	}

	utils.SuccessResponse(c, "Pricing retrieved successfully", pricing)
}

// GetUserPlan returns current user's plan
func (pc *PlanController) GetUserPlan(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	userPlan, err := pc.planService.GetUserPlan(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get user plan")
		return
	}

	utils.SuccessResponse(c, "User plan retrieved successfully", userPlan)
}

// Subscribe handles plan subscription
func (pc *PlanController) Subscribe(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		PlanID        string `json:"plan_id" validate:"required"`
		PaymentMethod string `json:"payment_method" validate:"required"`
		BillingCycle  string `json:"billing_cycle"`
		CouponCode    string `json:"coupon_code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if err := utils.ValidateStruct(&req); err != nil {
		utils.ValidationErrorResponse(c, err)
		return
	}

	if !utils.IsValidObjectID(req.PlanID) {
		utils.BadRequestResponse(c, "Invalid plan ID")
		return
	}

	planObjID, _ := utils.StringToObjectID(req.PlanID)
	subscription, err := pc.planService.Subscribe(user.ID, planObjID, req.PaymentMethod)
	if err != nil {
		utils.ErrorResponse(c, http.StatusPaymentRequired, err.Error(), nil)
		return
	}

	utils.CreatedResponse(c, "Subscription created successfully", subscription)
}

// UpgradePlan handles plan upgrade
func (pc *PlanController) UpgradePlan(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		NewPlanID     string `json:"new_plan_id" validate:"required"`
		PaymentMethod string `json:"payment_method"`
		BillingCycle  string `json:"billing_cycle"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if !utils.IsValidObjectID(req.NewPlanID) {
		utils.BadRequestResponse(c, "Invalid plan ID")
		return
	}

	newPlanObjID, _ := utils.StringToObjectID(req.NewPlanID)
	upgrade, err := pc.planService.UpgradePlan(user.ID, newPlanObjID, req.PaymentMethod)
	if err != nil {
		utils.ErrorResponse(c, http.StatusPaymentRequired, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Plan upgraded successfully", upgrade)
}

// DowngradePlan handles plan downgrade
func (pc *PlanController) DowngradePlan(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		NewPlanID string `json:"new_plan_id" validate:"required"`
		Immediate bool   `json:"immediate"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	if !utils.IsValidObjectID(req.NewPlanID) {
		utils.BadRequestResponse(c, "Invalid plan ID")
		return
	}

	newPlanObjID, _ := utils.StringToObjectID(req.NewPlanID)
	downgrade, err := pc.planService.DowngradePlan(user.ID, newPlanObjID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Plan downgrade scheduled successfully", downgrade)
}

// CancelSubscription handles subscription cancellation
func (pc *PlanController) CancelSubscription(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		Reason    string `json:"reason"`
		Immediate bool   `json:"immediate"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	cancellation, err := pc.planService.CancelSubscription(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, err.Error())
		return
	}

	utils.SuccessResponse(c, "Subscription cancelled successfully", cancellation)
}

// RenewSubscription handles subscription renewal
func (pc *PlanController) RenewSubscription(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		PaymentMethod string `json:"payment_method"`
		BillingCycle  string `json:"billing_cycle"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	renewal, err := pc.planService.RenewSubscription(user.ID, req.PaymentMethod, req.BillingCycle)
	if err != nil {
		utils.ErrorResponse(c, http.StatusPaymentRequired, err.Error(), nil)
		return
	}

	utils.SuccessResponse(c, "Subscription renewed successfully", renewal)
}

// GetBillingHistory returns user's billing history
func (pc *PlanController) GetBillingHistory(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	history, total, err := pc.planService.GetBillingHistory(user.ID, page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get billing history")
		return
	}

	utils.PaginatedResponse(c, "Billing history retrieved successfully", history, page, limit, total)
}

// GetInvoices returns user's invoices
func (pc *PlanController) GetInvoices(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	invoices, total, err := pc.planService.GetInvoices(user.ID, page, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get invoices")
		return
	}

	utils.PaginatedResponse(c, "Invoices retrieved successfully", invoices, page, limit, total)
}

// DownloadInvoice handles invoice download
func (pc *PlanController) DownloadInvoice(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	invoiceID := c.Param("id")
	if !utils.IsValidObjectID(invoiceID) {
		utils.BadRequestResponse(c, "Invalid invoice ID")
		return
	}

	objID, _ := utils.StringToObjectID(invoiceID)
	downloadURL, err := pc.planService.GetInvoiceDownloadURL(user.ID, objID)
	if err != nil {
		utils.NotFoundResponse(c, "Invoice not found")
		return
	}

	c.Redirect(http.StatusFound, downloadURL)
}

// Payment methods management
func (pc *PlanController) AddPaymentMethod(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	var req struct {
		Type      string            `json:"type" validate:"required"`  // card, paypal, bank
		Token     string            `json:"token" validate:"required"` // Payment gateway token
		IsDefault bool              `json:"is_default"`
		Metadata  map[string]string `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	paymentMethod, err := pc.planService.AddPaymentMethod(user.ID, req.Type, req.Token, req.IsDefault, req.Metadata)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to add payment method")
		return
	}

	utils.CreatedResponse(c, "Payment method added successfully", paymentMethod)
}

func (pc *PlanController) GetPaymentMethods(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	methods, err := pc.planService.GetPaymentMethods(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get payment methods")
		return
	}

	utils.SuccessResponse(c, "Payment methods retrieved successfully", methods)
}

func (pc *PlanController) UpdatePaymentMethod(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	methodID := c.Param("id")
	if !utils.IsValidObjectID(methodID) {
		utils.BadRequestResponse(c, "Invalid payment method ID")
		return
	}

	var req struct {
		IsDefault bool              `json:"is_default"`
		Metadata  map[string]string `json:"metadata"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request data")
		return
	}

	objID, _ := utils.StringToObjectID(methodID)
	err := pc.planService.UpdatePaymentMethod(user.ID, objID, req.IsDefault, req.Metadata)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to update payment method")
		return
	}

	utils.SuccessResponse(c, "Payment method updated successfully", nil)
}

func (pc *PlanController) DeletePaymentMethod(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	methodID := c.Param("id")
	if !utils.IsValidObjectID(methodID) {
		utils.BadRequestResponse(c, "Invalid payment method ID")
		return
	}

	objID, _ := utils.StringToObjectID(methodID)
	err := pc.planService.DeletePaymentMethod(user.ID, objID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to delete payment method")
		return
	}

	utils.SuccessResponse(c, "Payment method deleted successfully", nil)
}

// Usage tracking
func (pc *PlanController) GetUsage(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	usage, err := pc.planService.GetUsage(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get usage")
		return
	}

	utils.SuccessResponse(c, "Usage retrieved successfully", usage)
}

func (pc *PlanController) GetUsageHistory(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	period := c.DefaultQuery("period", "30") // days
	usageType := c.Query("type")             // storage, bandwidth, files

	history, err := pc.planService.GetUsageHistory(user.ID, period, usageType)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get usage history")
		return
	}

	utils.SuccessResponse(c, "Usage history retrieved successfully", history)
}

func (pc *PlanController) GetLimits(c *gin.Context) {
	user, exists := utils.GetUserFromContext(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User not found in context")
		return
	}

	limits, err := pc.planService.GetUserLimits(user.ID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get limits")
		return
	}

	utils.SuccessResponse(c, "Limits retrieved successfully", limits)
}

// Webhook handlers for payment processors
func (pc *PlanController) StripeWebhook(c *gin.Context) {
	signature := c.GetHeader("Stripe-Signature")
	if signature == "" {
		utils.BadRequestResponse(c, "Missing Stripe signature")
		return
	}

	payload, err := c.GetRawData()
	if err != nil {
		utils.BadRequestResponse(c, "Failed to read request body")
		return
	}

	err = pc.planService.HandleStripeWebhook(payload, signature)
	if err != nil {
		utils.BadRequestResponse(c, "Failed to process webhook")
		return
	}

	c.Status(200)
}
