package store

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"

	"github.com/dcm-project/policy-manager/internal/store/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPolicyNotFound = errors.New("policy not found")
	ErrPolicyIDTaken  = errors.New("policy ID already taken")
)

// PolicyFilter contains optional fields for filtering policy queries.
// nil fields are ignored (not filtered).
type PolicyFilter struct {
	PolicyType *string
	Enabled    *bool
}

// PolicyListOptions contains options for listing policies.
type PolicyListOptions struct {
	Filter    *PolicyFilter
	OrderBy   string
	PageToken *string
	PageSize  int
}

// PolicyListResult contains the result of a List operation.
type PolicyListResult struct {
	Policies      model.PolicyList
	NextPageToken string
}

type Policy interface {
	List(ctx context.Context, opts *PolicyListOptions) (*PolicyListResult, error)
	Create(ctx context.Context, policy model.Policy) (*model.Policy, error)
	Delete(ctx context.Context, id string) error
	Update(ctx context.Context, policy model.Policy) (*model.Policy, error)
	Get(ctx context.Context, id string) (*model.Policy, error)
}

type PolicyStore struct {
	db *gorm.DB
}

var _ Policy = (*PolicyStore)(nil)

func NewPolicy(db *gorm.DB) Policy {
	return &PolicyStore{db: db}
}

func (s *PolicyStore) List(ctx context.Context, opts *PolicyListOptions) (*PolicyListResult, error) {
	var policies model.PolicyList
	query := s.db.WithContext(ctx)

	// Default page size
	pageSize := 50
	if opts != nil && opts.PageSize > 0 {
		pageSize = opts.PageSize
	}

	// Decode page token to get offset
	offset := 0
	if opts != nil && opts.PageToken != nil && *opts.PageToken != "" {
		decoded, err := base64.StdEncoding.DecodeString(*opts.PageToken)
		if err == nil {
			if parsedOffset, err := strconv.Atoi(string(decoded)); err == nil {
				offset = parsedOffset
			}
		}
	}

	if opts != nil {
		if opts.Filter != nil {
			if opts.Filter.PolicyType != nil {
				query = query.Where("policy_type = ?", *opts.Filter.PolicyType)
			}
			if opts.Filter.Enabled != nil {
				query = query.Where("enabled = ?", *opts.Filter.Enabled)
			}
		}

		// Apply ordering
		if opts.OrderBy != "" {
			query = query.Order(opts.OrderBy)
		} else {
			// Default order by priority ascending
			query = query.Order("priority ASC")
		}
	} else {
		// Default order when no options provided
		query = query.Order("priority ASC")
	}

	// Query with limit+1 to detect if there are more results
	query = query.Limit(pageSize + 1).Offset(offset)

	if err := query.Find(&policies).Error; err != nil {
		return nil, err
	}

	// Generate next page token if there are more results
	result := &PolicyListResult{
		Policies: policies,
	}

	if len(policies) > pageSize {
		// Trim to requested page size
		result.Policies = policies[:pageSize]
		// Encode next offset as page token
		nextOffset := offset + pageSize
		result.NextPageToken = base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(nextOffset)))
	}

	return result, nil
}

func (s *PolicyStore) Create(ctx context.Context, policy model.Policy) (*model.Policy, error) {
	if err := s.db.WithContext(ctx).Clauses(clause.Returning{}).Select("*").Create(&policy).Error; err != nil {
		return nil, err
	}
	return &policy, nil
}

func (s *PolicyStore) Delete(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.Policy{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPolicyNotFound
	}
	return nil
}

func (s *PolicyStore) Update(ctx context.Context, policy model.Policy) (*model.Policy, error) {
	result := s.db.WithContext(ctx).Model(&policy).Clauses(clause.Returning{}).Updates(&policy)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrPolicyNotFound
	}
	return &policy, nil
}

func (s *PolicyStore) Get(ctx context.Context, id string) (*model.Policy, error) {
	var policy model.Policy
	if err := s.db.WithContext(ctx).First(&policy, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPolicyNotFound
		}
		return nil, err
	}
	return &policy, nil
}
