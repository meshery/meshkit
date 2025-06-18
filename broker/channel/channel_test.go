package channel

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/stretchr/testify/assert"
)

// Create Channel Broker Handler
// numOfQueues subscribers to one subject different queue names
// 1 publisher
// numOfMessages messages
// assert that each suscriber receives numOfMessages messages
func TestPubSubModel(t *testing.T) {
	br := NewChannelBrokerHandler()
	defer br.CloseConnection()

	numOfQueues := 8
	numOfMessages := 64
	queueFormat := "queue-%02d"
	subject := "important_subject"

	in := make(chan *broker.Message)
	out := make([]chan *broker.Message, 0, numOfQueues)
	expectedConnectedEndpoints := make([]string, 0, numOfQueues)

	// subscribse same subject numOfQueues queues
	for i := range make([]struct{}, numOfQueues) {
		out = append(out, make(chan *broker.Message))
		errSubscribeWithChannel := br.SubscribeWithChannel(
			subject,
			fmt.Sprintf(queueFormat, i),
			out[i],
		)
		assert.NoError(t, errSubscribeWithChannel, "br.SubscribeWithChannel must not end with error")
		if errSubscribeWithChannel != nil {
			t.FailNow()
		}
		expectedConnectedEndpoints = append(
			expectedConnectedEndpoints,
			fmt.Sprintf(
				"%s::%s",
				subject,
				fmt.Sprintf(queueFormat, i),
			),
		)
	}

	errPublishWithChannel := br.PublishWithChannel(subject, in)
	assert.NoError(t, errPublishWithChannel, "br.PublishWithChannel must not end with error")
	if errPublishWithChannel != nil {
		t.FailNow()
	}

	var wg sync.WaitGroup
	wg.Add(1) // one for subscribers
	wg.Add(1) // one for publisher

	// publish in "in"
	go func(ch chan<- *broker.Message) {
		defer wg.Done()

		for i := range make([]struct{}, numOfMessages) {
			ch <- &broker.Message{
				Object: i,
			}
		}
	}(in)

	// read from out[i]
	// assert that each queue receive same amount of messages
	go func() {
		defer wg.Done()

		var wgSub sync.WaitGroup
		wgSub.Add(numOfQueues)
		for i := range make([]struct{}, numOfQueues) {
			go func(ch <-chan *broker.Message, index int) {
				defer wgSub.Done()

				count := 0
			loop:
				for {
					select {
					case <-ch:
						count++
					case <-time.After(1 * time.Second):
						break loop
					}
				}
				assert.Equalf(
					t,
					count,
					numOfMessages,
					"must receive exact amount of messages from broker for queue %d",
					index,
				)
			}(out[i], i)
		}
		wgSub.Wait()
	}()

	wg.Wait()

	assert.ElementsMatch(t, expectedConnectedEndpoints, br.ConnectedEndpoints(), "br.ConnectedEndpoints() must return list of connected endpoints in format subject::queue")
	assert.NotEmpty(t, br.Info(), "br.Info() nust not return empty string")
}
