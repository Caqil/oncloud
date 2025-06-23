package services

import (
	"context"
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

func (ps *PlanService) ComparePlans() ([]models.Plan, error) {
	plans, err := ps.GetPlans()
	if err != nil {
		return nil, err
	}

	// Add comparison data if needed
	for i := range plans {
		plans[i].IsPopular = plans[i].PopularBadge
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
