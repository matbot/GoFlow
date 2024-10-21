package workflow

import (
	"context"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// Task represents a single task in the workflow
type Task struct {
	Name      string   `yaml:"task"`
	Type      string   `yaml:"type"`
	Command   string   `yaml:"command"`
	DependsOn []string `yaml:"depends_on,omitempty"`
	Timeout   string   `yaml:"timeout,omitempty"`
	Retries   int      `yaml:"retries,omitempty"`
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

func executeWithRetries(task Task) bool {
	retryCount := 0
	for {
		if retryCount > task.Retries {
			fmt.Printf("Task %s exceeded retry limit %d.\n", task.Name, task.Retries)
			return false
		}

		err := executeTask(task)
		if err == nil {
			fmt.Printf("Task %s executed successfully.\n", task.Name)
			return true
		}

		fmt.Printf("Task %s failed: %v. Retrying...\n", task.Name, err)
		retryCount++
		// exponential backoff
		time.Sleep(time.Duration(2^retryCount) * time.Second)
	}
}

func executeTask(task Task) error {
	timeoutDuration, err := time.ParseDuration(task.Timeout)
	if err != nil {
		timeoutDuration = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", task.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", task.Command)
	}

	//cmd := exec.CommandContext(ctx, "sh", "-c", task.Command)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error executing task %s: %v. Output: %s.\n", task.Name, err, string(output))
		return err
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		fmt.Printf("Task %s exceeded timeout %s.\n", task.Name, task.Timeout)
		return ctx.Err()
	}

	fmt.Printf("Task %s executed successfully: %s.\n", task.Name, string(output))
	return nil
}

// runTasks executes tasks in parallel, respecting dependencies
func runTasks(tasks []Task) {
	var wg sync.WaitGroup
	taskStatus := make(map[string]bool)
	var mu sync.Mutex // Mutex to safely update taskStatus

	// Initialize the taskStatus map to track task completion
	for _, task := range tasks {
		taskStatus[task.Name] = false
	}

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
			//fmt.Printf("Running task: %s\n", t.Name)
			//time.Sleep(2 * time.Second) // Simulate task running
			//fmt.Printf("Task %s completed\n", t.Name)

			// Mark task as complete if it succeeds
			success := executeWithRetries(t)
			if success {
				mu.Lock()
				taskStatus[t.Name] = true
				mu.Unlock()
			} else {
				fmt.Printf("Task %s failed after %i retries.\n", t.Name, t.Retries)
			}

		}(task)
	}

	wg.Wait()
	fmt.Println("All tasks completed.")
}
