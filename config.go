package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

var validActions = []string{"collate", "concat", "js-minify", "sass", "shell"}

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
	Targets []string
	Globs   []string
}

// Action defines the struct for a specific action to perform in a task
type Action struct {
	Action  string
	Targets []string
	Options map[string]interface{}
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
		return unmarshalledData, &invalidTargetError{unmarshalledData.Target}
	}

	if _, err := checkValidActions(unmarshalledData.Tasks); err != nil {
		return unmarshalledData, err
	}

	return unmarshalledData, err
}

func checkValidActions(tasks map[string]Task) (bool, error) {
	for taskName, task := range tasks {
		for _, action := range task.Actions {
			if !stringInSlice(action.Action, validActions) {
				return false, fmt.Errorf("invalid action '%s' in task '%s'", action.Action, taskName)
			}
		}
	}
	return true, nil
}
