package workflow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Task struct {
	Name      string   `yaml:"task"`
	Type      string   `yaml:"type"`       // "shell", "db", "http"
	Command   string   `yaml:"command"`    // Shell command or SQL query
	URL       string   `yaml:"url"`        // HTTP URL
	Method    string   `yaml:"method"`     // HTTP method (GET, POST)
	Body      string   `yaml:"body"`       // HTTP request body
	DependsOn []string `yaml:"depends_on"` // Dependencies
	Timeout   string   `yaml:"timeout"`    // Timeout for task
	Schedule  string   `yaml:"schedule"`   // Scheduled hr:mm
	Retries   int      `yaml:"retries"`    // Number of retries
	DbConn    string   `yaml:"db_conn"`    // DB connection string
}

type Workflow struct {
	Tasks []Task `yaml:"workflow"`
}

func ExecuteWorkflow(filename string) error {
	data, err := os.ReadFile(filename)
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

// runTasks executes tasks in parallel, respecting dependencies
func runTasks(tasks []Task) {
	var wg sync.WaitGroup
	taskStatus := make(map[string]bool)
	var mu sync.Mutex

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
				for {
					mu.Lock()
					if taskStatus[dep] {
						mu.Unlock()
						break
					}
					mu.Unlock()
					time.Sleep(100 * time.Millisecond)
				}
			}

			// Execute task with retries and timeout
			success := executeWithRetries(t)

			if success {
				mu.Lock()
				taskStatus[t.Name] = true
				mu.Unlock()
			} else {
				fmt.Printf("Task %s failed after retries\n", t.Name)
			}
		}(task)
	}

	wg.Wait()
	fmt.Println("All tasks completed.")
}

func executeWithRetries(task Task) bool {
	retryCount := 0
	for {
		if retryCount > task.Retries {
			fmt.Printf("Task %s exceeded retry limit of %d\n", task.Name, task.Retries)
			return false
		}

		// Execute the task based on its type
		err := executeTask(task)
		if err == nil {
			fmt.Printf("Task %s completed successfully\n", task.Name)
			return true
		}

		fmt.Printf("Task %s failed: %v. Retrying...\n", task.Name, err)
		retryCount++
		time.Sleep(time.Duration(2^retryCount) * time.Second)
	}
}

func executeTask(task Task) error {
	switch task.Type {
	case "shell":
		return executeShellTask(task)
	case "db":
		return executeDBTask(task)
	case "http":
		return executeHTTPTask(task)
	default:
		return fmt.Errorf("unsupported task type: %s", task.Type)
	}
}

func executeShellTask(task Task) error {
	ctx, cancel := context.WithTimeout(context.Background(), parseTimeout(task.Timeout))
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", task.Command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", task.Command)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error executing task %s: %v, Output: %s\n", task.Name, err, string(output))
		return err
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		fmt.Printf("Task %s timed out\n", task.Name)
		return ctx.Err()
	}

	fmt.Printf("Task %s output: %s\n", task.Name, string(output))
	return nil
}

func executeDBTask(task Task) error {
	db, err := sql.Open("postgres", task.DbConn)
	if err != nil {
		return fmt.Errorf("failed to connect to the database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(task.Command) // Running the SQL query (e.g., INSERT, UPDATE)
	if err != nil {
		return fmt.Errorf("failed to execute query: %v", err)
	}

	fmt.Printf("Task %s: query executed successfully\n", task.Name)
	return nil
}

func executeHTTPTask(task Task) error {
	client := &http.Client{Timeout: parseTimeout(task.Timeout)}
	req, err := http.NewRequest(task.Method, task.URL, strings.NewReader(task.Body))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Task %s: HTTP request to %s completed with status %d\n", task.Name, task.URL, resp.StatusCode)
	return nil
}

func scheduleTask(task Task, execute func()) {
	scheduleTime, err := time.Parse("15:04", task.Schedule)
	if err != nil {
		fmt.Printf("Failed to parse schedule time: %v\n", err)
	}
	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), scheduleTime.Hour(), scheduleTime.Minute(), 0, 0, now.Location())
	if nextRun.Before(now) {
		nextRun = nextRun.Add(24 * time.Hour)
	}
	time.AfterFunc(nextRun.Sub(now), execute)
}

func parseTimeout(timeout string) time.Duration {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return 30 * time.Second // Default to 30s if no valid timeout provided
	}
	return duration
}
