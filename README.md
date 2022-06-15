# starter
A starter / supervisor application based on https://github.com/kontera-technologies/go-supervisor

The starter program starts and monitor the list of applications described in the configuration file. It can kill them and restart them when necessary.

Stopping the starter process should also stop all the monitored applications.

Usage
-----

    Usage of starter:
      -conf string
            configuration file (default "starter.conf")
      -debug
            log supervisor events
      -interrupts int
            number of attempts to interrupt a process before killing it (default 10)
      -max-spawns int
            max spawns limit per process (default 10)
      -print-conf
            pretty-print configuration file and exit
      -respawns int
            number of attempts to start a process (default 10)
      -version
            print version and exit

The format of the configuration file is the following:

    # global options (same as equivalent command line options)
    debug      = true
    interrupts = 10
    max-spawn  = 10
    respawns   = 10

    # environment variables
    [env]
    name = "value"
    name = "${ENV:-default}"

    # per application options (you can have multiple [[applications]] sections, one per applications)
    [[applications]]
    id = "example-1"                # string - the identifier for this application (default "app-{number}")
    program = "example.bash"        # string - application path or name (required)
    args = ["hello"]                # [string,...] - a list of application arguments
    stdout-idle = 5                 # int (secs) - application is restarted if there are no writes for this amount time
                                    #   if 0, stdout is not monitored
    stderr-idle = 5                 # int (secs) - application is restarted if there are no writes for this amount time
                                    #   if 0, stderr is not monitored
    min-wait = 10                   # int (secs) - minimum amount of time to wait before restarting the application

Environment variables are passed to the program but can also be expanded in `program`, `args`, `dir` and `color`.

It is also possible to execute a series of tasks in steps (worflow) by setting the global option "workflow = true"
and adding a "next" taskid to each tasks:

    # global options
    workflow = true

    # sequence of tasks
    [[application]]
    id = "task1"
    ...
    next = "task2"

    [[application]]
    id = "task2"
    ...
    next = "task3"

    [[application]]
    id = "task3"
    ...
    # no next here

In this case tasks are executed sequentially, starting from the first task in the list or from the task specified on the command line.
When a task terminates, the next task is started (following the "next" field). If a task has no "next" field, the workflow terminates.

It should be actually possible to start multiple workflows by specifying multiple start tasks on the command line. They will start in parallel,
then each one will move to the next step untill all worflows are completed.
