package utils

import (
	"fmt"

	"github.com/google/uuid"
)

func NewUUID() (string, error) {
	id := uuid.New()
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x", id[0:4], id[4:6], id[6:8], id[8:10], id[10:])
	return uuid, nil
}
