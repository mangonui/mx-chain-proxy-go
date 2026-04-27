package process

import "runtime/debug"

func runGuardedBackgroundTask(taskName string, fn func()) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("background task panic recovered", "task", taskName, "panic", recovered, "stack", string(debug.Stack()))
			}
		}()

		fn()
	}()
}
