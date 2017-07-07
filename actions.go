package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/js"
	"github.com/wellington/go-libsass"
)

// Actioner defines the interface for any action
type Actioner interface {
	Action(files []string, options map[string]interface{}) (outputFiles []string)
}

type collateAction struct{}

func (action collateAction) Action(files []string, options map[string]interface{}) (outputFiles []string) {
	if len(files) == 0 {
		return files
	}

	regex, err := targetPathRegex()
	if err != nil {
		errorMsg("Could not parse target regular expression", err)
		return files
	}

	outputDir, ok := options["output"]
	if !ok {
		outputDir = config.BuildDir
	} else {
		outputDir = fmt.Sprintf("%s%s", config.BuildDir, outputDir)
	}

	for _, file := range files {
		if strings.Index(file, config.BuildDir) > -1 {
			errorMsg(fmt.Sprintf("Cannot pass a build directory file to 'collate' action. File: '%s'.", file), nil)
			continue
		}
		newFile := regex.ReplaceAllString(file, "")

		dir := filepath.Dir(newFile)
		dir = fmt.Sprintf("%s%s", outputDir, dir)
		newFile = fmt.Sprintf("%s%s", outputDir, newFile)
		outputFiles = append(outputFiles, newFile)

		os.MkdirAll(dir, 0744)
		content, _ := ioutil.ReadFile(file)
		ioutil.WriteFile(newFile, content, 0644)
	}
	return outputFiles
}

type concatAction struct{}

func (action concatAction) Action(files []string, options map[string]interface{}) (outputFiles []string) {
	if len(files) == 0 {
		return files
	}

	separator, ok := options["separator"].(string)
	if !ok {
		separator = "\n"
	}

	outputFile, ok := options["output"].(string)
	if !ok {
		errorMsg("No output file defined for 'concat' action. Skipping task...", nil)
		return files
	}
	outputFile = fmt.Sprintf("%s%s", config.BuildDir, outputFile)

	var concat bytes.Buffer
	for i, file := range files {
		content, err := ioutil.ReadFile(file)
		if err != nil {
			errorMsg(fmt.Sprintf("Could not read file '%s'", file), err)
			return
		}

		if i > 0 {
			concat.Write([]byte(separator))
		}
		concat.Write(content)
	}

	dir := filepath.Dir(outputFile)
	err := os.MkdirAll(dir, 0744)
	if err != nil {
		errorMsg(fmt.Sprintf("Could not create directory for '%s' defined in 'concat' action. Skipping task...", outputFile), err)
		return files
	}

	err = ioutil.WriteFile(outputFile, concat.Bytes(), 0644)
	if err != nil {
		errorMsg(fmt.Sprintf("Could not write to '%s' defined in 'concat' action. Skipping task...", outputFile), err)
		return files
	}
	return []string{outputFile}
}

type jsMinifyAction struct{}

func (action jsMinifyAction) Action(files []string, options map[string]interface{}) (outputFiles []string) {
	if len(files) == 0 {
		return files
	}

	targetRegex, err := targetPathRegex()
	if err != nil {
		errorMsg("Could not parse target regular expression", err)
		return files
	}

	c := make(chan string)
	defer close(c)

	for _, file := range files {
		file := file
		go func() {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				errorMsg(fmt.Sprintf("Could not read file '%s'", file), err)
				c <- ""
				return
			}

			m := minify.New()
			m.AddFunc("text/javascript", js.Minify)
			data, err = m.Bytes("text/javascript", data)
			if err != nil {
				errorMsg(fmt.Sprintf("Could not read file '%s'", file), err)
				c <- ""
				return
			}

			newFile, ok := options["output"].(string)
			if len(files) == 1 && ok {
				newFile = fmt.Sprintf("%s%s", config.BuildDir, newFile)
			} else {
				file = targetRegex.ReplaceAllString(file, "")
				file = strings.Replace(file, config.BuildDir, "", -1) // Replace BuildDir because jsMinify can receive build files as input
				ext := filepath.Ext(file)
				newFile = fmt.Sprintf("%s%s%s%s", config.BuildDir, file[:len(file)-len(ext)], ".min", ext)
			}

			dir := filepath.Dir(newFile)
			err = os.MkdirAll(dir, 0744)
			if err != nil {
				errorMsg(fmt.Sprintf("Could not create directory for '%s' defined in 'jsMinify' action.", newFile), err)
				c <- ""
				return
			}

			err = ioutil.WriteFile(newFile, data, 0644)
			if err != nil {
				errorMsg(fmt.Sprintf("Could not write to '%s' defined in 'jsMinify' action.", newFile), err)
				c <- ""
				return
			}

			c <- newFile
		}()
	}

	// Wait for all go routines to finish
	for i := 0; i < len(files); i++ {
		outputFile := <-c
		if outputFile == "" {
			continue
		}
		outputFiles = append(outputFiles, outputFile)
	}
	return outputFiles
}

type sassAction struct{}

func (action sassAction) Action(files []string, options map[string]interface{}) (outputFiles []string) {
	if len(files) == 0 {
		return files
	}

	// Collate files to their new location
	actioner := new(collateAction)
	collatedFiles := actioner.Action(files, options)

	// Compile SASS
	c := make(chan string)
	defer close(c)

	for _, file := range collatedFiles {
		file := file
		go func() {
			sourceMap := fmt.Sprintf("%s%s", file, ".map") // Remove the source map if it exists. If the source map is not removed, LibSASS throws an error
			os.Remove(sourceMap)

			bb := new(bytes.Buffer)
			comp, err := libsass.New(bb, nil)
			if err != nil {
				errorMsg("Could not initialize libsass", err)
				c <- ""
				return
			}
			comp.Option(libsass.Path(file))
			comp.Option(libsass.OutputStyle(3))
			comp.Option(libsass.SourceMap(true, sourceMap, filepath.Dir(file)))
			if err = comp.Run(); err != nil {
				errorMsg(fmt.Sprintf("Could not compile file '%s'.", file), err)
				c <- ""
				return
			}

			ofFile := fmt.Sprintf("%s%s", file[:len(file)-len(filepath.Ext(file))], ".css")
			ioutil.WriteFile(ofFile, bb.Bytes(), 0644)
			c <- ofFile
		}()
	}

	// Wait for all go routines to finish
	for i := 0; i < len(collatedFiles); i++ {
		outputFile := <-c
		if outputFile == "" {
			continue
		}
		outputFiles = append(outputFiles, outputFile)
	}

	// Remove original SASS files
	for _, file := range collatedFiles {
		os.Remove(file)
	}

	return outputFiles
}

type cmdAction struct{}

func (action cmdAction) Action(files []string, options map[string]interface{}) (outputFiles []string) {
	cmdName, ok := options["name"].(string)
	if !ok {
		errorMsg("Invalid command name", nil)
		return outputFiles
	}

	args, ok := options["args"].([]interface{})
	if !ok {
		errorMsg("Invalid arguments array", nil)
		return outputFiles
	}

	var argArray []string
	loopFiles := false
	for _, arg := range args {
		var argString string
		if argString, ok = arg.(string); !ok {
			errorMsg("Invalid arguments array", nil)
			return outputFiles
		}

		if argString == "{FILES}" {
			argArray = append(argArray, files...)
		} else {
			if argString == "{FILE}" {
				loopFiles = true
			}
			argArray = append(argArray, argString)
		}
	}

	if loopFiles {
		for _, file := range files {
			for i, arg := range argArray {
				if arg == "{FILE}" {
					argArray[i] = file
				}
			}
			runCommand(cmdName, argArray)
		}
	} else {
		runCommand(cmdName, argArray)
	}

	return outputFiles
}

func runCommand(cmdName string, args []string) {
	cmd := exec.Command(cmdName, args...)
	if err := cmd.Run(); err != nil {
		errorMsg(fmt.Sprintf("Error running command '%s'", cmdName), err)
	}
}
