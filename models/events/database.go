package events

import (
	"fmt"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

// BeforeCreate keeps the ID the producer minted (NewEvent mints one) and
// assigns one only when the event was constructed without the builder.
func (e *Event) BeforeCreate(tx *gorm.DB) (err error) {
	if e.ID == uuid.Nil {
		e.ID, err = uuid.NewV4()
		if err != nil {
			return
		}
	}
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
