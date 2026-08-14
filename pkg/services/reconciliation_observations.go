package services

import (
	"encoding/json"
	"fmt"
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// ReconciliationObservationRepository persists observations and their audit trail.
// Every method requires an explicit UID and every query applies it.
type ReconciliationObservationRepository struct {
	ServiceUsingDB
	ServiceUsingUuid
}

var ReconciliationObservations = &ReconciliationObservationRepository{
	ServiceUsingDB:   ServiceUsingDB{container: datastore.Container},
	ServiceUsingUuid: ServiceUsingUuid{container: uuid.Container},
}

type ObservationProcessingState struct {
	Status            string
	RetryCount        uint8
	NextRetryUnixTime int64
	LastErrorClass    string
}

func (r *ReconciliationObservationRepository) GetObservation(c core.Context, uid, observationId int64) (*models.FinancialObservation, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	if observationId <= 0 {
		return nil, errs.ErrReconciliationObservationInvalid
	}

	observation := &models.FinancialObservation{}
	has, err := r.UserDataDB(uid).NewSession(c).ID(observationId).Where("uid=?", uid).Get(observation)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrReconciliationObservationNotFound
	}
	return observation, nil
}

// CreateOrGetObservation makes a source version idempotent. A different source
// version is inserted as a new immutable row; existing rows are never updated.
func (r *ReconciliationObservationRepository) CreateOrGetObservation(c core.Context, uid int64, observation *models.FinancialObservation) (*models.FinancialObservation, bool, error) {
	if uid <= 0 {
		return nil, false, errs.ErrUserIdInvalid
	}
	if observation == nil || (observation.Uid != 0 && observation.Uid != uid) ||
		observation.Source == "" || observation.SourceObservationId == "" || observation.SourceVersion == "" {
		return nil, false, errs.ErrReconciliationObservationInvalid
	}
	if !json.Valid(observation.SanitizedRawSnapshot) || !json.Valid(observation.NormalizedSnapshot) {
		return nil, false, errs.ErrReconciliationSnapshotInvalid
	}

	database := r.UserDataDB(uid)
	var result *models.FinancialObservation
	created := false
	err := database.DoTransaction(c, func(sess *xorm.Session) error {
		if database.IsPostgres() {
			lockKey := fmt.Sprintf("%d\x1f%s\x1f%s\x1f%s\x1f%s", uid, observation.Source, observation.SourceConnectionId, observation.SourceObservationId, observation.SourceVersion)
			if _, err := sess.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", lockKey); err != nil {
				return err
			}
		}

		existing, err := findObservationBySourceVersion(sess, uid, observation)
		if err != nil {
			return err
		} else if existing != nil {
			result = existing
			return nil
		}

		now := time.Now().Unix()
		stored := *observation
		stored.Uid = uid
		stored.SanitizedRawSnapshot = cloneJSON(observation.SanitizedRawSnapshot)
		stored.NormalizedSnapshot = cloneJSON(observation.NormalizedSnapshot)
		if stored.ObservationId <= 0 {
			stored.ObservationId = r.GenerateUuid(uuid.UUID_TYPE_DEFAULT)
		}
		if stored.ObservationId <= 0 {
			return errs.ErrReconciliationObservationInvalid
		}
		if stored.CreatedUnixTime == 0 {
			stored.CreatedUnixTime = now
		}
		if stored.UpdatedUnixTime == 0 {
			stored.UpdatedUnixTime = now
		}
		if stored.ReceivedUnixTime == 0 {
			stored.ReceivedUnixTime = now
		}

		if _, err = sess.Insert(&stored); err != nil {
			return err
		}
		result, created = &stored, true
		return nil
	})
	return result, created, err
}

func (r *ReconciliationObservationRepository) LoadObservationSnapshots(c core.Context, uid, observationId int64) (json.RawMessage, json.RawMessage, error) {
	observation, err := r.GetObservation(c, uid, observationId)
	if err != nil {
		return nil, nil, err
	}
	return cloneJSON(observation.SanitizedRawSnapshot), cloneJSON(observation.NormalizedSnapshot), nil
}

func (r *ReconciliationObservationRepository) AddExternalReference(c core.Context, uid int64, reference *models.ObservationExternalRef) (*models.ObservationExternalRef, bool, error) {
	if uid <= 0 {
		return nil, false, errs.ErrUserIdInvalid
	}
	if reference == nil || reference.ObservationId <= 0 || (reference.Uid != 0 && reference.Uid != uid) || reference.Namespace == "" || reference.Value == "" || reference.RelationType == "" {
		return nil, false, errs.ErrReconciliationRecordInvalid
	}

	var result *models.ObservationExternalRef
	created := false
	err := r.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := requireObservation(sess, uid, reference.ObservationId); err != nil {
			return err
		}
		existing := &models.ObservationExternalRef{}
		has, err := sess.Where("uid=? AND observation_id=? AND namespace=? AND value=? AND relation_type=?", uid, reference.ObservationId, reference.Namespace, reference.Value, reference.RelationType).Get(existing)
		if err != nil {
			return err
		} else if has {
			result = existing
			return nil
		}

		stored := *reference
		stored.Uid = uid
		if stored.ExternalRefId <= 0 {
			stored.ExternalRefId = r.GenerateUuid(uuid.UUID_TYPE_DEFAULT)
		}
		if stored.ExternalRefId <= 0 {
			return errs.ErrReconciliationRecordInvalid
		}
		if stored.CreatedUnixTime == 0 {
			stored.CreatedUnixTime = time.Now().Unix()
		}
		if _, err = sess.Insert(&stored); err != nil {
			return err
		}
		result, created = &stored, true
		return nil
	})
	return result, created, err
}

func (r *ReconciliationObservationRepository) GetExternalReferences(c core.Context, uid, observationId int64) ([]*models.ObservationExternalRef, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	if observationId <= 0 {
		return nil, errs.ErrReconciliationRecordInvalid
	}
	var references []*models.ObservationExternalRef
	err := r.UserDataDB(uid).NewSession(c).Where("uid=? AND observation_id=?", uid, observationId).OrderBy("external_ref_id asc").Find(&references)
	return references, err
}

func (r *ReconciliationObservationRepository) UpdateProcessingState(c core.Context, uid, observationId int64, state ObservationProcessingState) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	if observationId <= 0 || state.Status == "" {
		return errs.ErrReconciliationRecordInvalid
	}
	updated, err := r.UserDataDB(uid).NewSession(c).ID(observationId).Where("uid=?", uid).
		Cols("status", "retry_count", "next_retry_unix_time", "last_error_class", "updated_unix_time").
		Update(&models.FinancialObservation{Status: state.Status, RetryCount: state.RetryCount, NextRetryUnixTime: state.NextRetryUnixTime, LastErrorClass: state.LastErrorClass, UpdatedUnixTime: time.Now().Unix()})
	if err == nil && updated != 1 {
		return errs.ErrReconciliationObservationNotFound
	}
	return err
}

func (r *ReconciliationObservationRepository) AddAttempt(c core.Context, uid int64, attempt *models.ReconciliationAttempt) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	if attempt == nil || attempt.ObservationId <= 0 || (attempt.Uid != 0 && attempt.Uid != uid) {
		return errs.ErrReconciliationRecordInvalid
	}
	if !json.Valid(attempt.EvidenceSummary) {
		return errs.ErrReconciliationSnapshotInvalid
	}
	return r.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := requireObservation(sess, uid, attempt.ObservationId); err != nil {
			return err
		}
		attempt.Uid = uid
		if attempt.AttemptId <= 0 {
			attempt.AttemptId = r.GenerateUuid(uuid.UUID_TYPE_DEFAULT)
		}
		if attempt.CreatedUnixTime == 0 {
			attempt.CreatedUnixTime = time.Now().Unix()
		}
		if attempt.AttemptId <= 0 {
			return errs.ErrReconciliationRecordInvalid
		}
		_, err := sess.Insert(attempt)
		return err
	})
}

func (r *ReconciliationObservationRepository) GetAttempts(c core.Context, uid, observationId int64) ([]*models.ReconciliationAttempt, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	if observationId <= 0 {
		return nil, errs.ErrReconciliationRecordInvalid
	}
	var attempts []*models.ReconciliationAttempt
	err := r.UserDataDB(uid).NewSession(c).Where("uid=? AND observation_id=?", uid, observationId).OrderBy("created_unix_time asc, attempt_id asc").Find(&attempts)
	return attempts, err
}

func (r *ReconciliationObservationRepository) AddLink(c core.Context, uid int64, link *models.TransactionObservationLink) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	if link == nil || link.ObservationId <= 0 || link.TransactionId <= 0 || (link.Uid != 0 && link.Uid != uid) {
		return errs.ErrReconciliationRecordInvalid
	}
	return r.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := requireObservation(sess, uid, link.ObservationId); err != nil {
			return err
		}
		if link.AttemptId != nil {
			if err := requireAttempt(sess, uid, link.ObservationId, *link.AttemptId); err != nil {
				return err
			}
		}
		link.Uid = uid
		if link.LinkId <= 0 {
			link.LinkId = r.GenerateUuid(uuid.UUID_TYPE_DEFAULT)
		}
		if link.CreatedUnixTime == 0 {
			link.CreatedUnixTime = time.Now().Unix()
		}
		if link.LinkId <= 0 {
			return errs.ErrReconciliationRecordInvalid
		}
		_, err := sess.Insert(link)
		return err
	})
}

func (r *ReconciliationObservationRepository) RevokeActiveLink(c core.Context, uid, observationId int64, reason string) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	if observationId <= 0 || reason == "" {
		return errs.ErrReconciliationRecordInvalid
	}
	updated, err := r.UserDataDB(uid).NewSession(c).Where("uid=? AND observation_id=? AND active=?", uid, observationId, true).
		Cols("active", "revoked_unix_time", "revoked_reason").
		Update(&models.TransactionObservationLink{Active: false, RevokedUnixTime: time.Now().Unix(), RevokedReason: reason})
	if err == nil && updated != 1 {
		return errs.ErrReconciliationRecordNotFound
	}
	return err
}

func (r *ReconciliationObservationRepository) GetLinks(c core.Context, uid, observationId int64) ([]*models.TransactionObservationLink, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	if observationId <= 0 {
		return nil, errs.ErrReconciliationRecordInvalid
	}
	var links []*models.TransactionObservationLink
	err := r.UserDataDB(uid).NewSession(c).Where("uid=? AND observation_id=?", uid, observationId).OrderBy("created_unix_time asc, link_id asc").Find(&links)
	return links, err
}

func (r *ReconciliationObservationRepository) AddReview(c core.Context, uid int64, review *models.ReconciliationReview) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	if review == nil || review.ObservationId <= 0 || review.AttemptId <= 0 || (review.Uid != 0 && review.Uid != uid) {
		return errs.ErrReconciliationRecordInvalid
	}
	if !json.Valid(review.AlternativesSnapshot) {
		return errs.ErrReconciliationSnapshotInvalid
	}
	return r.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := requireObservation(sess, uid, review.ObservationId); err != nil {
			return err
		}
		if err := requireAttempt(sess, uid, review.ObservationId, review.AttemptId); err != nil {
			return err
		}
		review.Uid = uid
		if review.ReviewId <= 0 {
			review.ReviewId = r.GenerateUuid(uuid.UUID_TYPE_DEFAULT)
		}
		now := time.Now().Unix()
		if review.CreatedUnixTime == 0 {
			review.CreatedUnixTime = now
		}
		if review.UpdatedUnixTime == 0 {
			review.UpdatedUnixTime = now
		}
		if review.ReviewId <= 0 {
			return errs.ErrReconciliationRecordInvalid
		}
		_, err := sess.Insert(review)
		return err
	})
}

func (r *ReconciliationObservationRepository) GetReviews(c core.Context, uid, observationId int64) ([]*models.ReconciliationReview, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	if observationId <= 0 {
		return nil, errs.ErrReconciliationRecordInvalid
	}
	var reviews []*models.ReconciliationReview
	err := r.UserDataDB(uid).NewSession(c).Where("uid=? AND observation_id=?", uid, observationId).OrderBy("created_unix_time asc, review_id asc").Find(&reviews)
	return reviews, err
}

func findObservationBySourceVersion(sess *xorm.Session, uid int64, observation *models.FinancialObservation) (*models.FinancialObservation, error) {
	existing := &models.FinancialObservation{}
	has, err := sess.Where("uid=? AND source=? AND source_connection_id=? AND source_observation_id=? AND source_version=?", uid, observation.Source, observation.SourceConnectionId, observation.SourceObservationId, observation.SourceVersion).Get(existing)
	if err != nil || !has {
		return nil, err
	}
	return existing, nil
}

func requireObservation(sess *xorm.Session, uid, observationId int64) error {
	has, err := sess.ID(observationId).Where("uid=?", uid).Exist(&models.FinancialObservation{})
	if err != nil {
		return err
	} else if !has {
		return errs.ErrReconciliationObservationNotFound
	}
	return nil
}

func requireAttempt(sess *xorm.Session, uid, observationId, attemptId int64) error {
	has, err := sess.ID(attemptId).Where("uid=? AND observation_id=?", uid, observationId).Exist(&models.ReconciliationAttempt{})
	if err != nil {
		return err
	} else if !has {
		return errs.ErrReconciliationRecordNotFound
	}
	return nil
}

func cloneJSON(value json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), value...)
}
