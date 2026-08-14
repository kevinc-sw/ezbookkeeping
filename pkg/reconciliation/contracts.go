// Package reconciliation contains pure, provider-independent reconciliation
// contracts and engine logic. Persistence and canonical transaction mutation
// belong to application services outside this package.
package reconciliation

import "context"

const (
	EngineVersionV1 = "reconciliation-engine-v1"
	PolicyVersionV1 = "reconciliation-policy-v1"
)

type Decision string

const (
	DecisionMatch  Decision = "MATCH"
	DecisionReview Decision = "REVIEW"
	DecisionNew    Decision = "NEW"
)

type DatePrecision string

const (
	DatePrecisionUnknown DatePrecision = "unknown"
	DatePrecisionDay     DatePrecision = "day"
	DatePrecisionInstant DatePrecision = "instant"
)

// ExternalReference is a namespaced source identifier or relationship.
type ExternalReference struct {
	Namespace    string
	Value        string
	RelationType string
}

// NormalizedObservation is the versioned, sanitized input to the pure engine.
type NormalizedObservation struct {
	ObservationID        int64
	UID                  int64
	Kind                 string
	Source               string
	SourceConnectionID   string
	SourceObservationID  string
	SourceVersion        string
	Amount               int64
	Currency             string
	MerchantRaw          string
	MerchantNormalized   string
	OccurredUnixTime     int64
	LocalPurchaseDate    string
	DatePrecision        DatePrecision
	AccountHintID        int64
	PaymentMethodHint    string
	ExternalReferences   []ExternalReference
	SourceStatus         string
	ExplicitTargetID     int64
	NormalizationVersion string
}

// CanonicalCandidate is a read-only projection, not a persistence model.
type CanonicalCandidate struct {
	TransactionID       int64
	UID                 int64
	Kind                string
	Amount              int64
	Currency            string
	Merchant            string
	TransactionUnixTime int64
	LocalDate           string
	AccountID           int64
	PaymentMethodHint   string
	ExternalReferences  []ExternalReference
}

type Evidence struct {
	Code    string
	Summary string
}

type DeterministicEvidence struct {
	RuleCode            string
	TargetTransactionID int64
	Evidence            []Evidence
}

type Feature struct {
	Name      string
	Value     float64
	Available bool
	Evidence  Evidence
}

type Conflict struct {
	Code     string
	Evidence Evidence
}

type ScoredCandidate struct {
	Candidate CanonicalCandidate
	Features  []Feature
	Conflicts []Conflict
	Score     float64
	Coverage  float64
}

type Alternative struct {
	TransactionID int64
	Score         float64
	Evidence      []Evidence
}

// Result is compact and safe to persist or expose after application-layer
// masking. It intentionally cannot write a canonical transaction.
type Result struct {
	Decision            Decision
	ReasonCode          string
	TargetTransactionID int64
	Evidence            []Evidence
	Alternatives        []Alternative
	PolicyVersion       string
	EngineVersion       string
}

// CandidateReader supplies bounded fuzzy candidates without exposing ORM data.
type CandidateReader interface {
	FindCandidates(ctx context.Context, observation NormalizedObservation) ([]CanonicalCandidate, bool, error)
}

// DeterministicTargetReader resolves authoritative relationships independently
// from fuzzy candidate bounds.
type DeterministicTargetReader interface {
	ResolveTargets(ctx context.Context, observation NormalizedObservation) ([]DeterministicEvidence, error)
}
