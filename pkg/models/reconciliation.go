package models

import "encoding/json"

const (
	FinancialObservationStatusPending  = "pending"
	FinancialObservationStatusRetrying = "retrying"
	FinancialObservationStatusReview   = "review"
)

// FinancialObservation is an immutable, user-scoped report received from a
// reconciliation source. RawPayload and NormalizedSnapshot must be sanitized
// before this persistence boundary is called and must never contain file data.
type FinancialObservation struct {
	ObservationId       int64           `xorm:"PK"`
	Uid                 int64           `xorm:"UNIQUE(UQE_financial_observation_source_version) INDEX(IDX_financial_observation_uid_status_received) NOT NULL"`
	Source              string          `xorm:"VARCHAR(32) UNIQUE(UQE_financial_observation_source_version) NOT NULL"`
	SourceConnectionId  string          `xorm:"VARCHAR(128) UNIQUE(UQE_financial_observation_source_version) NOT NULL"`
	SourceObservationId string          `xorm:"VARCHAR(256) UNIQUE(UQE_financial_observation_source_version) NOT NULL"`
	SourceVersion       string          `xorm:"VARCHAR(128) UNIQUE(UQE_financial_observation_source_version) NOT NULL"`
	ExpenseKind         string          `xorm:"VARCHAR(16) NOT NULL"`
	RawPayload          json.RawMessage `xorm:"JSONB NOT NULL"`
	NormalizedSnapshot  json.RawMessage `xorm:"JSONB NOT NULL"`
	ReceiptPictureId    int64           `xorm:"INDEX(IDX_financial_observation_uid_receipt_picture_id)"`
	Status              string          `xorm:"VARCHAR(32) INDEX(IDX_financial_observation_uid_status_received) NOT NULL"`
	ReceivedUnixTime    int64           `xorm:"INDEX(IDX_financial_observation_uid_status_received) NOT NULL"`
	SupersedesId        int64           `xorm:"INDEX"`
	RetryCount          int32           `xorm:"NOT NULL"`
	NextRetryUnixTime   int64           `xorm:"INDEX"`
	LastErrorClass      string          `xorm:"VARCHAR(64)"`
	CreatedUnixTime     int64           `xorm:"NOT NULL"`
	UpdatedUnixTime     int64           `xorm:"NOT NULL"`
}

func (*FinancialObservation) TableName() string { return "financial_observation" }

// ObservationExternalRef stores indexed, namespaced deterministic identifiers.
type ObservationExternalRef struct {
	ExternalRefId   int64  `xorm:"PK"`
	Uid             int64  `xorm:"UNIQUE(UQE_observation_external_ref) INDEX(IDX_observation_external_ref_uid_namespace_value) NOT NULL"`
	ObservationId   int64  `xorm:"UNIQUE(UQE_observation_external_ref) INDEX NOT NULL"`
	Namespace       string `xorm:"VARCHAR(64) UNIQUE(UQE_observation_external_ref) INDEX(IDX_observation_external_ref_uid_namespace_value) NOT NULL"`
	Value           string `xorm:"VARCHAR(256) UNIQUE(UQE_observation_external_ref) INDEX(IDX_observation_external_ref_uid_namespace_value) NOT NULL"`
	RelationType    string `xorm:"VARCHAR(32) UNIQUE(UQE_observation_external_ref) NOT NULL"`
	CreatedUnixTime int64  `xorm:"NOT NULL"`
}

func (*ObservationExternalRef) TableName() string { return "observation_external_ref" }

// TransactionObservationLink retains both active and revoked provenance links.
// The PostgreSQL partial index registered during database maintenance enforces
// at most one active row for each (uid, observation_id).
type TransactionObservationLink struct {
	LinkId          int64  `xorm:"PK"`
	Uid             int64  `xorm:"INDEX(IDX_transaction_observation_link_uid_observation_active) INDEX(IDX_transaction_observation_link_uid_transaction_active) NOT NULL"`
	ObservationId   int64  `xorm:"INDEX(IDX_transaction_observation_link_uid_observation_active) NOT NULL"`
	TransactionId   int64  `xorm:"INDEX(IDX_transaction_observation_link_uid_transaction_active) NOT NULL"`
	Active          bool   `xorm:"INDEX(IDX_transaction_observation_link_uid_observation_active) INDEX(IDX_transaction_observation_link_uid_transaction_active) NOT NULL"`
	LinkReason      string `xorm:"VARCHAR(64) NOT NULL"`
	Actor           string `xorm:"VARCHAR(32) NOT NULL"`
	AttemptId       int64  `xorm:"INDEX"`
	CreatedUnixTime int64  `xorm:"NOT NULL"`
	UpdatedUnixTime int64  `xorm:"NOT NULL"`
	RevokedReason   string `xorm:"VARCHAR(64)"`
}

func (*TransactionObservationLink) TableName() string { return "transaction_observation_link" }

// ReconciliationAttempt records an auditable engine outcome and compact,
// sanitized evidence. It deliberately does not persist general candidates.
type ReconciliationAttempt struct {
	AttemptId           int64  `xorm:"PK"`
	Uid                 int64  `xorm:"INDEX(IDX_reconciliation_attempt_uid_observation_created) NOT NULL"`
	ObservationId       int64  `xorm:"INDEX(IDX_reconciliation_attempt_uid_observation_created) NOT NULL"`
	EngineVersion       string `xorm:"VARCHAR(32) NOT NULL"`
	ScoringVersion      string `xorm:"VARCHAR(32) NOT NULL"`
	Decision            string `xorm:"VARCHAR(16) NOT NULL"`
	TargetTransactionId int64  `xorm:"INDEX"`
	Confidence          float64
	DecisionReason      string          `xorm:"VARCHAR(64) NOT NULL"`
	EvidenceSummary     json.RawMessage `xorm:"JSONB NOT NULL"`
	ErrorClass          string          `xorm:"VARCHAR(64)"`
	CreatedUnixTime     int64           `xorm:"INDEX(IDX_reconciliation_attempt_uid_observation_created) NOT NULL"`
}

func (*ReconciliationAttempt) TableName() string { return "reconciliation_attempt" }

// ReconciliationReview is the bounded user work item produced for ambiguity.
type ReconciliationReview struct {
	ReviewId                 int64  `xorm:"PK"`
	Uid                      int64  `xorm:"INDEX(IDX_reconciliation_review_uid_status_created) NOT NULL"`
	ObservationId            int64  `xorm:"INDEX NOT NULL"`
	AttemptId                int64  `xorm:"UNIQUE NOT NULL"`
	Status                   string `xorm:"VARCHAR(32) INDEX(IDX_reconciliation_review_uid_status_created) NOT NULL"`
	RecommendedTransactionId int64
	AlternativesSnapshot     json.RawMessage `xorm:"JSONB NOT NULL"`
	Resolution               string          `xorm:"VARCHAR(32)"`
	Resolver                 string          `xorm:"VARCHAR(64)"`
	CreatedUnixTime          int64           `xorm:"INDEX(IDX_reconciliation_review_uid_status_created) NOT NULL"`
	UpdatedUnixTime          int64           `xorm:"NOT NULL"`
	ResolvedUnixTime         int64
}

func (*ReconciliationReview) TableName() string { return "reconciliation_review" }
