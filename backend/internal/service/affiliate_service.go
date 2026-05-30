package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var (
	ErrAffiliateProfileNotFound = infraerrors.NotFound("AFFILIATE_PROFILE_NOT_FOUND", "affiliate profile not found")
	ErrAffiliateCodeInvalid     = infraerrors.BadRequest("AFFILIATE_CODE_INVALID", "invalid affiliate code")
	ErrAffiliateCodeTaken       = infraerrors.Conflict("AFFILIATE_CODE_TAKEN", "affiliate code already in use")
	ErrAffiliateAlreadyBound    = infraerrors.Conflict("AFFILIATE_ALREADY_BOUND", "affiliate inviter already bound")
	ErrAffiliateQuotaEmpty      = infraerrors.BadRequest("AFFILIATE_QUOTA_EMPTY", "no affiliate quota available to transfer")
	ErrAffiliateLevelNotFound   = infraerrors.NotFound("AFFILIATE_LEVEL_NOT_FOUND", "affiliate level not found")
	ErrAffiliateLevelCodeTaken  = infraerrors.Conflict("AFFILIATE_LEVEL_CODE_TAKEN", "affiliate level code already in use")
)

const (
	affiliateInviteesLimit = 100
	// AffiliateCodeMinLength / AffiliateCodeMaxLength bound both system-generated
	// 12-char codes and admin-customized codes (e.g. "VIP2026").
	AffiliateCodeMinLength      = 4
	AffiliateCodeMaxLength      = 32
	AffiliateLevelCodeMinLength = 2
	AffiliateLevelCodeMaxLength = 32
	AffiliateLevelNameMaxLength = 64
)

// affiliateCodeValidChar accepts uppercase letters, digits, underscore and dash.
// All input passes through strings.ToUpper before validation, so lowercase from
// users is normalized — admins may supply mixed case in their UI.
var affiliateCodeValidChar = func() [256]bool {
	var tbl [256]bool
	for c := byte('A'); c <= 'Z'; c++ {
		tbl[c] = true
	}
	for c := byte('0'); c <= '9'; c++ {
		tbl[c] = true
	}
	tbl['_'] = true
	tbl['-'] = true
	return tbl
}()

// isValidAffiliateCodeFormat validates code format for both binding (user input)
// and admin updates. Caller is expected to upper-case the input first.
func isValidAffiliateCodeFormat(code string) bool {
	if len(code) < AffiliateCodeMinLength || len(code) > AffiliateCodeMaxLength {
		return false
	}
	for i := 0; i < len(code); i++ {
		if !affiliateCodeValidChar[code[i]] {
			return false
		}
	}
	return true
}

type AffiliateSummary struct {
	UserID               int64     `json:"user_id"`
	AffCode              string    `json:"aff_code"`
	AffCodeCustom        bool      `json:"aff_code_custom"`
	AffRebateRatePercent *float64  `json:"aff_rebate_rate_percent,omitempty"`
	AffLevelID           *int64    `json:"aff_level_id,omitempty"`
	AffLevelManual       bool      `json:"aff_level_manual"`
	InviterID            *int64    `json:"inviter_id,omitempty"`
	AffCount             int       `json:"aff_count"`
	AffQuota             float64   `json:"aff_quota"`
	AffFrozenQuota       float64   `json:"aff_frozen_quota"`
	AffHistoryQuota      float64   `json:"aff_history_quota"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type AffiliateInvitee struct {
	UserID      int64      `json:"user_id"`
	Email       string     `json:"email"`
	Username    string     `json:"username"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
	TotalRebate float64    `json:"total_rebate"`
}

type AffiliateDetail struct {
	UserID          int64   `json:"user_id"`
	AffCode         string  `json:"aff_code"`
	InviterID       *int64  `json:"inviter_id,omitempty"`
	AffCount        int     `json:"aff_count"`
	AffQuota        float64 `json:"aff_quota"`
	AffFrozenQuota  float64 `json:"aff_frozen_quota"`
	AffHistoryQuota float64 `json:"aff_history_quota"`
	// EffectiveRebateRatePercent 是当前用户作为邀请人时实际生效的返利比例：
	// 优先用户自己的专属比例（aff_rebate_rate_percent），否则回退到全局比例。
	// 用于在用户的 /affiliate 页面直观展示「分享后能拿到多少」。
	EffectiveRebateRatePercent float64              `json:"effective_rebate_rate_percent"`
	EffectiveAgentLevel        *AffiliateAgentLevel `json:"effective_agent_level,omitempty"`
	RebateRateSource           string               `json:"rebate_rate_source,omitempty"`
	Invitees                   []AffiliateInvitee   `json:"invitees"`
}

type AffiliateAgentLevel struct {
	ID                int64     `json:"id"`
	Code              string    `json:"code"`
	Name              string    `json:"name"`
	RebateRatePercent float64   `json:"rebate_rate_percent"`
	MinInvitedCount   int       `json:"min_invited_count"`
	MinHistoryQuota   float64   `json:"min_history_quota"`
	SortOrder         int       `json:"sort_order"`
	Enabled           bool      `json:"enabled"`
	IsDefault         bool      `json:"is_default"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type AffiliateAgentLevelInput struct {
	Code              string
	Name              string
	RebateRatePercent float64
	MinInvitedCount   int
	MinHistoryQuota   float64
	SortOrder         int
	Enabled           bool
	IsDefault         bool
}

type AffiliateRebateSnapshot struct {
	RebateRatePercent float64
	AgentLevelID      *int64
	AgentLevelCode    string
	AgentLevelName    string
}

type affiliateRebatePolicy struct {
	RatePercent float64
	AgentLevel  *AffiliateAgentLevel
	Source      string
}

type AffiliateRepository interface {
	EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error)
	GetAffiliateByCode(ctx context.Context, code string) (*AffiliateSummary, error)
	BindInviter(ctx context.Context, userID, inviterID int64) (bool, error)
	AccrueQuota(ctx context.Context, inviterID, inviteeUserID int64, amount float64, freezeHours int, sourceOrderID *int64, snapshot *AffiliateRebateSnapshot) (bool, error)
	GetAccruedRebateFromInvitee(ctx context.Context, inviterID, inviteeUserID int64) (float64, error)
	ThawFrozenQuota(ctx context.Context, userID int64) (float64, error)
	TransferQuotaToBalance(ctx context.Context, userID int64) (float64, float64, error)
	ListInvitees(ctx context.Context, inviterID int64, limit int) ([]AffiliateInvitee, error)
	ListAffiliateAgentLevels(ctx context.Context, includeDisabled bool) ([]AffiliateAgentLevel, error)
	CreateAffiliateAgentLevel(ctx context.Context, input AffiliateAgentLevelInput) (*AffiliateAgentLevel, error)
	UpdateAffiliateAgentLevel(ctx context.Context, id int64, input AffiliateAgentLevelInput) (*AffiliateAgentLevel, error)
	DeleteAffiliateAgentLevel(ctx context.Context, id int64) error
	SetUserAffiliateLevel(ctx context.Context, userID int64, levelID *int64) error
	BatchSetUserAffiliateLevel(ctx context.Context, userIDs []int64, levelID *int64) error

	// 管理端：用户级专属配置
	UpdateUserAffCode(ctx context.Context, userID int64, newCode string) error
	ResetUserAffCode(ctx context.Context, userID int64) (string, error)
	SetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error
	BatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error
	ListUsersWithCustomSettings(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error)
	ListAffiliateInviteRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error)
	ListAffiliateRebateRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error)
	ListAffiliateTransferRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error)
	GetAffiliateUserOverview(ctx context.Context, userID int64) (*AffiliateUserOverview, error)
}

// AffiliateAdminFilter 列表筛选条件
type AffiliateAdminFilter struct {
	Search   string
	Page     int
	PageSize int
}

// AffiliateAdminEntry 专属用户列表条目
type AffiliateAdminEntry struct {
	UserID                     int64                `json:"user_id"`
	Email                      string               `json:"email"`
	Username                   string               `json:"username"`
	AffCode                    string               `json:"aff_code"`
	AffCodeCustom              bool                 `json:"aff_code_custom"`
	AffRebateRatePercent       *float64             `json:"aff_rebate_rate_percent,omitempty"`
	AffLevelID                 *int64               `json:"aff_level_id,omitempty"`
	AffLevelManual             bool                 `json:"aff_level_manual"`
	AgentLevel                 *AffiliateAgentLevel `json:"agent_level,omitempty"`
	EffectiveAgentLevel        *AffiliateAgentLevel `json:"effective_agent_level,omitempty"`
	EffectiveRebateRatePercent float64              `json:"effective_rebate_rate_percent"`
	AffCount                   int                  `json:"aff_count"`
	AffHistoryQuota            float64              `json:"aff_history_quota"`
}

type AffiliateRecordFilter struct {
	Search   string
	Page     int
	PageSize int
	StartAt  *time.Time
	EndAt    *time.Time
	SortBy   string
	SortDesc bool
}

type AffiliateInviteRecord struct {
	InviterID       int64     `json:"inviter_id"`
	InviterEmail    string    `json:"inviter_email"`
	InviterUsername string    `json:"inviter_username"`
	InviteeID       int64     `json:"invitee_id"`
	InviteeEmail    string    `json:"invitee_email"`
	InviteeUsername string    `json:"invitee_username"`
	AffCode         string    `json:"aff_code"`
	TotalRebate     float64   `json:"total_rebate"`
	CreatedAt       time.Time `json:"created_at"`
}

type AffiliateRebateRecord struct {
	OrderID           int64     `json:"order_id"`
	OutTradeNo        string    `json:"out_trade_no"`
	InviterID         int64     `json:"inviter_id"`
	InviterEmail      string    `json:"inviter_email"`
	InviterUsername   string    `json:"inviter_username"`
	InviteeID         int64     `json:"invitee_id"`
	InviteeEmail      string    `json:"invitee_email"`
	InviteeUsername   string    `json:"invitee_username"`
	OrderAmount       float64   `json:"order_amount"`
	PayAmount         float64   `json:"pay_amount"`
	RebateAmount      float64   `json:"rebate_amount"`
	RebateRatePercent *float64  `json:"rebate_rate_percent,omitempty"`
	AgentLevelName    string    `json:"agent_level_name,omitempty"`
	PaymentType       string    `json:"payment_type"`
	OrderStatus       string    `json:"order_status"`
	CreatedAt         time.Time `json:"created_at"`
}

type AffiliateTransferRecord struct {
	LedgerID            int64     `json:"ledger_id"`
	UserID              int64     `json:"user_id"`
	UserEmail           string    `json:"user_email"`
	Username            string    `json:"username"`
	Amount              float64   `json:"amount"`
	BalanceAfter        *float64  `json:"balance_after,omitempty"`
	AvailableQuotaAfter *float64  `json:"available_quota_after,omitempty"`
	FrozenQuotaAfter    *float64  `json:"frozen_quota_after,omitempty"`
	HistoryQuotaAfter   *float64  `json:"history_quota_after,omitempty"`
	SnapshotAvailable   bool      `json:"snapshot_available"`
	CurrentBalance      float64   `json:"-"`
	RemainingQuota      float64   `json:"-"`
	FrozenQuota         float64   `json:"-"`
	HistoryQuota        float64   `json:"-"`
	CreatedAt           time.Time `json:"created_at"`
}

type AffiliateUserOverview struct {
	UserID               int64                `json:"user_id"`
	Email                string               `json:"email"`
	Username             string               `json:"username"`
	AffCode              string               `json:"aff_code"`
	RebateRatePercent    float64              `json:"rebate_rate_percent"`
	RebateRateCustom     bool                 `json:"-"`
	AffRebateRatePercent *float64             `json:"-"`
	AffLevelID           *int64               `json:"aff_level_id,omitempty"`
	AffLevelManual       bool                 `json:"aff_level_manual"`
	AgentLevel           *AffiliateAgentLevel `json:"agent_level,omitempty"`
	RebateRateSource     string               `json:"rebate_rate_source,omitempty"`
	InvitedCount         int                  `json:"invited_count"`
	RebatedInviteeCount  int                  `json:"rebated_invitee_count"`
	AvailableQuota       float64              `json:"available_quota"`
	HistoryQuota         float64              `json:"history_quota"`
}

type AffiliateService struct {
	repo                 AffiliateRepository
	settingService       *SettingService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
}

func NewAffiliateService(repo AffiliateRepository, settingService *SettingService, authCacheInvalidator APIKeyAuthCacheInvalidator, billingCacheService *BillingCacheService) *AffiliateService {
	return &AffiliateService{
		repo:                 repo,
		settingService:       settingService,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
	}
}

// IsEnabled reports whether the affiliate (邀请返利) feature is turned on.
func (s *AffiliateService) IsEnabled(ctx context.Context) bool {
	if s == nil || s.settingService == nil {
		return AffiliateEnabledDefault
	}
	return s.settingService.IsAffiliateEnabled(ctx)
}

func (s *AffiliateService) EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.EnsureUserAffiliate(ctx, userID)
}

func (s *AffiliateService) GetAffiliateDetail(ctx context.Context, userID int64) (*AffiliateDetail, error) {
	// Lazy thaw: move any matured frozen quota to available before reading.
	if s != nil && s.repo != nil {
		// best-effort: thaw failure is non-fatal
		_, _ = s.repo.ThawFrozenQuota(ctx, userID)
	}

	summary, err := s.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return nil, err
	}
	invitees, err := s.listInvitees(ctx, userID)
	if err != nil {
		return nil, err
	}
	policy := s.resolveRebatePolicy(ctx, summary)
	return &AffiliateDetail{
		UserID:                     summary.UserID,
		AffCode:                    summary.AffCode,
		InviterID:                  summary.InviterID,
		AffCount:                   summary.AffCount,
		AffQuota:                   summary.AffQuota,
		AffFrozenQuota:             summary.AffFrozenQuota,
		AffHistoryQuota:            summary.AffHistoryQuota,
		EffectiveRebateRatePercent: policy.RatePercent,
		EffectiveAgentLevel:        policy.AgentLevel,
		RebateRateSource:           policy.Source,
		Invitees:                   invitees,
	}, nil
}

func (s *AffiliateService) BindInviterByCode(ctx context.Context, userID int64, rawCode string) error {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" {
		return nil
	}
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	// 总开关关闭时，注册阶段静默忽略 aff 参数（不报错，避免阻断注册流程）
	if !s.IsEnabled(ctx) {
		return nil
	}
	if !isValidAffiliateCodeFormat(code) {
		return ErrAffiliateCodeInvalid
	}

	selfSummary, err := s.repo.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return err
	}
	if selfSummary.InviterID != nil {
		return nil
	}

	inviterSummary, err := s.repo.GetAffiliateByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrAffiliateProfileNotFound) {
			return ErrAffiliateCodeInvalid
		}
		return err
	}
	if inviterSummary == nil || inviterSummary.UserID <= 0 || inviterSummary.UserID == userID {
		return ErrAffiliateCodeInvalid
	}

	bound, err := s.repo.BindInviter(ctx, userID, inviterSummary.UserID)
	if err != nil {
		return err
	}
	if !bound {
		return ErrAffiliateAlreadyBound
	}
	return nil
}

func (s *AffiliateService) AccrueInviteRebate(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64) (float64, error) {
	return s.AccrueInviteRebateForOrder(ctx, inviteeUserID, baseRechargeAmount, nil)
}

func (s *AffiliateService) AccrueInviteRebateForOrder(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64, sourceOrderID *int64) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if inviteeUserID <= 0 || baseRechargeAmount <= 0 || math.IsNaN(baseRechargeAmount) || math.IsInf(baseRechargeAmount, 0) {
		return 0, nil
	}
	// 总开关关闭时，新充值不再产生返利
	if !s.IsEnabled(ctx) {
		return 0, nil
	}

	inviteeSummary, err := s.repo.EnsureUserAffiliate(ctx, inviteeUserID)
	if err != nil {
		return 0, err
	}
	if inviteeSummary.InviterID == nil || *inviteeSummary.InviterID <= 0 {
		return 0, nil
	}

	// 加载邀请人 profile，优先使用专属比例（覆盖全局）
	inviterSummary, err := s.repo.EnsureUserAffiliate(ctx, *inviteeSummary.InviterID)
	if err != nil {
		return 0, err
	}
	// 有效期检查：超过返利有效期后不再产生返利
	if s.settingService != nil {
		if durationDays := s.settingService.GetAffiliateRebateDurationDays(ctx); durationDays > 0 {
			if time.Now().After(inviteeSummary.CreatedAt.AddDate(0, 0, durationDays)) {
				return 0, nil
			}
		}
	}

	policy := s.resolveRebatePolicy(ctx, inviterSummary)
	rebate := roundTo(baseRechargeAmount*(policy.RatePercent/100), 8)
	if rebate <= 0 {
		return 0, nil
	}

	// 单人上限检查：精确截断到剩余额度
	if s.settingService != nil {
		if perInviteeCap := s.settingService.GetAffiliateRebatePerInviteeCap(ctx); perInviteeCap > 0 {
			existing, err := s.repo.GetAccruedRebateFromInvitee(ctx, *inviteeSummary.InviterID, inviteeUserID)
			if err != nil {
				return 0, err
			}
			if existing >= perInviteeCap {
				return 0, nil
			}
			if remaining := perInviteeCap - existing; rebate > remaining {
				rebate = roundTo(remaining, 8)
			}
		}
	}

	var freezeHours int
	if s.settingService != nil {
		freezeHours = s.settingService.GetAffiliateRebateFreezeHours(ctx)
	}

	snapshot := &AffiliateRebateSnapshot{RebateRatePercent: policy.RatePercent}
	if policy.AgentLevel != nil {
		levelID := policy.AgentLevel.ID
		snapshot.AgentLevelID = &levelID
		snapshot.AgentLevelCode = policy.AgentLevel.Code
		snapshot.AgentLevelName = policy.AgentLevel.Name
	}
	applied, err := s.repo.AccrueQuota(ctx, *inviteeSummary.InviterID, inviteeUserID, rebate, freezeHours, sourceOrderID, snapshot)
	if err != nil {
		return 0, err
	}
	if !applied {
		return 0, nil
	}
	return rebate, nil
}

// resolveRebateRatePercent returns the inviter's exclusive rate when set,
// otherwise the global setting value (clamped to [Min, Max]).
func (s *AffiliateService) resolveRebateRatePercent(ctx context.Context, inviter *AffiliateSummary) float64 {
	return s.resolveRebatePolicy(ctx, inviter).RatePercent
}

func (s *AffiliateService) resolveRebatePolicy(ctx context.Context, inviter *AffiliateSummary) affiliateRebatePolicy {
	levels, _ := s.activeAffiliateAgentLevels(ctx)
	return s.resolveRebatePolicyWithLevels(ctx, inviter, levels)
}

func (s *AffiliateService) resolveRebatePolicyWithLevels(ctx context.Context, inviter *AffiliateSummary, levels []AffiliateAgentLevel) affiliateRebatePolicy {
	if inviter != nil && inviter.AffRebateRatePercent != nil {
		v := *inviter.AffRebateRatePercent
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return affiliateRebatePolicy{RatePercent: s.globalRebateRatePercent(ctx), Source: "global"}
		}
		return affiliateRebatePolicy{RatePercent: clampAffiliateRebateRate(v), Source: "exclusive"}
	}
	if inviter != nil {
		if inviter.AffLevelManual && inviter.AffLevelID != nil {
			if level := findEnabledAffiliateAgentLevel(levels, *inviter.AffLevelID); level != nil {
				return affiliateRebatePolicy{RatePercent: clampAffiliateRebateRate(level.RebateRatePercent), AgentLevel: level, Source: "manual_level"}
			}
		}
		if level := bestQualifiedAffiliateAgentLevel(levels, inviter.AffCount, inviter.AffHistoryQuota); level != nil {
			return affiliateRebatePolicy{RatePercent: clampAffiliateRebateRate(level.RebateRatePercent), AgentLevel: level, Source: "auto_level"}
		}
	}
	if level := defaultAffiliateAgentLevel(levels); level != nil {
		return affiliateRebatePolicy{RatePercent: clampAffiliateRebateRate(level.RebateRatePercent), AgentLevel: level, Source: "default_level"}
	}
	return affiliateRebatePolicy{RatePercent: s.globalRebateRatePercent(ctx), Source: "global"}
}

func (s *AffiliateService) activeAffiliateAgentLevels(ctx context.Context) ([]AffiliateAgentLevel, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.ListAffiliateAgentLevels(ctx, false)
}

func findEnabledAffiliateAgentLevel(levels []AffiliateAgentLevel, id int64) *AffiliateAgentLevel {
	for i := range levels {
		if levels[i].ID == id && levels[i].Enabled {
			level := levels[i]
			return &level
		}
	}
	return nil
}

func bestQualifiedAffiliateAgentLevel(levels []AffiliateAgentLevel, invitedCount int, historyQuota float64) *AffiliateAgentLevel {
	var best *AffiliateAgentLevel
	for i := range levels {
		level := levels[i]
		if !level.Enabled || invitedCount < level.MinInvitedCount || historyQuota < level.MinHistoryQuota {
			continue
		}
		if best == nil || affiliateAgentLevelRanksHigher(level, *best) {
			best = &level
		}
	}
	return best
}

func defaultAffiliateAgentLevel(levels []AffiliateAgentLevel) *AffiliateAgentLevel {
	var best *AffiliateAgentLevel
	for i := range levels {
		level := levels[i]
		if !level.Enabled || !level.IsDefault {
			continue
		}
		if best == nil || affiliateAgentLevelRanksHigher(level, *best) {
			best = &level
		}
	}
	return best
}

func affiliateAgentLevelRanksHigher(a, b AffiliateAgentLevel) bool {
	if a.SortOrder != b.SortOrder {
		return a.SortOrder > b.SortOrder
	}
	if a.MinInvitedCount != b.MinInvitedCount {
		return a.MinInvitedCount > b.MinInvitedCount
	}
	if a.MinHistoryQuota != b.MinHistoryQuota {
		return a.MinHistoryQuota > b.MinHistoryQuota
	}
	if a.RebateRatePercent != b.RebateRatePercent {
		return a.RebateRatePercent > b.RebateRatePercent
	}
	return a.ID > b.ID
}

// globalRebateRatePercent reads the system-wide rebate rate via SettingService,
// returning the documented default when SettingService is unavailable.
func (s *AffiliateService) globalRebateRatePercent(ctx context.Context) float64 {
	if s == nil || s.settingService == nil {
		return AffiliateRebateRateDefault
	}
	return s.settingService.GetAffiliateRebateRatePercent(ctx)
}

func (s *AffiliateService) TransferAffiliateQuota(ctx context.Context, userID int64) (float64, float64, error) {
	if s == nil || s.repo == nil {
		return 0, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}

	transferred, balance, err := s.repo.TransferQuotaToBalance(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	if transferred > 0 {
		s.invalidateAffiliateCaches(ctx, userID)
	}
	return transferred, balance, nil
}

func (s *AffiliateService) listInvitees(ctx context.Context, inviterID int64) ([]AffiliateInvitee, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	invitees, err := s.repo.ListInvitees(ctx, inviterID, affiliateInviteesLimit)
	if err != nil {
		return nil, err
	}
	for i := range invitees {
		invitees[i].Email = maskEmail(invitees[i].Email)
	}
	return invitees, nil
}

func roundTo(v float64, scale int) float64 {
	factor := math.Pow10(scale)
	return math.Round(v*factor) / factor
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	at := strings.Index(email, "@")
	if at <= 0 || at >= len(email)-1 {
		return "***"
	}

	local := email[:at]
	domain := email[at+1:]
	dot := strings.LastIndex(domain, ".")

	maskedLocal := maskSegment(local)
	if dot <= 0 || dot >= len(domain)-1 {
		return maskedLocal + "@" + maskSegment(domain)
	}

	domainName := domain[:dot]
	tld := domain[dot:]
	return maskedLocal + "@" + maskSegment(domainName) + tld
}

func maskSegment(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return "***"
	}
	if len(r) == 1 {
		return string(r[0]) + "***"
	}
	return string(r[0]) + "***"
}

func (s *AffiliateService) invalidateAffiliateCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			logger.LegacyPrintf("service.affiliate", "[Affiliate] Failed to invalidate billing cache for user %d: %v", userID, err)
		}
	}
}

// =========================
// Admin: 专属配置管理
// =========================

// validateExclusiveRate ensures a per-user override is finite and within
// [Min, Max]. nil is always valid (means "clear / fall back to global").
func validateExclusiveRate(ratePercent *float64) error {
	if ratePercent == nil {
		return nil
	}
	v := *ratePercent
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return infraerrors.BadRequest("INVALID_RATE", "invalid rebate rate")
	}
	if v < AffiliateRebateRateMin || v > AffiliateRebateRateMax {
		return infraerrors.BadRequest("INVALID_RATE", "rebate rate out of range")
	}
	return nil
}

func validateAffiliateAgentLevelInput(input *AffiliateAgentLevelInput) error {
	if input == nil {
		return infraerrors.BadRequest("INVALID_AGENT_LEVEL", "invalid agent level")
	}
	input.Code = strings.ToUpper(strings.TrimSpace(input.Code))
	input.Name = strings.TrimSpace(input.Name)
	if len(input.Code) < AffiliateLevelCodeMinLength || len(input.Code) > AffiliateLevelCodeMaxLength {
		return infraerrors.BadRequest("INVALID_AGENT_LEVEL_CODE", "invalid agent level code")
	}
	for i := 0; i < len(input.Code); i++ {
		if !affiliateCodeValidChar[input.Code[i]] {
			return infraerrors.BadRequest("INVALID_AGENT_LEVEL_CODE", "invalid agent level code")
		}
	}
	if input.Name == "" || len([]rune(input.Name)) > AffiliateLevelNameMaxLength {
		return infraerrors.BadRequest("INVALID_AGENT_LEVEL_NAME", "invalid agent level name")
	}
	rate := input.RebateRatePercent
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < AffiliateRebateRateMin || rate > AffiliateRebateRateMax {
		return infraerrors.BadRequest("INVALID_RATE", "rebate rate out of range")
	}
	if input.MinInvitedCount < 0 {
		return infraerrors.BadRequest("INVALID_AGENT_LEVEL_THRESHOLD", "min invited count cannot be negative")
	}
	if math.IsNaN(input.MinHistoryQuota) || math.IsInf(input.MinHistoryQuota, 0) || input.MinHistoryQuota < 0 {
		return infraerrors.BadRequest("INVALID_AGENT_LEVEL_THRESHOLD", "min history quota cannot be negative")
	}
	return nil
}

func (s *AffiliateService) AdminListAgentLevels(ctx context.Context, includeDisabled bool) ([]AffiliateAgentLevel, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateAgentLevels(ctx, includeDisabled)
}

func (s *AffiliateService) AdminCreateAgentLevel(ctx context.Context, input AffiliateAgentLevelInput) (*AffiliateAgentLevel, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateAffiliateAgentLevelInput(&input); err != nil {
		return nil, err
	}
	return s.repo.CreateAffiliateAgentLevel(ctx, input)
}

func (s *AffiliateService) AdminUpdateAgentLevel(ctx context.Context, id int64, input AffiliateAgentLevelInput) (*AffiliateAgentLevel, error) {
	if id <= 0 {
		return nil, ErrAffiliateLevelNotFound
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateAffiliateAgentLevelInput(&input); err != nil {
		return nil, err
	}
	return s.repo.UpdateAffiliateAgentLevel(ctx, id, input)
}

func (s *AffiliateService) AdminDeleteAgentLevel(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrAffiliateLevelNotFound
	}
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.DeleteAffiliateAgentLevel(ctx, id)
}

func (s *AffiliateService) AdminSetUserAgentLevel(ctx context.Context, userID int64, levelID *int64) error {
	if userID <= 0 {
		return infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if levelID != nil && *levelID <= 0 {
		return ErrAffiliateLevelNotFound
	}
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.SetUserAffiliateLevel(ctx, userID, levelID)
}

func (s *AffiliateService) AdminBatchSetUserAgentLevel(ctx context.Context, userIDs []int64, levelID *int64) error {
	if levelID != nil && *levelID <= 0 {
		return ErrAffiliateLevelNotFound
	}
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	cleaned := make([]int64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid > 0 {
			cleaned = append(cleaned, uid)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return s.repo.BatchSetUserAffiliateLevel(ctx, cleaned, levelID)
}

// AdminUpdateUserAffCode 管理员改写用户的邀请码（专属邀请码）。
func (s *AffiliateService) AdminUpdateUserAffCode(ctx context.Context, userID int64, rawCode string) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if !isValidAffiliateCodeFormat(code) {
		return ErrAffiliateCodeInvalid
	}
	return s.repo.UpdateUserAffCode(ctx, userID, code)
}

// AdminResetUserAffCode 重置用户邀请码为系统随机码。
func (s *AffiliateService) AdminResetUserAffCode(ctx context.Context, userID int64) (string, error) {
	if s == nil || s.repo == nil {
		return "", infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ResetUserAffCode(ctx, userID)
}

// AdminSetUserRebateRate 设置/清除用户专属返利比例。ratePercent==nil 表示清除。
func (s *AffiliateService) AdminSetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateExclusiveRate(ratePercent); err != nil {
		return err
	}
	return s.repo.SetUserRebateRate(ctx, userID, ratePercent)
}

// AdminBatchSetUserRebateRate 批量设置/清除用户专属返利比例。
func (s *AffiliateService) AdminBatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateExclusiveRate(ratePercent); err != nil {
		return err
	}
	cleaned := make([]int64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid > 0 {
			cleaned = append(cleaned, uid)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return s.repo.BatchSetUserRebateRate(ctx, cleaned, ratePercent)
}

// AdminListCustomUsers 列出有专属配置的用户。
func (s *AffiliateService) AdminListCustomUsers(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	entries, total, err := s.repo.ListUsersWithCustomSettings(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	levels, err := s.activeAffiliateAgentLevels(ctx)
	if err != nil {
		return nil, 0, err
	}
	for i := range entries {
		summary := &AffiliateSummary{
			UserID:               entries[i].UserID,
			AffRebateRatePercent: entries[i].AffRebateRatePercent,
			AffLevelID:           entries[i].AffLevelID,
			AffLevelManual:       entries[i].AffLevelManual,
			AffCount:             entries[i].AffCount,
			AffHistoryQuota:      entries[i].AffHistoryQuota,
		}
		policy := s.resolveRebatePolicyWithLevels(ctx, summary, levels)
		entries[i].EffectiveRebateRatePercent = policy.RatePercent
		entries[i].EffectiveAgentLevel = policy.AgentLevel
	}
	return entries, total, nil
}

func (s *AffiliateService) AdminListInviteRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateInviteRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminListRebateRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateRebateRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminListTransferRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateTransferRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminGetUserOverview(ctx context.Context, userID int64) (*AffiliateUserOverview, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	overview, err := s.repo.GetAffiliateUserOverview(ctx, userID)
	if err != nil {
		return nil, err
	}
	if overview != nil {
		summary := &AffiliateSummary{
			UserID:               overview.UserID,
			AffRebateRatePercent: overview.AffRebateRatePercent,
			AffLevelID:           overview.AffLevelID,
			AffLevelManual:       overview.AffLevelManual,
			AffCount:             overview.InvitedCount,
			AffHistoryQuota:      overview.HistoryQuota,
		}
		policy := s.resolveRebatePolicy(ctx, summary)
		overview.RebateRatePercent = clampAffiliateRebateRate(policy.RatePercent)
		overview.AgentLevel = policy.AgentLevel
		overview.RebateRateSource = policy.Source
	}
	return overview, nil
}

func normalizeAffiliateRecordFilter(filter AffiliateRecordFilter) AffiliateRecordFilter {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.SortBy = strings.TrimSpace(filter.SortBy)
	return filter
}
