package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"oncloud/database"
	"oncloud/models"
	"oncloud/utils"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type PlanService struct {
	planCollection         *mongo.Collection
	userCollection         *mongo.Collection
	subscriptionCollection *mongo.Collection
	usageCollection        *mongo.Collection
	billingCollection      *mongo.Collection
	invoiceCollection      *mongo.Collection
}

func NewPlanService() *PlanService {
	return &PlanService{
		planCollection:         database.GetCollection("plans"),
		userCollection:         database.GetCollection("users"),
		subscriptionCollection: database.GetCollection("subscriptions"),
		usageCollection:        database.GetCollection("usage_tracking"),
		billingCollection:      database.GetCollection("billing_history"),
		invoiceCollection:      database.GetCollection("invoices"),
	}
}

// Public Plan Operations (for users)
func (ps *PlanService) GetPlans() ([]models.Plan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := ps.planCollection.Find(ctx, bson.M{"is_active": true},
		options.Find().SetSort(bson.M{"sort_order": 1, "price": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var plans []models.Plan
	if err = cursor.All(ctx, &plans); err != nil {
		return nil, err
	}

	return plans, nil
}

func (ps *PlanService) GetPlan(planID primitive.ObjectID) (*models.Plan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var plan models.Plan
	err := ps.planCollection.FindOne(ctx, bson.M{
		"_id":       planID,
		"is_active": true,
	}).Decode(&plan)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %v", err)
	}

	return &plan, nil
}

// Plan Service - GetPlansForAdmin Function
func (ps *PlanService) GetPlansForAdmin(page, limit int, includeInactive bool) ([]models.Plan, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{}
	if !includeInactive {
		filter["is_active"] = true
	}

	skip := (page - 1) * limit

	// Get total count
	total, err := ps.planCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Get plans with pagination
	cursor, err := ps.planCollection.Find(ctx, filter,
		options.Find().
			SetSkip(int64(skip)).
			SetLimit(int64(limit)).
			SetSort(bson.M{"sort_order": 1, "created_at": -1}),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var plans []models.Plan
	if err = cursor.All(ctx, &plans); err != nil {
		return nil, 0, err
	}

	return plans, total, nil
}

func (ps *PlanService) ComparePlans() ([]models.Plan, error) {
	plans, err := ps.GetPlans()
	if err != nil {
		return nil, err
	}

	return plans, nil
}

func (ps *PlanService) GetPricing() (map[string]interface{}, error) {
	plans, err := ps.GetPlans()
	if err != nil {
		return nil, err
	}

	pricing := map[string]interface{}{
		"plans":    plans,
		"currency": "USD",
		"features": map[string]interface{}{
			"storage":   "Cloud storage space",
			"bandwidth": "Monthly data transfer",
			"files":     "Maximum number of files",
			"folders":   "Maximum number of folders",
			"support":   "Customer support level",
		},
	}

	return pricing, nil
}

// User Plan Management
func (ps *PlanService) GetUserPlan(userID primitive.ObjectID) (*models.Plan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get user
	var user models.User
	err := ps.userCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	// Get user's plan
	var plan models.Plan
	err = ps.planCollection.FindOne(ctx, bson.M{"_id": user.PlanID}).Decode(&plan)
	if err != nil {
		return nil, fmt.Errorf("user plan not found: %v", err)
	}

	return &plan, nil
}

func (ps *PlanService) Subscribe(userID, planID primitive.ObjectID, paymentMethodID string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get plan
	plan, err := ps.GetPlan(planID)
	if err != nil {
		return nil, err
	}

	// Get user
	var user models.User
	err = ps.userCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	// Check if already subscribed to this plan
	if user.PlanID == planID {
		return nil, fmt.Errorf("user is already subscribed to this plan")
	}

	// Create subscription record
	subscription := bson.M{
		"_id":              primitive.NewObjectID(),
		"user_id":          userID,
		"plan_id":          planID,
		"previous_plan_id": user.PlanID,
		"status":           "active",
		"payment_method":   paymentMethodID,
		"started_at":       time.Now(),
		"created_at":       time.Now(),
		"updated_at":       time.Now(),
	}

	_, err = ps.subscriptionCollection.InsertOne(ctx, subscription)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %v", err)
	}

	// Update user plan
	_, err = ps.userCollection.UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{
			"plan_id":    planID,
			"updated_at": time.Now(),
		}},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update user plan: %v", err)
	}

	result := map[string]interface{}{
		"subscription_id": subscription["_id"],
		"plan":            plan,
		"status":          "active",
		"started_at":      subscription["started_at"],
	}

	return result, nil
}

func (ps *PlanService) UpgradePlan(userID, newPlanID primitive.ObjectID, paymentMethodID string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get current plan
	currentPlan, err := ps.GetUserPlan(userID)
	if err != nil {
		return nil, err
	}

	// Get new plan
	newPlan, err := ps.GetPlan(newPlanID)
	if err != nil {
		return nil, err
	}

	// Validate it's an upgrade
	if newPlan.Price <= currentPlan.Price {
		return nil, fmt.Errorf("new plan must be more expensive than current plan")
	}

	// Create upgrade record
	upgrade := bson.M{
		"_id":              primitive.NewObjectID(),
		"user_id":          userID,
		"from_plan_id":     currentPlan.ID,
		"to_plan_id":       newPlanID,
		"payment_method":   paymentMethodID,
		"upgrade_type":     "immediate",
		"price_difference": newPlan.Price - currentPlan.Price,
		"status":           "completed",
		"upgraded_at":      time.Now(),
		"created_at":       time.Now(),
	}

	_, err = ps.subscriptionCollection.InsertOne(ctx, upgrade)
	if err != nil {
		return nil, fmt.Errorf("failed to record upgrade: %v", err)
	}

	// Update user plan
	_, err = ps.userCollection.UpdateOne(ctx,
		bson.M{"_id": userID},
		bson.M{"$set": bson.M{
			"plan_id":    newPlanID,
			"updated_at": time.Now(),
		}},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update user plan: %v", err)
	}

	result := map[string]interface{}{
		"from_plan":        currentPlan,
		"to_plan":          newPlan,
		"price_difference": newPlan.Price - currentPlan.Price,
		"status":           "completed",
		"upgraded_at":      time.Now(),
	}

	return result, nil
}

func (ps *PlanService) DowngradePlan(userID, newPlanID primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get current plan
	currentPlan, err := ps.GetUserPlan(userID)
	if err != nil {
		return nil, err
	}

	// Get new plan
	newPlan, err := ps.GetPlan(newPlanID)
	if err != nil {
		return nil, err
	}

	// Validate it's a downgrade
	if newPlan.Price >= currentPlan.Price {
		return nil, fmt.Errorf("new plan must be less expensive than current plan")
	}

	// Schedule downgrade (usually effective at next billing cycle)
	nextBillingDate := time.Now().AddDate(0, 1, 0) // Next month

	downgrade := bson.M{
		"_id":            primitive.NewObjectID(),
		"user_id":        userID,
		"from_plan_id":   currentPlan.ID,
		"to_plan_id":     newPlanID,
		"downgrade_type": "scheduled",
		"status":         "scheduled",
		"effective_date": nextBillingDate,
		"scheduled_at":   time.Now(),
		"created_at":     time.Now(),
	}

	_, err = ps.subscriptionCollection.InsertOne(ctx, downgrade)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule downgrade: %v", err)
	}

	result := map[string]interface{}{
		"from_plan":      currentPlan,
		"to_plan":        newPlan,
		"status":         "scheduled",
		"effective_date": nextBillingDate,
		"scheduled_at":   time.Now(),
	}

	return result, nil
}

func (ps *PlanService) CancelSubscription(userID primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get current plan
	currentPlan, err := ps.GetUserPlan(userID)
	if err != nil {
		return nil, err
	}

	// Get free plan
	var freePlan models.Plan
	err = ps.planCollection.FindOne(ctx, bson.M{
		"is_free":   true,
		"is_active": true,
	}).Decode(&freePlan)
	if err != nil {
		return nil, fmt.Errorf("free plan not found: %v", err)
	}

	// Schedule cancellation (move to free plan at next billing cycle)
	nextBillingDate := time.Now().AddDate(0, 1, 0) // Next month

	cancellation := bson.M{
		"_id":               primitive.NewObjectID(),
		"user_id":           userID,
		"cancelled_plan_id": currentPlan.ID,
		"fallback_plan_id":  freePlan.ID,
		"status":            "scheduled",
		"effective_date":    nextBillingDate,
		"cancelled_at":      time.Now(),
		"created_at":        time.Now(),
	}

	_, err = ps.subscriptionCollection.InsertOne(ctx, cancellation)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule cancellation: %v", err)
	}

	result := map[string]interface{}{
		"current_plan":   currentPlan,
		"fallback_plan":  freePlan,
		"status":         "scheduled",
		"effective_date": nextBillingDate,
		"cancelled_at":   time.Now(),
	}

	return result, nil
}

func (ps *PlanService) RenewSubscription(userID primitive.ObjectID, paymentMethod, billingCycle string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get current plan
	plan, err := ps.GetUserPlan(userID)
	if err != nil {
		return nil, err
	}

	// Create renewal record
	renewal := bson.M{
		"_id":            primitive.NewObjectID(),
		"user_id":        userID,
		"plan_id":        plan.ID,
		"payment_method": paymentMethod,
		"billing_cycle":  billingCycle,
		"amount":         plan.Price,
		"currency":       plan.Currency,
		"status":         "completed",
		"renewed_at":     time.Now(),
		"next_renewal":   ps.calculateNextRenewal(billingCycle),
		"created_at":     time.Now(),
	}

	_, err = ps.subscriptionCollection.InsertOne(ctx, renewal)
	if err != nil {
		return nil, fmt.Errorf("failed to record renewal: %v", err)
	}

	result := map[string]interface{}{
		"plan":          plan,
		"amount":        plan.Price,
		"currency":      plan.Currency,
		"billing_cycle": billingCycle,
		"renewed_at":    time.Now(),
		"next_renewal":  renewal["next_renewal"],
		"status":        "completed",
	}

	return result, nil
}

// Billing and Usage
func (ps *PlanService) GetBillingHistory(userID primitive.ObjectID, page, limit int) ([]map[string]interface{}, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	skip := (page - 1) * limit

	cursor, err := ps.billingCollection.Find(ctx, bson.M{"user_id": userID},
		options.Find().
			SetSkip(int64(skip)).
			SetLimit(int64(limit)).
			SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var history []map[string]interface{}
	if err = cursor.All(ctx, &history); err != nil {
		return nil, 0, err
	}

	total, err := ps.billingCollection.CountDocuments(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, 0, err
	}

	return history, int(total), nil
}

func (ps *PlanService) GetInvoices(userID primitive.ObjectID, page, limit int) ([]map[string]interface{}, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	skip := (page - 1) * limit

	cursor, err := ps.invoiceCollection.Find(ctx, bson.M{"user_id": userID},
		options.Find().
			SetSkip(int64(skip)).
			SetLimit(int64(limit)).
			SetSort(bson.M{"created_at": -1}),
	)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var invoices []map[string]interface{}
	if err = cursor.All(ctx, &invoices); err != nil {
		return nil, 0, err
	}

	total, err := ps.invoiceCollection.CountDocuments(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, 0, err
	}

	return invoices, int(total), nil
}

func (ps *PlanService) DownloadInvoice(userID, invoiceID primitive.ObjectID) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var invoice map[string]interface{}
	err := ps.invoiceCollection.FindOne(ctx, bson.M{
		"_id":     invoiceID,
		"user_id": userID,
	}).Decode(&invoice)
	if err != nil {
		return "", fmt.Errorf("invoice not found: %v", err)
	}

	// Generate download URL
	downloadURL := fmt.Sprintf("/api/invoices/%s/download", invoiceID.Hex())
	return downloadURL, nil
}

// Usage Tracking
func (ps *PlanService) GetUsage(userID primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get user
	var user models.User
	err := ps.userCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	// Get plan
	plan, err := ps.GetUserPlan(userID)
	if err != nil {
		return nil, err
	}

	usage := map[string]interface{}{
		"storage": map[string]interface{}{
			"used":       user.StorageUsed,
			"limit":      plan.StorageLimit,
			"percentage": utils.CalculatePercentage(user.StorageUsed, plan.StorageLimit),
			"formatted": map[string]interface{}{
				"used":  utils.FormatFileSize(user.StorageUsed),
				"limit": utils.FormatFileSize(plan.StorageLimit),
			},
		},
		"bandwidth": map[string]interface{}{
			"used":       user.BandwidthUsed,
			"limit":      plan.BandwidthLimit,
			"percentage": utils.CalculatePercentage(user.BandwidthUsed, plan.BandwidthLimit),
			"formatted": map[string]interface{}{
				"used":  utils.FormatFileSize(user.BandwidthUsed),
				"limit": utils.FormatFileSize(plan.BandwidthLimit),
			},
		},
		"files": map[string]interface{}{
			"used":       user.FilesCount,
			"limit":      plan.FilesLimit,
			"percentage": utils.CalculatePercentage(int64(user.FilesCount), int64(plan.FilesLimit)),
		},
		"folders": map[string]interface{}{
			"used":       user.FoldersCount,
			"limit":      plan.FoldersLimit,
			"percentage": utils.CalculatePercentage(int64(user.FoldersCount), int64(plan.FoldersLimit)),
		},
	}

	return usage, nil
}

func (ps *PlanService) GetUsageHistory(userID primitive.ObjectID, period, usageType string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	days, _ := strconv.Atoi(period)
	if days == 0 {
		days = 30
	}

	startDate := time.Now().AddDate(0, 0, -days)
	filter := bson.M{
		"user_id":    userID,
		"created_at": bson.M{"$gte": startDate},
	}

	if usageType != "" && usageType != "all" {
		filter["type"] = usageType
	}

	cursor, err := ps.usageCollection.Find(ctx, filter,
		options.Find().SetSort(bson.M{"created_at": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var history []map[string]interface{}
	if err = cursor.All(ctx, &history); err != nil {
		return nil, err
	}

	return history, nil
}

func (ps *PlanService) GetLimits(userID primitive.ObjectID) (map[string]interface{}, error) {
	plan, err := ps.GetUserPlan(userID)
	if err != nil {
		return nil, err
	}

	limits := map[string]interface{}{
		"storage": map[string]interface{}{
			"limit":           plan.StorageLimit,
			"limit_formatted": utils.FormatFileSize(plan.StorageLimit),
		},
		"bandwidth": map[string]interface{}{
			"limit":           plan.BandwidthLimit,
			"limit_formatted": utils.FormatFileSize(plan.BandwidthLimit),
		},
		"files": map[string]interface{}{
			"limit": plan.FilesLimit,
		},
		"folders": map[string]interface{}{
			"limit": plan.FoldersLimit,
		},
		"max_file_size": map[string]interface{}{
			"limit":           plan.MaxFileSize,
			"limit_formatted": utils.FormatFileSize(plan.MaxFileSize),
		},
		"allowed_types": plan.AllowedTypes,
		"features":      plan.Features,
		"limitations":   plan.Limitations,
		"plan_info": map[string]interface{}{
			"name":          plan.Name,
			"price":         plan.Price,
			"currency":      plan.Currency,
			"billing_cycle": plan.BillingCycle,
		},
	}

	return limits, nil
}

// Admin Plan Management (for AdminController)
func (ps *PlanService) GetAllPlansForAdmin() ([]models.Plan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := ps.planCollection.Find(ctx, bson.M{},
		options.Find().SetSort(bson.M{"sort_order": 1, "created_at": -1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var plans []models.Plan
	if err = cursor.All(ctx, &plans); err != nil {
		return nil, err
	}

	return plans, nil
}

func (ps *PlanService) GetPlanForAdmin(planID primitive.ObjectID) (*models.Plan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var plan models.Plan
	err := ps.planCollection.FindOne(ctx, bson.M{"_id": planID}).Decode(&plan)
	if err != nil {
		return nil, fmt.Errorf("plan not found: %v", err)
	}

	return &plan, nil
}

func (ps *PlanService) CreatePlan(plan *models.Plan) (*models.Plan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Generate slug if not provided
	if plan.Slug == "" {
		plan.Slug = utils.GenerateSlug(plan.Name)
	}

	// Check if slug already exists
	count, err := ps.planCollection.CountDocuments(ctx, bson.M{"slug": plan.Slug})
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, fmt.Errorf("plan with slug %s already exists", plan.Slug)
	}

	plan.ID = primitive.NewObjectID()
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()

	_, err = ps.planCollection.InsertOne(ctx, plan)
	if err != nil {
		return nil, fmt.Errorf("failed to create plan: %v", err)
	}

	return plan, nil
}

func (ps *PlanService) UpdatePlan(planID primitive.ObjectID, updates map[string]interface{}) (*models.Plan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updates["updated_at"] = time.Now()

	_, err := ps.planCollection.UpdateOne(ctx,
		bson.M{"_id": planID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update plan: %v", err)
	}

	return ps.GetPlanForAdmin(planID)
}

func (ps *PlanService) DeletePlan(planID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if plan is in use
	userCount, err := ps.userCollection.CountDocuments(ctx, bson.M{"plan_id": planID})
	if err != nil {
		return err
	}
	if userCount > 0 {
		return fmt.Errorf("cannot delete plan that is currently in use by %d users", userCount)
	}

	_, err = ps.planCollection.DeleteOne(ctx, bson.M{"_id": planID})
	if err != nil {
		return fmt.Errorf("failed to delete plan: %v", err)
	}

	return nil
}

func (ps *PlanService) SetPlanStatus(planID primitive.ObjectID, isActive bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := ps.planCollection.UpdateOne(ctx,
		bson.M{"_id": planID},
		bson.M{"$set": bson.M{
			"is_active":  isActive,
			"updated_at": time.Now(),
		}},
	)
	return err
}

// Helper functions
func (ps *PlanService) calculateNextRenewal(billingCycle string) time.Time {
	now := time.Now()
	switch billingCycle {
	case "daily":
		return now.AddDate(0, 0, 1)
	case "weekly":
		return now.AddDate(0, 0, 7)
	case "monthly":
		return now.AddDate(0, 1, 0)
	case "quarterly":
		return now.AddDate(0, 3, 0)
	case "yearly":
		return now.AddDate(1, 0, 0)
	default:
		return now.AddDate(0, 1, 0) // Default to monthly
	}
}

// GetAvailablePlans returns available plans with optional inactive plans
func (ps *PlanService) GetAvailablePlans(includeInactive bool) ([]models.Plan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{}
	if !includeInactive {
		filter["is_active"] = true
	}

	cursor, err := ps.planCollection.Find(ctx, filter,
		options.Find().SetSort(bson.M{"sort_order": 1, "price": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var plans []models.Plan
	if err = cursor.All(ctx, &plans); err != nil {
		return nil, err
	}

	return plans, nil
}

// GetPlanComparison returns comparison data for specified plans or all active plans
func (ps *PlanService) GetPlanComparison(planIDs []primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"is_active": true}
	if planIDs != nil && len(planIDs) > 0 {
		filter["_id"] = bson.M{"$in": planIDs}
	}

	cursor, err := ps.planCollection.Find(ctx, filter,
		options.Find().SetSort(bson.M{"sort_order": 1, "price": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var plans []models.Plan
	if err = cursor.All(ctx, &plans); err != nil {
		return nil, err
	}

	// Extract unique features for comparison matrix
	featureSet := make(map[string]bool)
	for _, plan := range plans {
		for _, feature := range plan.Features {
			featureSet[feature] = true
		}
	}

	var features []string
	for feature := range featureSet {
		features = append(features, feature)
	}

	comparison := map[string]interface{}{
		"plans":             plans,
		"features":          features,
		"currency":          "USD",
		"comparison_matrix": ps.buildComparisonMatrix(plans, features),
		"recommendations":   ps.getRecommendations(plans),
	}

	return comparison, nil
}

// GetInvoiceDownloadURL generates a download URL for an invoice
func (ps *PlanService) GetInvoiceDownloadURL(userID, invoiceID primitive.ObjectID) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var invoice map[string]interface{}
	err := ps.invoiceCollection.FindOne(ctx, bson.M{
		"_id":     invoiceID,
		"user_id": userID,
	}).Decode(&invoice)
	if err != nil {
		return "", fmt.Errorf("invoice not found: %v", err)
	}

	// Generate signed download URL with expiration
	secureToken := ps.generateSecureToken()
	downloadURL := fmt.Sprintf("/api/v1/invoices/%s/download?token=%s&expires=%d",
		invoiceID.Hex(),
		secureToken,
		time.Now().Add(1*time.Hour).Unix(),
	)

	return downloadURL, nil
}

// AddPaymentMethod adds a new payment method for a user
func (ps *PlanService) AddPaymentMethod(userID primitive.ObjectID, paymentType, token string, isDefault bool, metadata map[string]string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// If this is set as default, update existing payment methods
	if isDefault {
		_, err := database.GetCollection("payment_methods").UpdateMany(ctx,
			bson.M{"user_id": userID, "is_default": true},
			bson.M{"$set": bson.M{
				"is_default": false,
				"updated_at": time.Now(),
			}},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update existing default payment methods: %v", err)
		}
	}

	// Create new payment method
	paymentMethod := bson.M{
		"_id":        primitive.NewObjectID(),
		"user_id":    userID,
		"type":       paymentType, // card, paypal, bank_account
		"token":      token,       // Encrypted token from payment gateway
		"is_default": isDefault,
		"is_active":  true,
		"metadata":   metadata,
		"created_at": time.Now(),
		"updated_at": time.Now(),
	}

	// Add type-specific fields
	switch paymentType {
	case "card":
		if last4, ok := metadata["last4"]; ok {
			paymentMethod["last4"] = last4
		}
		if brand, ok := metadata["brand"]; ok {
			paymentMethod["brand"] = brand
		}
		if expMonth, ok := metadata["exp_month"]; ok {
			paymentMethod["exp_month"] = expMonth
		}
		if expYear, ok := metadata["exp_year"]; ok {
			paymentMethod["exp_year"] = expYear
		}
	case "paypal":
		if email, ok := metadata["email"]; ok {
			paymentMethod["paypal_email"] = email
		}
	case "bank_account":
		if accountType, ok := metadata["account_type"]; ok {
			paymentMethod["account_type"] = accountType
		}
		if routingNumber, ok := metadata["routing_number"]; ok {
			paymentMethod["routing_number"] = routingNumber
		}
		if last4, ok := metadata["last4"]; ok {
			paymentMethod["account_last4"] = last4
		}
	}

	result, err := database.GetCollection("payment_methods").InsertOne(ctx, paymentMethod)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment method: %v", err)
	}

	paymentMethod["_id"] = result.InsertedID

	// Remove sensitive token from response
	delete(paymentMethod, "token")

	return paymentMethod, nil
}

// GetPaymentMethods returns all payment methods for a user
func (ps *PlanService) GetPaymentMethods(userID primitive.ObjectID) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := database.GetCollection("payment_methods").Find(ctx,
		bson.M{"user_id": userID, "is_active": true},
		options.Find().SetSort(bson.M{"is_default": -1, "created_at": -1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var methods []map[string]interface{}
	if err = cursor.All(ctx, &methods); err != nil {
		return nil, err
	}

	// Remove sensitive data from response
	for _, method := range methods {
		delete(method, "token")
	}

	return methods, nil
}

// UpdatePaymentMethod updates an existing payment method
func (ps *PlanService) UpdatePaymentMethod(userID, methodID primitive.ObjectID, isDefault bool, metadata map[string]string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Verify payment method belongs to user
	var existingMethod map[string]interface{}
	err := database.GetCollection("payment_methods").FindOne(ctx, bson.M{
		"_id":     methodID,
		"user_id": userID,
	}).Decode(&existingMethod)
	if err != nil {
		return fmt.Errorf("payment method not found: %v", err)
	}

	updates := bson.M{
		"is_default": isDefault,
		"updated_at": time.Now(),
	}

	// Update metadata if provided
	if metadata != nil {
		for key, value := range metadata {
			updates["metadata."+key] = value
		}
	}

	// If setting as default, unset other default methods
	if isDefault {
		_, err := database.GetCollection("payment_methods").UpdateMany(ctx,
			bson.M{
				"user_id":    userID,
				"is_default": true,
				"_id":        bson.M{"$ne": methodID},
			},
			bson.M{"$set": bson.M{
				"is_default": false,
				"updated_at": time.Now(),
			}},
		)
		if err != nil {
			return fmt.Errorf("failed to update existing default payment methods: %v", err)
		}
	}

	_, err = database.GetCollection("payment_methods").UpdateOne(ctx,
		bson.M{"_id": methodID, "user_id": userID},
		bson.M{"$set": updates},
	)
	if err != nil {
		return fmt.Errorf("failed to update payment method: %v", err)
	}

	return nil
}

// DeletePaymentMethod removes a payment method
func (ps *PlanService) DeletePaymentMethod(userID, methodID primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if this is the user's only payment method
	count, err := database.GetCollection("payment_methods").CountDocuments(ctx, bson.M{
		"user_id":   userID,
		"is_active": true,
	})
	if err != nil {
		return fmt.Errorf("failed to check payment method count: %v", err)
	}

	if count <= 1 {
		return fmt.Errorf("cannot delete the only payment method")
	}

	// Check if this is the default payment method
	var method map[string]interface{}
	err = database.GetCollection("payment_methods").FindOne(ctx, bson.M{
		"_id":     methodID,
		"user_id": userID,
	}).Decode(&method)
	if err != nil {
		return fmt.Errorf("payment method not found: %v", err)
	}

	isDefault, _ := method["is_default"].(bool)

	// Soft delete the payment method
	_, err = database.GetCollection("payment_methods").UpdateOne(ctx,
		bson.M{"_id": methodID, "user_id": userID},
		bson.M{"$set": bson.M{
			"is_active":  false,
			"deleted_at": time.Now(),
			"updated_at": time.Now(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to delete payment method: %v", err)
	}

	// If this was the default, set another payment method as default
	if isDefault {
		_, err = database.GetCollection("payment_methods").UpdateOne(ctx,
			bson.M{
				"user_id":   userID,
				"is_active": true,
				"_id":       bson.M{"$ne": methodID},
			},
			bson.M{"$set": bson.M{
				"is_default": true,
				"updated_at": time.Now(),
			}},
		)
		if err != nil {
			return fmt.Errorf("failed to set new default payment method: %v", err)
		}
	}

	return nil
}

// GetUserLimits returns user's current plan limits and usage
func (ps *PlanService) GetUserLimits(userID primitive.ObjectID) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get user
	var user models.User
	err := ps.userCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	// Get plan
	plan, err := ps.GetUserPlan(userID)
	if err != nil {
		return nil, err
	}

	limits := map[string]interface{}{
		"storage": map[string]interface{}{
			"used":                user.StorageUsed,
			"limit":               plan.StorageLimit,
			"remaining":           plan.StorageLimit - user.StorageUsed,
			"percentage":          ps.calculatePercentage(user.StorageUsed, plan.StorageLimit),
			"limit_formatted":     utils.FormatFileSize(plan.StorageLimit),
			"used_formatted":      utils.FormatFileSize(user.StorageUsed),
			"remaining_formatted": utils.FormatFileSize(plan.StorageLimit - user.StorageUsed),
		},
		"bandwidth": map[string]interface{}{
			"used":                user.BandwidthUsed,
			"limit":               plan.BandwidthLimit,
			"remaining":           plan.BandwidthLimit - user.BandwidthUsed,
			"percentage":          ps.calculatePercentage(user.BandwidthUsed, plan.BandwidthLimit),
			"limit_formatted":     utils.FormatFileSize(plan.BandwidthLimit),
			"used_formatted":      utils.FormatFileSize(user.BandwidthUsed),
			"remaining_formatted": utils.FormatFileSize(plan.BandwidthLimit - user.BandwidthUsed),
		},
		"files": map[string]interface{}{
			"used":       user.FilesCount,
			"limit":      plan.FilesLimit,
			"remaining":  plan.FilesLimit - user.FilesCount,
			"percentage": ps.calculatePercentage(float64(user.FilesCount), float64(plan.FilesLimit)),
		},
		"folders": map[string]interface{}{
			"used":       user.FoldersCount,
			"limit":      plan.FoldersLimit,
			"remaining":  plan.FoldersLimit - user.FoldersCount,
			"percentage": ps.calculatePercentage(float64(user.FoldersCount), float64(plan.FoldersLimit)),
		},
		"max_file_size": map[string]interface{}{
			"limit":           plan.MaxFileSize,
			"limit_formatted": utils.FormatFileSize(plan.MaxFileSize),
		},
		"allowed_types": plan.AllowedTypes,
		"features":      plan.Features,
		"limitations":   plan.Limitations,
		"plan_info": map[string]interface{}{
			"name":          plan.Name,
			"price":         plan.Price,
			"currency":      plan.Currency,
			"billing_cycle": plan.BillingCycle,
		},
	}

	return limits, nil
}

// HandleStripeWebhook processes Stripe webhook events
func (ps *PlanService) HandleStripeWebhook(payload []byte, signature string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Parse the webhook event
	event, err := ps.parseStripeEvent(payload, signature)
	if err != nil {
		return fmt.Errorf("failed to parse Stripe event: %v", err)
	}

	// Process the event based on type
	switch event["type"].(string) {
	case "invoice.payment_succeeded":
		return ps.handleInvoicePaymentSucceeded(ctx, event)
	case "invoice.payment_failed":
		return ps.handleInvoicePaymentFailed(ctx, event)
	case "customer.subscription.created":
		return ps.handleSubscriptionCreated(ctx, event)
	case "customer.subscription.updated":
		return ps.handleSubscriptionUpdated(ctx, event)
	case "customer.subscription.deleted":
		return ps.handleSubscriptionDeleted(ctx, event)
	case "payment_method.attached":
		return ps.handlePaymentMethodAttached(ctx, event)
	case "payment_method.detached":
		return ps.handlePaymentMethodDetached(ctx, event)
	default:
		// Log unhandled event type
		ps.logWebhookEvent(event, "unhandled")
	}

	return nil
}

// Helper functions for plan comparison
func (ps *PlanService) buildComparisonMatrix(plans []models.Plan, features []string) []map[string]interface{} {
	matrix := make([]map[string]interface{}, len(features))

	for i, feature := range features {
		row := map[string]interface{}{
			"feature": feature,
			"plans":   make(map[string]bool),
		}

		for _, plan := range plans {
			hasFeature := false
			for _, planFeature := range plan.Features {
				if planFeature == feature {
					hasFeature = true
					break
				}
			}
			row["plans"].(map[string]bool)[plan.ID.Hex()] = hasFeature
		}

		matrix[i] = row
	}

	return matrix
}

func (ps *PlanService) getRecommendations(plans []models.Plan) map[string]interface{} {
	if len(plans) == 0 {
		return nil
	}

	var mostPopular, bestValue *models.Plan

	for i := range plans {
		if plans[i].PopularBadge && mostPopular == nil {
			mostPopular = &plans[i]
		}
		if bestValue == nil || (plans[i].Price < bestValue.Price && len(plans[i].Features) >= len(bestValue.Features)) {
			bestValue = &plans[i]
		}
	}

	recommendations := map[string]interface{}{}

	if mostPopular != nil {
		recommendations["most_popular"] = map[string]interface{}{
			"plan_id": mostPopular.ID,
			"name":    mostPopular.Name,
			"reason":  "Most chosen by our users",
		}
	}

	if bestValue != nil {
		recommendations["best_value"] = map[string]interface{}{
			"plan_id": bestValue.ID,
			"name":    bestValue.Name,
			"reason":  "Best features for the price",
		}
	}

	return recommendations
}

// Stripe webhook helper functions
func (ps *PlanService) parseStripeEvent(payload []byte, signature string) (map[string]interface{}, error) {
	// In real implementation, verify signature with Stripe webhook secret
	// For now, just parse the JSON payload
	var event map[string]interface{}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}
	return event, nil
}

func (ps *PlanService) handleInvoicePaymentSucceeded(ctx context.Context, event map[string]interface{}) error {
	// Extract invoice data and update subscription status
	data := event["data"].(map[string]interface{})
	object := data["object"].(map[string]interface{})

	customerID := object["customer"].(string)
	subscriptionID := object["subscription"].(string)
	amountPaid := object["amount_paid"].(float64) / 100 // Convert from cents

	// Update subscription status
	_, err := ps.subscriptionCollection.UpdateOne(ctx,
		bson.M{"stripe_subscription_id": subscriptionID},
		bson.M{"$set": bson.M{
			"status":          "active",
			"last_payment_at": time.Now(),
			"updated_at":      time.Now(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update subscription: %v", err)
	}

	// Create billing record
	billing := bson.M{
		"_id":                primitive.NewObjectID(),
		"stripe_customer_id": customerID,
		"subscription_id":    subscriptionID,
		"amount":             amountPaid,
		"currency":           object["currency"].(string),
		"status":             "completed",
		"payment_method":     "stripe",
		"created_at":         time.Now(),
	}

	_, err = ps.billingCollection.InsertOne(ctx, billing)
	return err
}

func (ps *PlanService) handleInvoicePaymentFailed(ctx context.Context, event map[string]interface{}) error {
	// Handle failed payment - update subscription status, send notifications
	data := event["data"].(map[string]interface{})
	object := data["object"].(map[string]interface{})

	subscriptionID := object["subscription"].(string)

	_, err := ps.subscriptionCollection.UpdateOne(ctx,
		bson.M{"stripe_subscription_id": subscriptionID},
		bson.M{"$set": bson.M{
			"status":            "payment_failed",
			"payment_failed_at": time.Now(),
			"updated_at":        time.Now(),
		}},
	)

	return err
}

func (ps *PlanService) handleSubscriptionCreated(ctx context.Context, event map[string]interface{}) error {
	// Handle new subscription creation
	data := event["data"].(map[string]interface{})
	object := data["object"].(map[string]interface{})

	// Create subscription record in database
	subscription := bson.M{
		"_id":                    primitive.NewObjectID(),
		"stripe_subscription_id": object["id"].(string),
		"stripe_customer_id":     object["customer"].(string),
		"status":                 object["status"].(string),
		"current_period_start":   time.Unix(int64(object["current_period_start"].(float64)), 0),
		"current_period_end":     time.Unix(int64(object["current_period_end"].(float64)), 0),
		"created_at":             time.Now(),
		"updated_at":             time.Now(),
	}

	_, err := ps.subscriptionCollection.InsertOne(ctx, subscription)
	return err
}

func (ps *PlanService) handleSubscriptionUpdated(ctx context.Context, event map[string]interface{}) error {
	// Handle subscription updates (plan changes, etc.)
	data := event["data"].(map[string]interface{})
	object := data["object"].(map[string]interface{})

	subscriptionID := object["id"].(string)

	updates := bson.M{
		"status":               object["status"].(string),
		"current_period_start": time.Unix(int64(object["current_period_start"].(float64)), 0),
		"current_period_end":   time.Unix(int64(object["current_period_end"].(float64)), 0),
		"updated_at":           time.Now(),
	}

	_, err := ps.subscriptionCollection.UpdateOne(ctx,
		bson.M{"stripe_subscription_id": subscriptionID},
		bson.M{"$set": updates},
	)

	return err
}

func (ps *PlanService) handleSubscriptionDeleted(ctx context.Context, event map[string]interface{}) error {
	// Handle subscription cancellation
	data := event["data"].(map[string]interface{})
	object := data["object"].(map[string]interface{})

	subscriptionID := object["id"].(string)

	_, err := ps.subscriptionCollection.UpdateOne(ctx,
		bson.M{"stripe_subscription_id": subscriptionID},
		bson.M{"$set": bson.M{
			"status":       "cancelled",
			"cancelled_at": time.Now(),
			"updated_at":   time.Now(),
		}},
	)

	return err
}

func (ps *PlanService) handlePaymentMethodAttached(ctx context.Context, event map[string]interface{}) error {
	// Handle payment method attachment
	data := event["data"].(map[string]interface{})
	object := data["object"].(map[string]interface{})

	// Update payment method record if it exists
	paymentMethodID := object["id"].(string)
	customerID := object["customer"].(string)

	_, err := database.GetCollection("payment_methods").UpdateOne(ctx,
		bson.M{"stripe_payment_method_id": paymentMethodID},
		bson.M{"$set": bson.M{
			"stripe_customer_id": customerID,
			"is_active":          true,
			"updated_at":         time.Now(),
		}},
	)

	return err
}

func (ps *PlanService) handlePaymentMethodDetached(ctx context.Context, event map[string]interface{}) error {
	// Handle payment method detachment
	data := event["data"].(map[string]interface{})
	object := data["object"].(map[string]interface{})

	paymentMethodID := object["id"].(string)

	_, err := database.GetCollection("payment_methods").UpdateOne(ctx,
		bson.M{"stripe_payment_method_id": paymentMethodID},
		bson.M{"$set": bson.M{
			"is_active":  false,
			"deleted_at": time.Now(),
			"updated_at": time.Now(),
		}},
	)

	return err
}

func (ps *PlanService) logWebhookEvent(event map[string]interface{}, status string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		log := bson.M{
			"_id":        primitive.NewObjectID(),
			"event_type": event["type"].(string),
			"event_id":   event["id"].(string),
			"status":     status,
			"data":       event,
			"created_at": time.Now(),
		}

		database.GetCollection("webhook_logs").InsertOne(ctx, log)
	}()
}

// Helper utility functions
func (ps *PlanService) generateSecureToken() string {
	// Generate a secure random token for file downloads
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based token
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func (ps *PlanService) calculatePercentage(used, limit interface{}) float64 {
	var usedFloat, limitFloat float64

	switch v := used.(type) {
	case int64:
		usedFloat = float64(v)
	case int:
		usedFloat = float64(v)
	case float64:
		usedFloat = v
	default:
		return 0
	}

	switch v := limit.(type) {
	case int64:
		limitFloat = float64(v)
	case int:
		limitFloat = float64(v)
	case float64:
		limitFloat = v
	default:
		return 0
	}

	if limitFloat == 0 {
		return 0
	}

	percentage := (usedFloat / limitFloat) * 100
	if percentage > 100 {
		return 100
	}

	return percentage
}
