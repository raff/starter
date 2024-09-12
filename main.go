package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
        "regexp"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gobs/pretty"
	"github.com/mgutz/ansi"
	"github.com/kontera-technologies/go-supervisor"
)

const (
	VERSION = "0.4"
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

	Manual bool `toml:"manual"` // don't start automatically (select from command line)
	//Restart bool `toml:"restart"`       // restart on termination

	Next string `toml:"next"` // in workflow mode, start "next" application after this ends

	Count int `toml:"count"` // this is a template, generate `Count` instances of this application
}

type Config struct {
	Respawns   int  `toml:"respawns"`   // number of attempts to start a process
	Interrupts int  `toml:"interrupts"` // number of attempts to interrupt the process before killing it
	MaxSpawns  int  `toml:"max-spawns"` // max spawns limit
	Debug      bool `toml:"debug"`      // log supervisor events
	Colors     bool `toml:"colors"`     // colorize logs
	Workflow   bool `toml:"workflow"`   // execute applications in sequence, instead of starting all of them
        Patterns map[string]string `toml:"patterns"` // map of color -> regex patterns

	Applications []*Application `toml:"application"` // list of applications to start and monitor

	Environment map[string]string `toml:"env"` // environment variables

	colors map[string]string
        recolors map[string]*regexp.Regexp
	manual map[string]bool
}

func (c *Config) getApp(name string) *Application {
	for _, app := range c.Applications {
		if app.Id == name {
			return app
		}
	}

	return nil
}

func expandEnv(s string) string {
	mapper := func(vname string) string {
		vd := strings.SplitN(vname, ":-", 2)
		if len(vd) == 1 {
			return os.Getenv(vname)
		}

		if venv := os.Getenv(vd[0]); venv != "" {
			return venv
		}

		return vd[1]
	}

	return os.Expand(s, mapper)
}

func getConfig() *Config {
	var config Config

	cfile := flag.String("conf", "starter.conf", "configuration file")
	version := flag.Bool("version", false, "print version and exit")
	printConf := flag.Bool("print-conf", false, "pretty-print configuration file and exit")

	flag.BoolVar(&config.Debug, "debug", false, "log supervisor events")
	flag.BoolVar(&config.Colors, "colors", true, "enable/disable colorizing")
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

	config.manual = map[string]bool{} // applications selected from command line
	config.colors = map[string]string{}
	config.recolors = map[string]*regexp.Regexp{}

        for c, p := range config.Patterns {
            config.recolors[c] = regexp.MustCompile(p)
        }

	for k, v := range config.Environment {
		os.Setenv(k, expandEnv(v))
	}

	for _, app := range flag.Args() {
		if config.getApp(app) == nil { // validate application names
			log.Printf("invalid application name %q", app)
			continue
		}

		config.manual[app] = true
	}

	var templates []*Application

	for _, app := range config.Applications {
		if app.Count > 0 { // template
			templates = append(templates, app)
		}
	}

	for _, t := range templates {
		for i := 0; i < t.Count; i++ {
			app := *t
			app.Count = 0

			config.Applications = append(config.Applications, &app)
		}
	}

	for i, app := range config.Applications {
		if app.Id == "" {
			app.Id = fmt.Sprintf("app-%v", i+1)
		} else if strings.Contains(app.Id, "%") {
			app.Id = fmt.Sprintf(app.Id, i+1)
		}
		if app.Color == "" {
			app.Color = "off"
		} else if app.Color == "auto" {
			c := 231 - i
			if c < 20 {
				c = 255
			}

			app.Color = fmt.Sprintf("%v", c)
		} else {
			app.Color = expandEnv(app.Color)
		}

		config.colors[app.Id] = app.Color

		app.Program = expandEnv(app.Program)
		app.Dir = expandEnv(app.Dir)

		for i, arg := range app.Args {
			app.Args[i] = expandEnv(arg)
		}
	}

	if *printConf {
		pretty.PrettyPrint(config)
		return nil
	}

	return &config
}

type colorWriter struct {
	colorize func(string) string
        recolors map[string]*regexp.Regexp
}

func ColorWriter(c string, recols map[string]*regexp.Regexp) io.Writer {
	return &colorWriter{ansi.ColorFunc(c), recols}
}

func (w *colorWriter) Write(b []byte) (int, error) {
	s := strings.TrimRight(string(b), "\r\n")

        for c, r := range w.recolors {
            if r.MatchString(s) {
	        return fmt.Println(ansi.Color(s, c))
            }
        }

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

	defaultLogger := log.New(os.Stdout, "", log.LstdFlags)

	var wg sync.WaitGroup
	var startApplication func(*Application, bool)

	startApplication = func(app *Application, stopOnSuccess bool) {
		p, err := supervisor.Supervise(app.Program, supervisor.Options{
			Args:                    app.Args,          // argumets to pass ( default is none )
			SpawnAttempts:           config.Respawns,   // attempts before giving up ( default 10 )
			AttemptsBeforeTerminate: config.Interrupts, // on Stop() terminate process after X interrupt attempts (default is 10)
			StopOnSuccess:           stopOnSuccess,     // in workflow mode we want to run only once
			Dir:                     app.Dir,           // run dir ( default is current dir )
			Id:                      app.Id,            // will be added to every log print ( default is "NOID")
			MaxSpawns:               config.MaxSpawns,  // Max spawn limit ( default is 1 )
			StdoutIdleTime:          app.StdoutIdle,    // stop worker if we didn't recived stdout message in X seconds ( default is 0 - disbaled )
			StderrIdleTime:          app.StderrIdle,    // stop worker if we didn't recived stderr message in X seconds ( default is 0 - disbaled )

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
			return
		}

		wg.Add(1)

		// read stuff
		go func(app *Application, p *supervisor.Process) {
			pid := app.Id

			done := p.NotifyDone(make(chan bool)) // process is done...
			events := p.NotifyEvents(make(chan *supervisor.Event, 1000))
			logger := defaultLogger
			if config.Colors {
				logger = log.New(ColorWriter(config.colors[pid], config.recolors), "", log.LstdFlags)
			}

			for {
				select {
				case msg := <-p.Stdout:
					logger.Printf("%v:O  %s", pid, *msg)
				case msg := <-p.Stderr:
					logger.Printf("%v:E %s", pid, *msg)
				case event := <-events:
					if config.Debug {
						logger.Println(event.Code, event.Message) // the message already contains the pid
					}
				case <-done: // process quit
					logger.Printf("%v: DONE", pid)
					defer wg.Done()

					if config.Workflow && app.Next != "" { // start next step
						if p.LastError() != nil {
							logger.Printf("%v: Step terminated with error %v: stop workflow", pid, p.LastError())
							return
						}

						next := config.getApp(app.Next)
						if next == nil {
							log.Printf("%v: invalid next application %q", pid, app.Next)
						} else {
							startApplication(next, true)
						}
					}

					return
				}
			}
		}(app, p)
	}

	if config.Workflow {
		// here we want to run any manual task to start one or more workflows
		// if there are no selected manual tasks, we pick the first in the list

		if len(config.manual) == 0 {
			app := config.Applications[0].Id
			config.manual[app] = true
		}

		for appid := range config.manual {
			app := config.getApp(appid)
			startApplication(app, true)
		}
	} else {
		// here we want to run all "automatic" tasks, and all manual tasks that have been selected on the command line
		for _, app := range config.Applications {
			if app.Manual && !config.manual[app.Id] {
				continue
			}

			startApplication(app, false)
		}
	}

	time.Sleep(time.Second)
	wg.Wait()
}
