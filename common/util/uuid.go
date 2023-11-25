package util

import (
	"strings"

	"github.com/google/uuid"
)

func UUID() string {
	return strings.Replace(uuid.NewString(), "-", "", -1)
}
