package workflow

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"sync"
	"time"
)

// Task represents a single task in the workflow
type Task struct {
	Name      string   `yaml:"task"`
	Type      string   `yaml:"type"`
	Command   string   `yaml:"command"`
	DependsOn []string `yaml:"depends_on,omitempty"`
}

// Workflow represents a collection of tasks
type Workflow struct {
	Tasks []Task `yaml:"workflow"`
}

// ExecuteWorkflow loads and runs the tasks from the YAML workflow file
func ExecuteWorkflow(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read workflow file: %v", err)
	}

	var wf Workflow
	err = yaml.Unmarshal(data, &wf)
	if err != nil {
		return fmt.Errorf("failed to parse workflow: %v", err)
	}

	runTasks(wf.Tasks)
	return nil
}

// findTaskByName finds a task by its name
func findTaskByName(tasks []Task, name string) *Task {
	for _, task := range tasks {
		if task.Name == name {
			return &task
		}
	}
	return nil
}

// runTasks executes tasks in parallel, respecting dependencies
func runTasks(tasks []Task) {
	var wg sync.WaitGroup
	taskStatus := make(map[string]bool)

	// Initialize the taskStatus map to track task completion
	for _, task := range tasks {
		taskStatus[task.Name] = false
	}

	var mu sync.Mutex // Mutex to safely update taskStatus

	for _, task := range tasks {
		wg.Add(1)

		go func(t Task) {
			defer wg.Done()

			// Wait for all dependencies to complete
			for _, dep := range t.DependsOn {
				// Polling until the dependency is marked as completed
				for {
					mu.Lock()
					if taskStatus[dep] {
						mu.Unlock()
						break
					}
					mu.Unlock()
					time.Sleep(100 * time.Millisecond) // Sleep for a short time before retrying
				}
			}

			// Simulate task execution
			fmt.Printf("Running task: %s\n", t.Name)
			time.Sleep(2 * time.Second) // Simulate task running
			fmt.Printf("Task %s completed\n", t.Name)

			// Mark task as complete
			mu.Lock()
			taskStatus[t.Name] = true
			mu.Unlock()
		}(task)
	}

	wg.Wait()
	fmt.Println("All tasks completed.")
}
