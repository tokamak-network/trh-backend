package utils

import (
	"context"
	"errors"
	"strings"
)

func IsContextCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled") || err == context.Canceled
}
