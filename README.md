# Go Task Scheduler

This is a task scheduler written in go to mimic some basic features of Cron

## Running the application

Just run the build script and then run the output binary.

## Flags

This tool uses flags to read configuration data from users. These include:

- `--task` or `-t` A manually defined task to run. Can be a command or a path to a local script file (.sh only for now).
  Can be passed multiple times for many tasks.


- `--duration` or `-d` How often a task should run (hourly, minutely etc). Needs to be defined at least once for each
  task.


- `--logs` A filepath to where the tool should output logs. Defaults to outputting in the current folder.


- `--file` The location of a predefined task file, should have one task per line. Tasks need to be wrapped in backticks separate from their duration value

## Sample Usage

### Print the date every 70 seconds and log to a custom log file

```
./build
./dist/task-schduler --task date --duration 1m10s --logs ./logs/my-logs.log
```

### Print the date every 70 seconds and also ping an address 3 times every 2 minutes

```
./build
./dist/task-schduler --task date --duration 1m10s --task "ping github.com -c3" --duration 2m
```

### Run a pre-written file full of ping tasks at different intervals. Tasks in a file must be wrapped in a backtick

Task File `tasks/ping_tasks.txt`:

```
`ping -c 1 github.com`  1h15min20s
`ping -c 1 google.com`  3h5min12s
`ping -c 1 golang.org`  24h1min2s
```

```
./build
./dist/task-schduler --file tasks/ping_tasks.txt
```