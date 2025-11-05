package fleet

type FleetScheduler interface {
	ScheduleTask() bool
	BroadcastTask() bool
}

// TODO: queue of running tasks,
// queue of todo tasks
// broadcast received device event
// receive a device event
// turn on device
// turn off device
//
