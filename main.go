package main

import (
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

// The pointer to the actual logfile, used for cleanup after the application is closed
var logFile *os.File

// The tasks or script files to run
var tasks []string

// The duration between each run of a task (same index as the associated task)
var durations []time.Duration

func init() {
	// Setup user input flags
	taskStr := flag.String("tasks", "", "The comma separated list of manually defined tasks to run. Can be a command or a path to a local script file (.sh only)")
	durationStr := flag.String("durations", "", "The comma separated list of durations to wait between running tasks. Durations should be in the same order as their associated tasks")
	logfilePath := flag.String("logs", "./task-scheduler.log", "Where to output application logs")
	taskFilePath := flag.String("file", "", "The location of a predefined task file, should have one task per line in the following format: \"/etc/path/to/my/script.sh 2h5m10s\" to run the designated script / task every 2hrs 5mins and 10 seconds")
	flag.Parse()

	// Read tasks from the manual flags first
	taskList := strings.Split(*taskStr, ",")
	durationList := readDurationStr(*durationStr)

	// Setup logging
	setupLogFile(*logfilePath)
}

func main() {
	// Cleanup
	defer logFile.Close()
}

// Reads an array of duration strings and converts them into a slice of durations
func readDurationStr(durationStr string) []time.Duration {
	durations := strings.Split(durationStr, ",")

	var durationSlice []time.Duration
	for _, durationText := range durations {
		if duration, err := time.ParseDuration(durationText); err != nil {
			// Exit application early with warning
			log.Println(fmt.Sprintf("ERROR!: A duration was entered incorrectly: %s. Duration needs to be in the format: 1h1m1s1ms", durationText))
		}
	}
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

// Runs a command line task. Only allows one of the task to run at a time
func runCustomCommand(command string, mutex *sync.Mutex) {
	defer mutex.Unlock()

	// Run the task exclusively
	mutex.Lock()
	cmd := exec.Command(command)
	runAndLogTask(cmd, command)
}

// Runs a bash file. Only allows one of the scripts to execute at a time
func runBashFile(scriptPath string, mutex *sync.Mutex) {
	defer mutex.Unlock()
	// Execute the file exclusively
	mutex.Lock()
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
	}

	// Succeeded, print the response in a human readable log format
	log.Println(fmt.Sprintf("%s - %s", taskName, out.String()))
}
