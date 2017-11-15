package main

import (
	"flag"
	"fmt"
	"io"
	"log"
        "os"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/gobs/pretty"
	"github.com/kontera-technologies/go-supervisor/supervisor"
	"github.com/mgutz/ansi"
)

const (
	VERSION = "0.1"
)

type Application struct {
	Id      string   `toml:"id"`       // program id
	Program string   `toml:"program"`  // executable to run
	Args    []string `toml:"args"`     // arguments
	Dir     string   `toml:"dir"`      // working directory
	MinWait int      `toml:"min-wait"` // minimum wait time before restarting the process
	Color   string   `toml:"color"`    // color for log messages

	StdoutIdle int `toml:"stdout-idle"` // stdout idle time, before stopping
	StderrIdle int `toml:"stderr-idle"` // stderr idle time, before stopping
}

type Config struct {
	Respawns   int  `toml:"respawns"`   // number of attempts to start a process
	Interrupts int  `toml:"interrupts"` // number of attempts to interrupt the process before killing it
	MaxSpawns  int  `toml:"max-spawns"` // max spawns limit
	Debug      bool `toml:"debug"`      // log supervisor events

	Applications []Application `toml:"application"` // list of applications to start and monitor

	Environment map[string]string `toml:"env"` // environment variables
}

func getConfig() *Config {
	var config Config

	cfile := flag.String("conf", "starter.conf", "configuration file")
	version := flag.Bool("version", false, "print version and exit")
	printConf := flag.Bool("print-conf", false, "pretty-print configuration file and exit")

	flag.BoolVar(&config.Debug, "debug", false, "log supervisor events")
	flag.IntVar(&config.Respawns, "respawns", 10, "number of attempts to start a process")
	flag.IntVar(&config.Interrupts, "interrupts", 10, "number of attempts to interrupt a process before killing it")
	flag.IntVar(&config.MaxSpawns, "max-spawns", 10, "max spawns limit per process")

	flag.Parse()

	if *version {
		fmt.Println("starter version", VERSION)
		return nil
	}

	if _, err := toml.DecodeFile(*cfile, &config); err != nil {
		log.Fatal(err)
	}

	if *printConf {
		pretty.PrettyPrint(config)
		return nil
	}

	return &config
}

type colorWriter struct {
	colorize func(string) string
}

func ColorWriter(c string) io.Writer {
	return &colorWriter{ansi.ColorFunc(c)}
}

func (w *colorWriter) Write(b []byte) (int, error) {
	s := strings.TrimRight(string(b), "\r\n")
	return fmt.Println(w.colorize(s))
}

func main() {
	config := getConfig()
	if config == nil {
		return
	}

	if len(config.Applications) == 0 {
		log.Fatal("no applications to run")
	}

        for k, v := range config.Environment {
            os.Setenv(k, v)
        }

	processes := map[string]*supervisor.Process{}
	colors := map[string]string{}

	for i, app := range config.Applications {
		if app.Id == "" {
			app.Id = fmt.Sprintf("app-%v", i)
		}

		if app.Color == "" {
			app.Color = "off"
		}

		p, err := supervisor.Supervise(app.Program, supervisor.Options{
			Args:                    app.Args,          // argumets to pass ( default is none )
			SpawnAttempts:           config.Respawns,   // attempts before giving up ( default 10 )
			AttemptsBeforeTerminate: config.Interrupts, // on Stop() terminate process after X interrupt attempts (default is 10)
			Dir:            app.Dir,          // run dir ( default is current dir )
			Id:             app.Id,           // will be added to every log print ( default is "NOID")
			MaxSpawns:      config.MaxSpawns, // Max spawn limit ( default is 1 )
			StdoutIdleTime: app.StdoutIdle,   // stop worker if we didn't recived stdout message in X seconds ( default is 0 - disbaled )
			StderrIdleTime: app.StderrIdle,   // stop worker if we didn't recived stderr message in X seconds ( default is 0 - disbaled )

			// function that calculate sleep time based in the current sleep time
			// useful for exponential backoff ( default is this function )
			DelayBetweenSpawns: func(currentSleep int) (sleepTime int) {
				if app.MinWait > 0 {
					return app.MinWait
				} else {
					return currentSleep * 2
				}
			},
		})

		if err != nil {
			log.Printf("failed to start process: %s", err)
		}

		processes[app.Id] = p
		colors[app.Id] = app.Color
	}

	var wg sync.WaitGroup

	for curpid, curp := range processes {
		wg.Add(1)

		pid := curpid
		p := curp

		// read stuff
		go func() {
			done := p.NotifyDone(make(chan bool)) // process is done...
			events := p.NotifyEvents(make(chan *supervisor.Event, 1000))
			logger := log.New(ColorWriter(colors[pid]), "", log.LstdFlags)

			for {
				select {
				case msg := <-p.Stdout:
					logger.Printf("%v:INFO  %s", pid, *msg)
				case msg := <-p.Stderr:
					logger.Printf("%v:ERROR %s", pid, *msg)
				case event := <-events:
					if config.Debug {
						logger.Println(event.Message)
					}
				case <-done: // process quit
					logger.Printf("%v:STARTER Closing loop we are done....", pid)
					wg.Done()
					return
				}
			}
		}()
	}

	wg.Wait()
}
