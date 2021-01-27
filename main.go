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

func init() {
	// Setup user input flags
	taskListString := flag.String("tasks", "", "The comma separated list of manually defined tasks to run. Can be a command or a path to a local script file (.sh only)")
	durationStr := flag.String("durations", "", "The comma separated list of durations to wait between running tasks. Durations should be in the same order as their associated tasks")
	logfilePath := flag.String("logs", "./task-scheduler.log", "Where to output application logs")
	taskFilePath := flag.String("file", "", "The location of a predefined task file, should have one task per line in the following format: \"/etc/path/to/my/script.sh 2h5m10s\" to run the designated script / task every 2hrs 5mins and 10 seconds")
	flag.Parse()

	// Read tasks from the manual flags first
	taskList := strings.Split(*taskListString, ",")
	durationList := readDurationListStr(*durationStr)

	// Read tasks from the defined file if it was provided
	if *taskFilePath != "" {
		println("Reading tasks file")
		fileTasks, fileDurations := parseTasksFile(*taskFilePath)
		taskList = append(taskList, fileTasks...)
		durationList = append(durationList, fileDurations...)
	}

	// Setup the task list
	for i := 0; i < len(taskList); i++ {
		taskCommand := taskList[i]

		thisTask := Task{
			taskText:        taskCommand,
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
	// TODO rework once panic logic different
	duration := parseDurationStr(splitTask[1])

	return task, duration, nil
}

// Reads an array of duration strings and converts them into a slice of durations
func readDurationListStr(durationStr string) []time.Duration {
	durations := strings.Split(durationStr, ",")

	var durationSlice []time.Duration

	for _, durationText := range durations {
		// Append the parsed duration to the slice
		durationSlice = append(durationSlice, parseDurationStr(durationText))
	}

	return durationSlice
}

// Parses a duration string and panicsif it is invalid as can't support running tasks without valid duration between runs
// TODO make not panic so can be reused
func parseDurationStr(durationText string) time.Duration {
	duration, err := time.ParseDuration(durationText)
	if err != nil {
		// Exit application early with warning
		log.Panicf("ERROR!: A duration was entered incorrectly: %v", err)
	} else {
		// Block negative values
		if duration < 0 {
			log.Panicf("ERROR!: A duration had a negative value: %s. This application doesn't have the ability to time travel to the past to run tasks", durationText)
		}
	}
	return duration
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

// Defines a task struct to allow running exclusive tasks on time
type Task struct {
	taskText        string
	isShellScript   bool
	timeBetweenRuns time.Duration
	mutex           *sync.Mutex
}
