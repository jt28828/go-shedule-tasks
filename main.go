package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
)

// The pointer to the actual logfile, used for cleanup after the application is closed
var logFile *os.File

// The path of the application logfile
var logfilePath string

// The optional location of the crontab compatible file to read.
var cronFilePath string

// The manually defined tasks or script files to run
var tasks string

func init() {
	// Setup user input flags
	flag.StringVar(&tasks, "tasks", "", "The comma separated list of manually defined tasks to run. Can be a command or a path to a local script file (.sh only)")
	flag.StringVar(&tasks, "times", "", "The comma separated list of manually defined tasks to run. Can be a command or a path to a local script file (.sh only)")
	flag.StringVar(&cronFilePath, "file", "", "The location of a crontab compatible file")
	flag.StringVar(&logfilePath, "logs", "./task-scheduler.log", "Where to output application logs")
	flag.Parse()
	// Setup logging
	setupLogFile(logfilePath)
}

func main() {
	// Cleanup
	defer logFile.Close()
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
		log.Print(initialError)
		log.Print("An error occurred attempting to use a custom log file, falling back to ./task-scheduler.log")
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
		// Failed, Print an error log and panic
		log.Print(fmt.Sprintf("ERROR!: %v", err))

	}

	// Succeeded, print the response in a human readable log format
	log.Print(fmt.Sprintf("%s - %s", taskName, out.String()))
}
