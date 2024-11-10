package main

import (
	"fmt"
	"sync"
	"time"
)

// Task struct represents a unit of work to be executed in the event loop.
// It contains the main task function and an optional callback function for post-task actions.
type Task struct {
	MainTask  func()  // The primary task function to execute
	Callback  func()  // An optional callback function to execute after the main task
	IsBlocking bool   // Flag indicating if the task is blocking (needs to be handled by a worker)
}

// EventLoop struct represents the main event loop that manages tasks.
// It includes channels for incoming tasks, tasks that require callbacks, 
// and a stop signal to terminate the loop.
type EventLoop struct {
	mainTasks chan Task  // Channel for tasks that need to be processed immediately
	taskQueue chan Task  // Channel for callback tasks that need to be processed later
	stop      chan bool  // Channel to stop the event loop
}

// Add adds a new task to the mainTasks channel for immediate processing.
func Add(eventLoop *EventLoop, task *Task) {
	eventLoop.mainTasks <- *task // Push task to the mainTasks channel for execution
}

// AddToTaskQueue adds a new blocking task to the taskQueue for later processing.
func AddToTaskQueue(eventLoop *EventLoop, task *Task) {
	eventLoop.taskQueue <- *task // Push the task to the taskQueue
}

// StopEventLoop sends a stop signal to the event loop, instructing it to terminate.
func StopEventLoop(eventLoop *EventLoop) {
	eventLoop.stop <- true // Send a signal to stop the event loop
}

// InitEventLoop initializes and starts the event loop in a separate goroutine.
// The loop processes tasks from the mainTasks channel, manages blocking tasks with a worker pool,
// and executes callback tasks from the taskQueue.
func InitEventLoop(eventLoop *EventLoop, workerPoolSize int) *sync.WaitGroup {
	var wg sync.WaitGroup

	// Add the event loop goroutine to the wait group to track its execution
	wg.Add(1)

	// Worker pool for handling blocking tasks
	workerPool := make(chan struct{}, workerPoolSize)

	// Start the event loop in a separate goroutine
	go func() {
		defer wg.Done() // Ensure the wait group is decremented when the goroutine finishes

		// Main event loop: processes tasks continuously
		for {
			// Wait for tasks from the mainTasks or taskQueue channels
			select {
			case task := <-eventLoop.mainTasks: 
				if task.IsBlocking {
					// If the task is blocking, it will be handled by a worker from the pool
					workerPool <- struct{}{} // Acquire a worker from the pool (wait if none available)

					// Execute the blocking task in a separate goroutine
					go func() {
						defer func() {
							<-workerPool // Release the worker back to the pool once task is completed
						}()

						// Run the main task
						task.MainTask()

						// If a callback is provided, add it to the taskQueue
						if task.Callback != nil {
							AddToTaskQueue(eventLoop, &Task{
								MainTask: task.Callback,  // The callback will be executed after the main task completes
							})
						}
					}()
				} else {
					// For non-blocking tasks, execute immediately in the event loop
					task.MainTask()
				}

			// Process callback tasks that were deferred during blocking tasks
			case task := <-eventLoop.taskQueue: 
				// Execute the callback task
				task.MainTask()

			// Handle the stop signal to terminate the event loop
			case stop := <-eventLoop.stop:
				if stop {
					return // Exit the loop and terminate the event loop
				}
			}
		}
	}()

	// Return the wait group to allow tracking of goroutine completion
	return &wg
}

func main() {
	// Initialize the event loop with buffered channels for tasks and stop signal
	eventLoop := EventLoop{
		mainTasks: make(chan Task, 10), // Channel for immediate tasks (buffered to hold 10 tasks)
		taskQueue: make(chan Task, 10),  // Channel for deferred callback tasks (buffered for 10 tasks)
		stop:      make(chan bool),      // Channel to signal the event loop to stop
	}

	// Initialize the event loop with a worker pool size of 10
	wg := InitEventLoop(&eventLoop, 10)

	// Add non-blocking tasks to the event loop
	Add(&eventLoop, &Task{
		MainTask: func() {
			fmt.Println("Non-blocking task 1 executed.")
		},
		IsBlocking: false,
	})

	Add(&eventLoop, &Task{
		MainTask: func() {
			fmt.Println("Non-blocking task 2 executed.")
		},
		IsBlocking: false,
	})

	// Add blocking tasks with callbacks
	Add(&eventLoop, &Task{
		MainTask: func() {
			time.Sleep(2 * time.Second) // Simulate a blocking task
			fmt.Println("Blocking task 3 executed.")
		},
		Callback: func() {
			fmt.Println("Callback for blocking task 3 executed.")
		},
		IsBlocking: true,
	})

	Add(&eventLoop, &Task{
		MainTask: func() {
			time.Sleep(4 * time.Second) // Simulate a long blocking task
			fmt.Println("Blocking task 4 executed.")
		},
		Callback: func() {
			fmt.Println("Callback for blocking task 4 executed.")
		},
		IsBlocking: true,
	})

	// Add another non-blocking task
	Add(&eventLoop, &Task{
		MainTask: func() {
			fmt.Println("Non-blocking task 5 executed.")
		},
		IsBlocking: false,
	})

	// Allow enough time for all tasks to complete
	time.Sleep(10 * time.Second)

	// Stop the event loop after all tasks are processed
	StopEventLoop(&eventLoop)

	// Wait for the event loop to finish processing all tasks before exiting
	wg.Wait()
}
