package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// The pointer to the logfile, used for cleanup after the application is closed
var logFile *os.File

// The tasks to run
var tasks []*Task

// Allow users to input multiple copies of a single flag.
// Implements the Var interface from flags
type stringMultiFlag []string

func (f *stringMultiFlag) String() string {
	return "StringValue"
}

func (f *stringMultiFlag) Set(flagVal string) error {
	// Append with each value that's added
	*f = append(*f, flagVal)
	return nil
}

type durationMultiFlag []time.Duration

func (f *durationMultiFlag) String() string {
	return "StringValue"
}

func (f *durationMultiFlag) Set(flagVal string) error {
	// Attempt to parse the value
	parsedVal, err := parseDurationStr(flagVal)
	if err != nil {
		// Can't continue with invalid durations
		os.Exit(1)
	}
	// Append with each value that's added
	*f = append(*f, parsedVal)
	return nil
}

// Defines a task struct to allow running exclusive tasks on time
type Task struct {
	taskText        string
	isShellScript   bool
	timeBetweenRuns time.Duration
	mutex           *sync.Mutex
}

func init() {
	// Setup user input flags
	var taskList stringMultiFlag
	var durationList durationMultiFlag
	flag.Var(&taskList, "task", "A manually defined task to run. Can be a command or a path to a local script file (.sh only for now). Can be defined multiple times for many tasks")
	flag.Var(&taskList, "t", "A manually defined task to run. Can be a command or a path to a local script file (.sh only for now). Can be defined multiple times for many tasks")
	flag.Var(&durationList, "duration", "The duration to wait for each task to run (hourly, minutely etc). Needs to be defined at least once for each task")
	flag.Var(&durationList, "d", "The duration to wait for each task to run (hourly, minutely etc). Needs to be defined at least once for each task")
	logfilePath := flag.String("logs", "./task-scheduler.log", "Where to output application logs")
	taskFilePath := flag.String("file", "", "The location of a predefined task file, should have one task per line in the following format: \"/etc/path/to/my/script.sh 2h5m10s\" to run the designated script / task every 2hrs 5mins and 10 seconds")
	flag.Parse()

	if len(taskList) > len(durationList) {
		// Can't continue execution
		log.Fatal("Not all tasks were provided with durations. Every task needs a matching duration value to continue")
	}

	// Read tasks from the defined file if it was provided
	if *taskFilePath != "" {
		println("Reading tasks file")
		fileTasks, fileDurations := parseTasksFile(*taskFilePath)
		taskList = append(taskList, fileTasks...)
		durationList = append(durationList, fileDurations...)
	}

	// Create the task list
	for i := 0; i < len(taskList); i++ {
		taskCommand := taskList[i]

		thisTask := Task{
			taskText:        strings.Trim(taskCommand, "\""),
			isShellScript:   strings.HasSuffix(taskCommand, ".sh"),
			timeBetweenRuns: durationList[i],
			mutex:           &sync.Mutex{},
		}

		tasks = append(tasks, &thisTask)
	}

	// Setup logging
	setupLogFile(*logfilePath)
}

func main() {
	// Cleanup
	defer logFile.Close()

	if len(tasks) == 0 {
		// Can't run nothing
		log.Fatal("No tasks provided to the application")
	}

	println("Tasks parsed correctly, now running tasks on a schedule")

	// Skip the first one in the list as it'll be run forever on the main thread
	for i := 1; i < len(tasks); i++ {
		go scheduleTask(tasks[i])
	}

	// Run the first task on the main thread forever to keep the application alive
	scheduleTask(tasks[0])
}

// Run a task on a timer user a channel
func scheduleTask(task *Task) {

	thisTicker := time.NewTicker(task.timeBetweenRuns)

	for range thisTicker.C {
		// Run the task every tick from the channel (Every duration)
		go runTask(task)
	}
}

// Parses a tasks file and returns 2 slices with matching indexes, 1 with the tasks and 1 with the durations
func parseTasksFile(taskFilePath string) ([]string, []time.Duration) {
	file, err := os.Open(taskFilePath)

	if err != nil {
		// Log but don't stop the application, use any existing tasks instead
		log.Println(fmt.Sprintf("ERROR!: Failed to open taskfile at %s. Not running tasks defined in this file", taskFilePath))
		return []string{}, []time.Duration{}
	}

	fileScanner := bufio.NewScanner(file)

	var fileTasks []string
	var fileDurations []time.Duration

	for fileScanner.Scan() {
		task, duration, parseErr := parseTaskFileRow(fileScanner.Text())
		if parseErr == nil {
			// Only add to the list if no errors occurred, otherwise skip
			fileTasks = append(fileTasks, task)
			fileDurations = append(fileDurations, duration)
		}
	}

	if fileScanner.Err() != nil {
		log.Println(fmt.Sprintf("ERROR!: Failed to read the taskfile. %v", fileScanner.Err()))
	}

	return fileTasks, fileDurations
}

// Parses the row of a task file, handling any panics from reading by not returning that task
func parseTaskFileRow(fileRow string) (string, time.Duration, error) {
	// Handle panics from reading the duration
	splitTask := strings.Split(fileRow, " ")
	if len(splitTask) > 2 {
		// Invalid row, can't parse
		err := fmt.Errorf("ERROR!: Invalid row in a provided task file, can't parse %s", fileRow)
		return "", 0, err
	}

	task := splitTask[0]
	if duration, err := parseDurationStr(splitTask[1]); err == nil {
		return task, duration, nil
	} else {
		return "", 0, err
	}
}

// Parses a duration string and returns error if invalid or in the negatives (valid duration but not valid for application)
func parseDurationStr(durationText string) (time.Duration, error) {
	duration, err := time.ParseDuration(durationText)
	if err != nil {
		// Exit application early with warning
		log.Println(fmt.Sprintf("ERROR!: A duration was entered incorrectly: %v. Only units of (h,m,s,ms) are supported", err))
		return 0, err
	} else {
		// Block negative values
		if duration < 0 {
			negativeErr := fmt.Errorf("ERROR!: A duration had a negative value: %s. This application doesn't have the ability to time travel to the past to run tasks", durationText)
			log.Println(negativeErr)
			return 0, negativeErr
		}
	}
	return duration, nil
}

// Sets up the system logger to use the file specified
func setupLogFile(logPath string) {

	// Open the file as write only, don't care about reading that's for the user
	file, initialError := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 644)
	if initialError != nil {
		// Attempt to fallback to local logfile if possible
		if logPath == "./task-scheduler.log" {
			// Already using the default, can't continue
			log.Fatal(initialError)
		}
		// Not using the default, use fallback
		log.Println(initialError)
		log.Println("An error occurred attempting to use a custom log file, falling back to ./task-scheduler.log")
		defaultFile, err := os.OpenFile("./task-scheduler.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 644)

		if err != nil {
			// Can't even fall back to default, can't continue
			log.Fatal("err")
		}
		// Reassign file
		file = defaultFile
	}

	// Use as logging output
	logFile = file
	log.SetOutput(file)
}

// Runs a task that could either be a script or a commandline task.
// Ensures the task is only run once with a mutex lock
func runTask(task *Task) {
	defer task.mutex.Unlock()

	// Lock so no other equivalent task can run at the same time
	task.mutex.Lock()
	if task.isShellScript {
		runBashFile(task.taskText)
	} else {
		runCustomCommand(task.taskText)
	}
}

// Runs a command line task. Only allows one of the task to run at a time
func runCustomCommand(command string) {
	cmd := exec.Command(command)
	runAndLogTask(cmd, command)
}

// Runs a bash file. Only allows one of the scripts to execute at a time
func runBashFile(scriptPath string) {
	cmd := exec.Command("/usr/bin/bash", scriptPath)
	runAndLogTask(cmd, scriptPath)
}

// Runs and logs a predefined user task or script
func runAndLogTask(cmd *exec.Cmd, taskName string) {

	// Bind the output to a new buffer
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		// Task failed, print the failure to the logs and exit
		log.Println(fmt.Sprintf("ERROR!:  %v", err))
		return
	}

	// Succeeded, print the response in a human readable log format
	log.Println(fmt.Sprintf("%s - %s", taskName, out.String()))
}
