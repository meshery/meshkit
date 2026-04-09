package events

import (
	"fmt"

	core "github.com/meshery/schemas/models/core"
)

func DesignDownloadEvent(designID core.Uuid, designName string, userID core.Uuid, systemID core.Uuid) *Event {

	event := NewEvent().ActedUpon(designID).FromSystem(systemID).FromUser(userID).WithCategory("pattern").WithAction("download").
		WithSeverity(Informational).WithDescription(fmt.Sprintf("Downloaded \"%s\" design", designName)).Build()
	return event
}

func DesignViewEvent(designID core.Uuid, designName string, userID core.Uuid, systemID core.Uuid) *Event {

	event := NewEvent().ActedUpon(designID).FromSystem(systemID).FromUser(userID).WithCategory("pattern").WithAction("view").WithSeverity(Informational).WithDescription(fmt.Sprintf("Accessed \"%s\" pattern.", designName)).Build()
	return event
}
