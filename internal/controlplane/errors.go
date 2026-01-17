package controlplane

import "errors"

// Sentinel errors for control plane operations.
var (
	ErrAlreadyClaimed  = errors.New("task already claimed")
	ErrNoLease         = errors.New("no active lease")
	ErrNotOwner        = errors.New("not the lease owner")
	ErrNotFound        = errors.New("resource not found")
	ErrTaskNotFound    = errors.New("task not found")
	ErrResourceLocked  = errors.New("resource already locked")
	ErrLockNotHeld     = errors.New("lock not held by this holder")
)
