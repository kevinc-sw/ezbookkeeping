package errs

import "net/http"

var (
	ErrReconciliationObservationInvalid  = NewNormalError(NormalSubcategoryReconciliation, 0, http.StatusBadRequest, "reconciliation observation is invalid")
	ErrReconciliationObservationNotFound = NewNormalError(NormalSubcategoryReconciliation, 1, http.StatusNotFound, "reconciliation observation not found")
	ErrReconciliationSnapshotInvalid     = NewNormalError(NormalSubcategoryReconciliation, 2, http.StatusBadRequest, "reconciliation snapshot is invalid")
	ErrReconciliationRecordInvalid       = NewNormalError(NormalSubcategoryReconciliation, 3, http.StatusBadRequest, "reconciliation record is invalid")
	ErrReconciliationRecordNotFound      = NewNormalError(NormalSubcategoryReconciliation, 4, http.StatusNotFound, "reconciliation record not found")
)
