// Package reconciliation defines the provider- and persistence-independent
// values exchanged by the reconciliation engine.
package reconciliation

import (
	"context"
	"time"
)

type Decision string

const (
	DecisionMatch  Decision = "MATCH"
	DecisionReview Decision = "REVIEW"
	DecisionNew    Decision = "NEW"
)

type TimePrecision string

const (
	TimePrecisionUnknown TimePrecision = "unknown"
	TimePrecisionDate    TimePrecision = "date"
	TimePrecisionInstant TimePrecision = "instant"
)

// ExternalReference is an authoritative identifier in a source-owned namespace.
type ExternalReference struct {
	Namespace string
	Value     string
}

// NormalizedObservation is the immutable input to deterministic and fuzzy matching.
// Amount is stored in the currency's minor unit; ReceiptPictureID is metadata only.
type NormalizedObservation struct {
	ContractVersion      string
	ObservationID        int64
	UID                  int64
	Source               string
	SourceConnectionID   string
	SourceObservationID  string
	SourceVersion        string
	NormalizationVersion string
	Amount               int64
	Currency             string
	Merchant             string
	OccurredAt           time.Time
	LocalDate            string
	TimePrecision        TimePrecision
	AccountID            *int64
	PaymentHint          string
	ExternalRefs         []ExternalReference
	ReceiptPictureID     *int64
}

// Candidate is a read-only projection of a canonical expense. It intentionally
// contains no transaction persistence model or mutation method.
type Candidate struct {
	TransactionID int64
	UID           int64
	Amount        int64
	Currency      string
	Merchant      string
	OccurredAt    time.Time
	LocalDate     string
	TimePrecision TimePrecision
	AccountID     int64
	PaymentHint   string
	ExternalRefs  []ExternalReference
}

type Evidence struct {
	Code          string
	TransactionID *int64
	MaskedValue   string
}

type Feature struct {
	Available bool
	Score     float64
	Evidence  []Evidence
}

type Features struct {
	Amount         Feature
	Merchant       Feature
	Date           Feature
	AccountPayment Feature
	Currency       Feature
}

type Conflict struct {
	Code           string
	TransactionIDs []int64
	Evidence       []Evidence
}

type ScoredCandidate struct {
	Candidate Candidate
	Features  Features
	Score     float64
	Coverage  float64
	Conflicts []Conflict
}

type Alternative struct {
	TransactionID int64
	Score         float64
	Evidence      []Evidence
}

// Result is the complete pure-engine outcome. Every outcome is versioned and
// carries a named reason plus compact, already-masked evidence.
type Result struct {
	Decision            Decision
	ReasonCode          string
	TargetTransactionID *int64
	Evidence            []Evidence
	Conflicts           []Conflict
	Alternatives        []Alternative
	PolicyVersion       string
	EngineVersion       string
}

type CandidateQuery struct {
	Observation NormalizedObservation
	Limit       int
}

// CandidateReader supplies bounded fuzzy candidates without exposing writes.
type CandidateReader interface {
	FindCandidates(ctx context.Context, query CandidateQuery) ([]Candidate, error)
}

// DeterministicReader supplies only the authoritative lookups required before
// fuzzy matching. Implementations must scope every lookup by UID.
type DeterministicReader interface {
	FindTransactionByID(ctx context.Context, uid int64, transactionID int64) (*Candidate, error)
	FindLinkedTransaction(ctx context.Context, uid int64, observationID int64) (*Candidate, error)
	FindTransactionsByExternalReference(ctx context.Context, uid int64, reference ExternalReference) ([]Candidate, error)
}
