// Package api provides the API definition for enterprise service.
package api

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/store"
	v1pb "github.com/bytebase/bytebase/proto/generated-go/v1"
)

// validPlans is a string array of valid plan types.
var validPlans = []v1pb.PlanType{
	v1pb.PlanType_TEAM,
	v1pb.PlanType_ENTERPRISE,
}

// License is the API message for enterprise license.
type License struct {
	Subject       string
	InstanceCount int
	Seat          int
	ExpiresTS     int64
	IssuedTS      int64
	Plan          v1pb.PlanType
	Trialing      bool
	OrgName       string
}

// Valid will check if license expired or has correct plan type.
func (l *License) Valid() error {
	if expireTime := time.Unix(l.ExpiresTS, 0); expireTime.Before(time.Now()) {
		return errors.Errorf("license has expired at %v", expireTime)
	}

	return l.validPlanType()
}

func (l *License) validPlanType() error {
	for _, plan := range validPlans {
		if plan == l.Plan {
			return nil
		}
	}

	return errors.Errorf("plan %q is not valid, expect %s or %s",
		l.Plan.String(),
		v1pb.PlanType_TEAM.String(),
		v1pb.PlanType_ENTERPRISE.String(),
	)
}

// OrgID extract the organization id from license subject.
func (l *License) OrgID() string {
	return strings.Split(l.Subject, ".")[0]
}

// LicenseService is the service for enterprise license.
type LicenseService interface {
	// StoreLicense will store license into file.
	StoreLicense(ctx context.Context, patch *SubscriptionPatch) error
	// LoadSubscription will load subscription.
	LoadSubscription(ctx context.Context) *Subscription
	// IsFeatureEnabled returns whether a feature is enabled.
	IsFeatureEnabled(feature v1pb.PlanFeature) error
	// IsFeatureEnabledForInstance returns whether a feature is enabled for the instance.
	IsFeatureEnabledForInstance(feature v1pb.PlanFeature, instance *store.InstanceMessage) error
	// GetEffectivePlan gets the effective plan.
	GetEffectivePlan() v1pb.PlanType
	// GetPlanLimitValue gets the limit value for the plan.
	GetPlanLimitValue(ctx context.Context, name PlanLimit) int
	// GetInstanceLicenseCount returns the instance count limit for current subscription.
	GetInstanceLicenseCount(ctx context.Context) int
	// RefreshCache will invalidate and refresh the subscription cache.
	RefreshCache(ctx context.Context)
}
