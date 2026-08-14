package models

import "encoding/json"

// Reconciliation observation sources.
const (
	RECONCILIATION_SOURCE_PLAID       = "plaid"
	RECONCILIATION_SOURCE_RECEIPT_OCR = "receipt_ocr"
	RECONCILIATION_SOURCE_MANUAL      = "manual"
)

// Financial observation processing statuses used by persistence and receipt retention.
const (
	FINANCIAL_OBSERVATION_STATUS_PENDING         = "pending"
	FINANCIAL_OBSERVATION_STATUS_RETRYING        = "retrying"
	FINANCIAL_OBSERVATION_STATUS_AWAITING_REVIEW = "awaiting_review"
	FINANCIAL_OBSERVATION_STATUS_RECONCILED      = "reconciled"
	FINANCIAL_OBSERVATION_STATUS_FAILED          = "failed"
)

const RECONCILIATION_REVIEW_STATUS_OPEN = "open"

// FinancialObservation stores an immutable, sanitized source report and its mutable processing state.
type FinancialObservation struct {
	ObservationId           int64           `xorm:"PK UNIQUE(UQE_financial_observation_uid_observation_id)"`
	Uid                     int64           `xorm:"UNIQUE(UQE_financial_observation_source_version) UNIQUE(UQE_financial_observation_uid_observation_id) INDEX(IDX_financial_observation_uid_status_received) NOT NULL"`
	Source                  string          `xorm:"VARCHAR(32) UNIQUE(UQE_financial_observation_source_version) NOT NULL"`
	SourceConnectionId      string          `xorm:"VARCHAR(128) UNIQUE(UQE_financial_observation_source_version) NOT NULL"`
	SourceObservationId     string          `xorm:"VARCHAR(191) UNIQUE(UQE_financial_observation_source_version) NOT NULL"`
	SourceVersion           string          `xorm:"VARCHAR(128) UNIQUE(UQE_financial_observation_source_version) NOT NULL"`
	ExpenseKind             string          `xorm:"VARCHAR(32) NOT NULL"`
	SanitizedRawSnapshot    json.RawMessage `xorm:"JSONB NOT NULL"`
	NormalizedSnapshot      json.RawMessage `xorm:"JSONB NOT NULL"`
	NormalizationVersion    string          `xorm:"VARCHAR(64) NOT NULL"`
	ReceiptPictureId        *int64          `xorm:"INDEX(IDX_financial_observation_uid_receipt_picture_id)"`
	Status                  string          `xorm:"VARCHAR(32) INDEX(IDX_financial_observation_uid_status_received) NOT NULL"`
	ReceivedUnixTime        int64           `xorm:"INDEX(IDX_financial_observation_uid_status_received) NOT NULL"`
	SupersedesObservationId *int64          `xorm:"INDEX(IDX_financial_observation_uid_supersedes_id)"`
	RetryCount              uint8           `xorm:"NOT NULL"`
	NextRetryUnixTime       int64
	LastErrorClass          string `xorm:"VARCHAR(64) NOT NULL"`
	CreatedUnixTime         int64  `xorm:"NOT NULL"`
	UpdatedUnixTime         int64  `xorm:"NOT NULL"`
}

// ObservationExternalRef stores an indexed, namespaced deterministic source relationship.
type ObservationExternalRef struct {
	ExternalRefId   int64  `xorm:"PK"`
	Uid             int64  `xorm:"UNIQUE(UQE_observation_external_ref_identity) INDEX(IDX_observation_external_ref_uid_namespace_value) INDEX(IDX_observation_external_ref_uid_observation_id) NOT NULL"`
	ObservationId   int64  `xorm:"UNIQUE(UQE_observation_external_ref_identity) INDEX(IDX_observation_external_ref_uid_observation_id) NOT NULL"`
	Namespace       string `xorm:"VARCHAR(64) UNIQUE(UQE_observation_external_ref_identity) INDEX(IDX_observation_external_ref_uid_namespace_value) NOT NULL"`
	Value           string `xorm:"VARCHAR(191) UNIQUE(UQE_observation_external_ref_identity) INDEX(IDX_observation_external_ref_uid_namespace_value) NOT NULL"`
	RelationType    string `xorm:"VARCHAR(64) UNIQUE(UQE_observation_external_ref_identity) NOT NULL"`
	CreatedUnixTime int64  `xorm:"NOT NULL"`
}

// TransactionObservationLink stores current and historical canonical provenance.
type TransactionObservationLink struct {
	LinkId          int64  `xorm:"PK"`
	Uid             int64  `xorm:"INDEX(IDX_transaction_observation_link_uid_observation_id) INDEX(IDX_transaction_observation_link_uid_transaction_id) NOT NULL"`
	ObservationId   int64  `xorm:"INDEX(IDX_transaction_observation_link_uid_observation_id) NOT NULL"`
	TransactionId   int64  `xorm:"INDEX(IDX_transaction_observation_link_uid_transaction_id) NOT NULL"`
	Active          bool   `xorm:"NOT NULL"`
	LinkReason      string `xorm:"VARCHAR(64) NOT NULL"`
	Actor           string `xorm:"VARCHAR(64) NOT NULL"`
	AttemptId       *int64 `xorm:"INDEX(IDX_transaction_observation_link_attempt_id)"`
	CreatedUnixTime int64  `xorm:"NOT NULL"`
	RevokedUnixTime int64
	RevokedReason   string `xorm:"VARCHAR(128) NOT NULL"`
}

// ReconciliationAttempt records one final, versioned engine result.
type ReconciliationAttempt struct {
	AttemptId            int64  `xorm:"PK UNIQUE(UQE_reconciliation_attempt_uid_attempt_id)"`
	Uid                  int64  `xorm:"UNIQUE(UQE_reconciliation_attempt_uid_attempt_id) INDEX(IDX_reconciliation_attempt_uid_observation_id_created) NOT NULL"`
	ObservationId        int64  `xorm:"INDEX(IDX_reconciliation_attempt_uid_observation_id_created) NOT NULL"`
	EngineVersion        string `xorm:"VARCHAR(64) NOT NULL"`
	ScoringPolicyVersion string `xorm:"VARCHAR(64) NOT NULL"`
	Decision             string `xorm:"VARCHAR(16) NOT NULL"`
	TargetTransactionId  *int64
	Confidence           float64
	DecisionReason       string          `xorm:"VARCHAR(64) NOT NULL"`
	EvidenceSummary      json.RawMessage `xorm:"JSONB NOT NULL"`
	ErrorClass           string          `xorm:"VARCHAR(64) NOT NULL"`
	CreatedUnixTime      int64           `xorm:"INDEX(IDX_reconciliation_attempt_uid_observation_id_created) NOT NULL"`
}

// ReconciliationReview stores the bounded alternatives needed for user resolution.
type ReconciliationReview struct {
	ReviewId                 int64  `xorm:"PK"`
	Uid                      int64  `xorm:"INDEX(IDX_reconciliation_review_uid_status_created) INDEX(IDX_reconciliation_review_uid_observation_id) NOT NULL"`
	ObservationId            int64  `xorm:"INDEX(IDX_reconciliation_review_uid_observation_id) NOT NULL"`
	AttemptId                int64  `xorm:"INDEX(IDX_reconciliation_review_attempt_id) NOT NULL"`
	Status                   string `xorm:"VARCHAR(32) INDEX(IDX_reconciliation_review_uid_status_created) NOT NULL"`
	RecommendedTransactionId *int64
	AlternativesSnapshot     json.RawMessage `xorm:"JSONB NOT NULL"`
	Resolution               string          `xorm:"VARCHAR(64) NOT NULL"`
	ResolvedBy               string          `xorm:"VARCHAR(64) NOT NULL"`
	CreatedUnixTime          int64           `xorm:"INDEX(IDX_reconciliation_review_uid_status_created) NOT NULL"`
	UpdatedUnixTime          int64           `xorm:"NOT NULL"`
	ResolvedUnixTime         int64
}
