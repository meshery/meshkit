package events

import (
	"fmt"

	"github.com/gofrs/uuid"
)

func DesignDownloadEvent(designID uuid.UUID, designName string, userID uuid.UUID, systemID uuid.UUID) *Event {

	event := NewEvent().ActedUpon(designID).FromSystem(systemID).FromUser(userID).WithCategory("pattern").WithAction("download").
		WithSeverity(Informational).WithDescription(fmt.Sprintf("Downloaded \"%s\" design", designName)).Build()
	return event
}

func DesignViewEvent(designID uuid.UUID, designName string, userID uuid.UUID, systemID uuid.UUID) *Event {

	event := NewEvent().ActedUpon(designID).FromSystem(systemID).FromUser(userID).WithCategory("pattern").WithAction("view").WithSeverity(Informational).WithDescription(fmt.Sprintf("Accessed \"%s\" pattern.", designName)).Build()
	return event
}
