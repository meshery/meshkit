package channel

import (
	"fmt"
	"strings"
)

type ErrChannelBrokerPublishType struct {
	Err              error
	SuccessQueueList []string
	FailedQueueList  []string
}

func NewErrChannelBrokerPublish(
	err error,
	successQueueList []string,
	failedQueueList []string,
) *ErrChannelBrokerPublishType {
	return &ErrChannelBrokerPublishType{
		Err:              err,
		SuccessQueueList: successQueueList,
		FailedQueueList:  failedQueueList,
	}
}

func (e *ErrChannelBrokerPublishType) Error() string {
	return fmt.Sprintf(
		"%s, success queue list: [%s], failed queue list [%s]",
		e.Err.Error(),
		strings.Join(e.SuccessQueueList, ","),
		strings.Join(e.FailedQueueList, ","),
	)
}
