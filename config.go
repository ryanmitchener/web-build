package main

import (
	"encoding/json"
	"io/ioutil"
)

// Config defines the struct for the User Configuration file
type Config struct {
	TemplateVersion int
	SrcDir          string
	BuildDir        string
	Tasks           map[string]Task
	Targets         map[string]struct {
		Dependency string
	}
	Target string
}

// Task defines the struct for a task
type Task struct {
	Actions []Action
	Globs   []string
}

// Action defines the struct for a specific action to perform in a task
type Action struct {
	Action  string
	Options map[string]string
}

const configFile = "./web-build.json"

func parseConfig() (Config, error) {
	var unmarshalledData Config
	jsonContent, err := ioutil.ReadFile(configFile)
	if err != nil {
		return unmarshalledData, err
	}

	err = json.Unmarshal(jsonContent, &unmarshalledData)
	if err != nil {
		return unmarshalledData, err
	}

	if !checkValidTarget(unmarshalledData.Target, unmarshalledData) {
		return unmarshalledData, &InvalidTargetError{unmarshalledData.Target}
	}

	return unmarshalledData, err
}
