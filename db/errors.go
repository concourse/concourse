package db

import "errors"

var ErrNoVersions = errors.New("no versions found")
var ErrNoBuild = errors.New("no build found")

var ErrLockRowNotPresentOrAlreadyDeleted = errors.New("lock could not be acquired because it didn't exist or was already cleaned up")

var ErrLockNotAvailable = errors.New("lock is currently held and cannot be immediately acquired")
