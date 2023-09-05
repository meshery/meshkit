package events

import (
	"fmt"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

func (e *Event) BeforeCreate(tx *gorm.DB) (err error) {
	e.ID, _ = uuid.NewV4()
	err = isEventStatusSupported(e)
	return
}

func (e *Event) BeforeUpdate(tc *gorm.DB) (err error) {
	err = isEventStatusSupported(e)
	return
}

func isEventStatusSupported(event *Event) error {

	if event.Status != Read && event.Status != Unread {
		return fmt.Errorf("event status %s is not supported", event.Status)
	}
	return nil
}