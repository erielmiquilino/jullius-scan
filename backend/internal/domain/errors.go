package domain

import "errors"

var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("resource not found")

	// ErrAccessDenied indicates the user does not have access to the resource.
	ErrAccessDenied = errors.New("access denied")

	// ErrNoHouseMembership indicates the user has no active House membership.
	ErrNoHouseMembership = errors.New("no house membership found for user")

	// ErrDuplicateJob indicates a scraping job already exists for this fiscal URL in the House.
	ErrDuplicateJob = errors.New("scraping job already exists for this fiscal URL in this house")

	// ErrInvalidInput indicates malformed or missing request data.
	ErrInvalidInput = errors.New("invalid input")
)
