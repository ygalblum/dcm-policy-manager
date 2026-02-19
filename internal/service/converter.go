package service

import (
	"fmt"

	"github.com/dcm-project/policy-manager/api/v1alpha1"
	"github.com/dcm-project/policy-manager/internal/store/model"
)

const (
	DefaultPriority = 500
)

// APIToDBModel converts an API Policy model to a database Policy model.
// RegoCode is stripped as it's not stored in the database.
// PackageName must be set separately by the caller after extracting it from the Rego code.
// All Policy fields are optional in the schema; required fields for create are enforced by the service.
func APIToDBModel(api v1alpha1.Policy, id string) model.Policy {
	db := model.Policy{ID: id}

	if api.DisplayName != nil {
		db.DisplayName = *api.DisplayName
	}
	if api.PolicyType != nil {
		db.PolicyType = string(*api.PolicyType)
	}

	if api.Description != nil {
		db.Description = *api.Description
	}
	if api.Enabled != nil {
		db.Enabled = *api.Enabled
	} else {
		db.Enabled = true
	}
	if api.Priority != nil {
		db.Priority = *api.Priority
	} else {
		db.Priority = DefaultPriority
	}
	if api.LabelSelector != nil {
		db.LabelSelector = *api.LabelSelector
	}

	return db
}

// DBToAPIModel converts a database Policy model to an API Policy model.
// Path is set, RegoCode is empty (not stored in DB), timestamps included.
func DBToAPIModel(db *model.Policy) v1alpha1.Policy {
	path := fmt.Sprintf("policies/%s", db.ID)
	regoCode := ""
	displayName := db.DisplayName
	policyType := v1alpha1.PolicyPolicyType(db.PolicyType)
	api := v1alpha1.Policy{
		Id:          &db.ID,
		Path:        &path,
		DisplayName: &displayName,
		PolicyType:  &policyType,
		Priority:    &db.Priority,
		Enabled:     &db.Enabled,
		CreateTime:  &db.CreateTime,
		UpdateTime:  &db.UpdateTime,
		RegoCode:    &regoCode,
	}
	if db.Description != "" {
		api.Description = &db.Description
	}
	if len(db.LabelSelector) > 0 {
		api.LabelSelector = &db.LabelSelector
	}
	return api
}
