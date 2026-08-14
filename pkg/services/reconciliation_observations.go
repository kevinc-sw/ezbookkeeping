package services

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

var ErrUnsafeObservationSnapshot = errors.New("observation snapshot contains sensitive or binary data")

// ReconciliationObservationService persists reconciliation provenance using the
// existing user-data store. Every operation takes UID explicitly.
type ReconciliationObservationService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

var ReconciliationObservations = &ReconciliationObservationService{
	ServiceUsingDB:   ServiceUsingDB{container: datastore.Container},
	ServiceUsingUuid: ServiceUsingUuid{container: uuid.Container},
}

// InsertOrGetObservation inserts one immutable source version or returns the
// row already stored for the user-scoped idempotency key.
func (s *ReconciliationObservationService) InsertOrGetObservation(c core.Context, uid int64, observation *models.FinancialObservation) (*models.FinancialObservation, bool, error) {
	if uid <= 0 || observation == nil || observation.Uid != uid {
		return nil, false, errs.ErrUserIdInvalid
	}
	if err := validateObservationSnapshot(observation.RawPayload); err != nil {
		return nil, false, err
	}
	if err := validateObservationSnapshot(observation.NormalizedSnapshot); err != nil {
		return nil, false, err
	}

	if existing, err := s.getBySourceVersion(c, uid, observation); err != nil {
		return nil, false, err
	} else if existing != nil {
		return existing, false, nil
	}

	now := time.Now().Unix()
	observation.ObservationId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)
	if observation.ObservationId <= 0 {
		return nil, false, errs.ErrSystemIsBusy
	}
	observation.CreatedUnixTime = now
	observation.UpdatedUnixTime = now
	_, err := s.UserDataDB(uid).NewSession(c).Insert(observation)
	if err == nil {
		return observation, true, nil
	}

	// A concurrent delivery may have won the unique-key race.
	existing, queryErr := s.getBySourceVersion(c, uid, observation)
	if queryErr == nil && existing != nil {
		return existing, false, nil
	}
	return nil, false, err
}

func (s *ReconciliationObservationService) getBySourceVersion(c core.Context, uid int64, observation *models.FinancialObservation) (*models.FinancialObservation, error) {
	existing := &models.FinancialObservation{}
	has, err := s.UserDataDB(uid).NewSession(c).Where(
		"uid=? AND source=? AND source_connection_id=? AND source_observation_id=? AND source_version=?",
		uid, observation.Source, observation.SourceConnectionId, observation.SourceObservationId, observation.SourceVersion,
	).Get(existing)
	if err != nil || !has {
		return nil, err
	}
	return existing, nil
}

// GetObservation returns only an observation owned by UID.
func (s *ReconciliationObservationService) GetObservation(c core.Context, uid, observationID int64) (*models.FinancialObservation, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	observation := &models.FinancialObservation{}
	has, err := s.UserDataDB(uid).NewSession(c).ID(observationID).Where("uid=?", uid).Get(observation)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, errs.ErrOperationFailed
	}
	return observation, nil
}

func (s *ReconciliationObservationService) AddExternalReferences(c core.Context, uid, observationID int64, references []*models.ObservationExternalRef) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	for _, reference := range references {
		if reference == nil || reference.Uid != uid || reference.ObservationId != observationID {
			return errs.ErrUserIdInvalid
		}
		reference.ExternalRefId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)
		reference.CreatedUnixTime = time.Now().Unix()
	}
	if len(references) == 0 {
		return nil
	}
	_, err := s.UserDataDB(uid).NewSession(c).Insert(references)
	return err
}

func (s *ReconciliationObservationService) GetExternalReferences(c core.Context, uid, observationID int64) ([]*models.ObservationExternalRef, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	var references []*models.ObservationExternalRef
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND observation_id=?", uid, observationID).Find(&references)
	return references, err
}

func (s *ReconciliationObservationService) UpdateProcessingState(c core.Context, uid, observationID int64, status, errorClass string, retryCount int32, nextRetry int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	update := &models.FinancialObservation{Status: status, LastErrorClass: errorClass, RetryCount: retryCount, NextRetryUnixTime: nextRetry, UpdatedUnixTime: time.Now().Unix()}
	updated, err := s.UserDataDB(uid).NewSession(c).ID(observationID).Cols("status", "last_error_class", "retry_count", "next_retry_unix_time", "updated_unix_time").Where("uid=?", uid).Update(update)
	if err == nil && updated == 0 {
		return errs.ErrOperationFailed
	}
	return err
}

func (s *ReconciliationObservationService) CreateAttempt(c core.Context, uid int64, attempt *models.ReconciliationAttempt) error {
	if uid <= 0 || attempt == nil || attempt.Uid != uid {
		return errs.ErrUserIdInvalid
	}
	if err := validateObservationSnapshot(attempt.EvidenceSummary); err != nil {
		return err
	}
	attempt.AttemptId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)
	attempt.CreatedUnixTime = time.Now().Unix()
	_, err := s.UserDataDB(uid).NewSession(c).Insert(attempt)
	return err
}

func (s *ReconciliationObservationService) GetLinks(c core.Context, uid, observationID int64, activeOnly bool) ([]*models.TransactionObservationLink, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	query := s.UserDataDB(uid).NewSession(c).Where("uid=? AND observation_id=?", uid, observationID)
	if activeOnly {
		query = query.And("active=?", true)
	}
	var links []*models.TransactionObservationLink
	err := query.OrderBy("created_unix_time asc").Find(&links)
	return links, err
}

func (s *ReconciliationObservationService) CreateLink(c core.Context, uid int64, link *models.TransactionObservationLink) error {
	if uid <= 0 || link == nil || link.Uid != uid {
		return errs.ErrUserIdInvalid
	}
	link.LinkId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)
	now := time.Now().Unix()
	link.CreatedUnixTime, link.UpdatedUnixTime = now, now
	_, err := s.UserDataDB(uid).NewSession(c).Insert(link)
	return err
}

func (s *ReconciliationObservationService) CreateReview(c core.Context, uid int64, review *models.ReconciliationReview) error {
	if uid <= 0 || review == nil || review.Uid != uid {
		return errs.ErrUserIdInvalid
	}
	if err := validateObservationSnapshot(review.AlternativesSnapshot); err != nil {
		return err
	}
	review.ReviewId = s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)
	now := time.Now().Unix()
	review.CreatedUnixTime, review.UpdatedUnixTime = now, now
	_, err := s.UserDataDB(uid).NewSession(c).Insert(review)
	return err
}

func (s *ReconciliationObservationService) GetReviews(c core.Context, uid, observationID int64) ([]*models.ReconciliationReview, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	var reviews []*models.ReconciliationReview
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND observation_id=?", uid, observationID).OrderBy("created_unix_time asc").Find(&reviews)
	return reviews, err
}

func validateObservationSnapshot(snapshot json.RawMessage) error {
	if len(snapshot) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(snapshot, &value); err != nil {
		return err
	}
	return inspectSnapshotValue(value)
}

func inspectSnapshotValue(value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalizedKey := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
			switch normalizedKey {
			case "accesstoken", "password", "secret", "credential", "filebytes", "imagebytes", "binary", "bytes":
				return ErrUnsafeObservationSnapshot
			}
			if err := inspectSnapshotValue(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := inspectSnapshotValue(child); err != nil {
				return err
			}
		}
	}
	return nil
}
