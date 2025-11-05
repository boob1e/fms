//	type FleetDevice interface {
//		SendTask() bool
//		ReceiveTask() bool
//		AcceptTask() bool
//		IsBusy() bool
//		RunLatestQueuedTask() bool
//		CancelRunningTask() bool
//	}
package fleet

import (
	"context"
	"sync"
)

// these methods implmentations are never going to change.
// use inheritance for the messaging stuff.
type FleetDevice struct {
	broker *MessageBroker
	inbox  chan Task
	wg     sync.WaitGroup
}

func (f *FleetDevice) listen(ctx context.Context) {
	defer f.wg.Done()
	for {
		select {
		case task := <-f.inbox:
			f.handleTask(task)
		case <-ctx.Done():
			return //exit
		}
	}
}

func (f *FleetDevice) handleTask(task Task) {

}

func (f *FleetDevice) Shutdown() {
	f.broker.Unsubscribe("irrigation-zone", f.inbox)
	f.wg.Wait()
}
