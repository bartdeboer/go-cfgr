package cfgr

import (
	"errors"

	"github.com/bartdeboer/go-cfgr/storage"
)

var (
	ErrNotFound    = storage.ErrNotFound
	ErrUnavailable = errors.New("cfgr: storage capability unavailable")
	ErrDenied      = errors.New("cfgr: capability denied")
)
