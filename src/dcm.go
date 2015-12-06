package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type doForService func(string, yamlConfig) (int, error)

type Dcm struct {
	Config *Config
	Args   []string
}

func NewDcm(c *Config, args []string) *Dcm {
	return &Dcm{c, args}
}

func (d *Dcm) Command() (int, error) {
	if len(d.Args) < 1 {
		d.Usage()
		return 1, nil
	}

	switch d.Args[0] {
	case "help", "h":
		d.Usage()
		return 0, nil
	case "setup":
		return d.Setup()
	case "run", "r":
		return d.Run(d.Args[1:]...)
	case "build", "b":
		return d.Run("build")
	case "update", "u":
		return d.Update()
	case "shell", "sh":
		return d.Shell()
	case "goto", "gt":
		return d.Goto(d.Args[1:]...)
	case "purge", "rm":
		return d.Purge(d.Args[1:]...)
	case "unload", "ul":
		return d.Unload()
	default:
		d.Usage()
		return 127, nil
	}
}

func (d *Dcm) Setup() (int, error) {
	if _, err := os.Stat(d.Config.Srv); os.IsNotExist(err) {
		os.MkdirAll(d.Config.Srv, 0777)
	}

	return d.doForEachService(func(service string, configs yamlConfig) (int, error) {
		repo, ok := getMapVal(configs, "labels", "com.dcm.repository").(string)
		if !ok {
			return 1, errors.New(
				"Error reading git repository config for service: " + service)
		}
		dir := d.Config.Srv + "/" + service
		cmd := cmd("git", "clone", repo, dir)
		cmd.Dir = d.Config.Dir
		if err := cmd.Run(); err != nil {
			return 1, errors.New("Error cloning git repository for service: " + service)
		}

		return 0, nil
	})
}

func (d *Dcm) doForEachService(fn doForService) (int, error) {
	for service, configs := range d.Config.Config {
		service, _ := service.(string)
		configs, ok := configs.(yamlConfig)
		if !ok {
			panic("Error reading git repository config for service: " + service)
		}

		code, err := fn(service, configs)
		if err != nil {
			return code, err
		}
	}

	return 0, nil
}

func (d *Dcm) Run(args ...string) (int, error) {
	if len(args) == 0 {
		args = append(args, "default")
	}

	switch args[0] {
	case "execute":
		return d.runExecute()
	case "init":
		fmt.Println("Initializing project [" + d.Config.Project + "]...")
		return d.runInit()
	case "build":
		fmt.Println("Building project [" + d.Config.Project + "]...")
		return d.Run("execute", "build")
	case "start":
		fmt.Println("Starting project [" + d.Config.Project + "]...")
		return d.Run("execute", "start")
	case "stop":
		fmt.Println("Stopping project [" + d.Config.Project + "]...")
		return d.Run("execute", "stop")
	case "restart":
		fmt.Println("Restarting project [" + d.Config.Project + "]...")
		return d.Run("execute", "restart")
	case "up":
		fmt.Println("Bringing up project [" + d.Config.Project + "]...")
		return d.runUp()
	default:
		return d.Run("up")
	}
}

func (d *Dcm) runExecute() (int, error) {
	cmd := cmd("docker-compose", args[1:]...)
	cmd.Dir = d.Config.Dir
	cmd.Env = append(
		os.Environ(),
		"COMPOSE_PROJECT_NAME="+d.Config.Project,
		"COMPOSE_FILE="+d.Config.File,
	)
	if err := cmd.Run(); err != nil {
		return 1, fmt.Errorf(
			"Error executing docker-compose with args [%s], and envs [%s]",
			strings.Join(args[1:], ", "), strings.Join(cmd.Env, ", "),
		)
	}
	return 0, nil
}

func (d *Dcm) runInit() (int, error) {
	return d.doForEachService(func(service string, configs yamlConfig) (int, error) {
		init, ok := getMapVal(
			d.Config.Config, service, "labels", "com.dcm.initscript").(string)
		if !ok {
			fmt.Println("Skipping init script for service:", service, "...")
			return 0, nil
		}
		if err := os.Chdir(d.Config.Srv); err != nil {
			return 1, err
		}
		cmd := cmd("/bin/sh", init)
		cmd.Dir = d.Config.Srv + "/" + service + "/"
		if err := cmd.Run(); err != nil {
			return 1, fmt.Errorf(
				"Error executing init script [%s] for service [%s]: %s",
				init, service, err,
			)
		}
		return 0, nil
	})
}

func (d *Dcm) runUp() (int, error) {
	code, err := d.Run("execute", "up", "-d", "--force-recreate")
	if err != nil {
		return code, err
	}
	return d.Run("init")
}

func (d *Dcm) Update() (int, error) {
	return 0, nil
}

func (d *Dcm) Shell() (int, error) {
	return 0, nil
}

func (d *Dcm) Goto(args ...string) (int, error) {
	return 0, nil
}

func (d *Dcm) Purge(args ...string) (int, error) {
	return 0, nil
}

func (d *Dcm) Unload() (int, error) {
	return 0, nil
}

func (d *Dcm) Usage() {
	fmt.Println("")
	fmt.Println("Docker Compose Manager")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  dcm help                Show this message")
	fmt.Println("                          shorthand ver.: `dcm h`")
	fmt.Println("  dcm setup               Check out all the repositories for API, UI and services")
	fmt.Println("  dcm run [<args>]        Run docker-compose commands. If <args> is not given, by")
	fmt.Println("                          default DCM will run `up` command.")
	fmt.Println("                          <args>: up, build, start, stop, restart")
	fmt.Println("                          shorthand ver.: `dcm r [<args>]`")
	fmt.Println("  dcm run build           Run build command that (re)create all the service images")
	fmt.Println("                          shorthand ver.: `dcm build` or `dcm b`")
	fmt.Println("  dcm shell <service>     Log into a given service container")
	fmt.Println("                          <service>: api, ui, postgres, mongo, redis, nginx, php")
	fmt.Println("                          shorthand ver.: `dcm sh <service>`")
	fmt.Println("  dcm purge [<type>]      Remove either all the containers or all the images or")
	fmt.Println("                          everything. If <type> is not given, by default DCM will")
	fmt.Println("                          purge everything")
	fmt.Println("                          <type>: images|img, containers|con, all")
	fmt.Println("                          shorthand ver.: `dcm rm [<type>]`")
	fmt.Println("  dcm branch <service>    Display the current branch for the given service name")
	fmt.Println("                          <service>: api, ui, postgres, mongo, redis, nginx, php")
	fmt.Println("                          shorthand ver.: `dcm br <service>`")
	fmt.Println("  dcm goto [<service>]    Go to the service's folder. If <service> is not given,")
	fmt.Println("                          by default DCM will go to $DCM_DIR")
	fmt.Println("                          <service>: api, ui, postgres, mongo, redis, nginx, php")
	fmt.Println("                          shorthand ver.: `dcm gt [<service>]`")
	fmt.Println("  dcm update [<service>]  Update DCM and dependent services (PostgrSQL, MongoDB,")
	fmt.Println("                          Redis, Nginx and Base PHP). If <service> is not given,")
	fmt.Println("                          by default DCM will update everything except api and ui.")
	fmt.Println("                          <service>: postgres, mongo, redis, nginx, php")
	fmt.Println("                          shorthand ver.: `dcm u`")
	fmt.Println("")
	fmt.Println("Example:")
	fmt.Println("  Initial setup")
	fmt.Println("    dcm setup")
	fmt.Println("    dcm run")
	fmt.Println("")
	fmt.Println("  Rebuild API or UI after switching branch")
	fmt.Println("    dcm build")
	fmt.Println("    dcm run")
	fmt.Println("")
	fmt.Println("  Log into different service containers")
	fmt.Println("    dcm shell api")
	fmt.Println("    dcm shell ui")
	fmt.Println("    ...")
	fmt.Println("")
}
