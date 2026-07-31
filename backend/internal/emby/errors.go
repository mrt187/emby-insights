package emby

import "errors"

var (
	ErrInvalidCredentials      = errors.New("invalid Emby credentials")
	ErrPrimaryImageUnavailable = errors.New("Emby user has no primary image")
	ErrItemImageUnavailable    = errors.New("Emby item has no image")
)
