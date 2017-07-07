package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

const version string = "1.0.1"

var argZip string
var argTarget string
var argVersion bool
var argWatch bool
var config Config
var srcFiles []string
var srcDirs []string

func main() {
	initFlags()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			initializeEmptyProject()
		case "clean":
			loadConfigAndClean()
		default:
			run(nil, true)
		}
	} else {
		run(nil, true)
	}
}

func initFlags() {
	flag.Usage = func() {
		fmt.Printf("Usage of %s:\n  web-build [COMMAND] OR web-build [FLAGS]\n\n", os.Args[0])
		fmt.Printf("  Commands:\n  init\n\tInitialize an empty project complete with: source directory, 'common' target and a default 'web-build.json.'\n  clean\n\tClear the build directory\n\n")
		fmt.Printf("  Flags:\n")
		flag.PrintDefaults()
	}
	flag.StringVar(&argTarget, "target", "", "Specify the target to build. This will override the target specified in 'web-build.json'.")
	flag.StringVar(&argZip, "zip", "", "Compress the build upon completion of program. Specify the location and name of the zip file. Example: './app.zip'")
	flag.BoolVar(&argVersion, "version", false, "Show the current version")
	flag.BoolVar(&argWatch, "watch", false, "Runs web-build and watches all files specified by user configuration globs for changes.")
	flag.Parse()

	if argVersion {
		fmt.Printf("web-build version %s\n", version)
		os.Exit(0)
	}
}

func loadConfigAndClean() {
	var err error
	config, err = parseConfig()
	if err != nil {
		errorMsg("Could not parse configuration file.", err)
		return
	}
	clean()
}

func clean() {
	var err error
	err = os.RemoveAll(config.BuildDir)
	if err != nil {
		errorMsg(fmt.Sprintf("Could not remove all contents from %s.", config.BuildDir), err)
		return
	}
}

func initializeEmptyProject() {
	var err error
	config, err = parseConfig()
	if err != nil {
		errorMsg("Could not parse configuration file.", err)
		os.Exit(1)
	}

	files, err := ioutil.ReadDir("./")
	if err != nil {
		errorMsg("Cannot access current working directory to initialize web-build", err)
		os.Exit(1)
	} else if len(files) > 0 {
		errorMsg("Cannot initialize web-build in a non-empty directory", nil)
		os.Exit(1)
	}

	data, err := configTemplate()
	if err != nil {
		errorMsg("Could not initialize due to template decoding error. Exiting.", err)
		os.Exit(1)
	}

	err = ioutil.WriteFile("web-build.json", data, 0744)
	if err != nil {
		errorMsg("Could not write web-build.json template. Exiting.", err)
		os.Exit(1)
	}

	err = os.MkdirAll("./src/common", 0744)
	if err != nil {
		errorMsg("Could not create source directory. Exiting.", err)
		os.Exit(1)
	}
}

func run(done chan<- bool, runWatcher bool) {
	var err error
	start := timestamp()

	if done != nil {
		defer func() { done <- true }()
	}

	config, err = parseConfig()
	if err != nil {
		errorMsg("Could not parse configuration file.", err)
		return
	}

	err = setup()
	if err != nil {
		errorMsg("Error during setup.", err)
		return
	}

	fmt.Printf("Building target: %s\n", fmtCyan(config.Target))
	fmt.Printf("Running Tasks...\n")
	runTasks(config.Tasks)
	fmt.Printf("Completed in: %s\n\n", fmtCyan(timestamp()-start, "ms"))

	if argZip != "" {
		createZip(argZip)
	}

	if runWatcher && argWatch {
		watch()
	}
}

func setup() error {
	if argTarget != "" {
		config.Target = argTarget
		if !checkValidTarget(argTarget, config) {
			errorMsg(fmt.Sprintf("The target '%s' is invalid.", argTarget), nil)
			return &invalidTargetError{argTarget}
		}
	}

	if _, err := os.Stat(config.SrcDir); err != nil {
		errorMsg(fmt.Sprintf("Source directory '%s' does not exist.", config.SrcDir), nil)
		return err
	} else if config.SrcDir == config.BuildDir {
		errorMsg("Source directory cannot be the same as the build directory.", nil)
		return err
	}

	// Prevent relative paths in config (i.e. use of ../../) from messing things up for SrcDir and BuildDir
	config.SrcDir, _ = filepath.Abs(config.SrcDir)
	config.SrcDir = filepath.ToSlash(config.SrcDir)
	config.BuildDir, _ = filepath.Abs(config.BuildDir)
	config.BuildDir = filepath.ToSlash(config.BuildDir)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		clean()
	}()
	go func() {
		defer wg.Done()
		var err error
		srcFiles, srcDirs, err = filesInPath(config.SrcDir)
		if err != nil {
			errorMsg(fmt.Sprintf("Folder '%s' not found.\n", config.SrcDir), err)
		}
	}()
	wg.Wait()
	return nil
}

func runTasks(tasks map[string]Task) {
	var wg sync.WaitGroup
	for name, task := range tasks {
		name := name
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			runTask(name, task)
		}()
	}
	wg.Wait()
}

func runTask(name string, task Task) {
	start := timestamp()
	files := resolveTargetFiles(task.Globs)
	prevOutput := files

	for _, action := range task.Actions {
		var actioner Actioner
		switch action.Action {
		case "collate":
			actioner = collateAction{}
		case "concat":
			actioner = concatAction{}
		case "js-minify":
			actioner = jsMinifyAction{}
		case "sass":
			actioner = sassAction{}
		case "cmd":
			actioner = cmdAction{}
		default:
			continue
		}
		prevOutput = actioner.Action(prevOutput, action.Options)
	}
	fmt.Printf("  %s: %s\n", fmtGreen(name), fmtCyan(timestamp()-start, "ms"))
}

func createZip(outputPath string) {
	fmt.Printf("Creating archive...\n")
	of, err := os.Create(outputPath)
	if err != nil {
		errorMsg(fmt.Sprintf("Could not create zip file '%s'", outputPath), err)
		return
	}
	defer of.Close()

	w := zip.NewWriter(of)
	defer w.Close()

	files, _, err := filesInPath(config.BuildDir)
	if err != nil {
		errorMsg(fmt.Sprintf("Folder '%s' not found.\n", config.BuildDir), err)
		return
	}

	for _, file := range files {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			errorMsg(fmt.Sprintf("Could not read file '%s' while creating zip archive. Removing archive", file), err)
			return
		}

		file = strings.Replace(file, config.BuildDir, "", -1)[1:]
		f, err := w.Create(file)
		if err != nil {
			errorMsg(fmt.Sprintf("Could not add file '%s' to archive.", file), err)
			return
		}

		_, err = f.Write(data)
		if err != nil {
			errorMsg(fmt.Sprintf("Could not write file '%s' to archive.", file), err)
			return
		}
	}
	fmt.Printf("Created archive '%s'.\n\n", outputPath)
}

func resolveTargetFiles(globs []string) []string {
	var files []string
	var keyOrder []string
	fileCache := make(map[string]string)
	target := config.Target
	dependencies := []string{target}

	// Get dependencies
	for config.Targets[target].Dependency != "" {
		target = config.Targets[target].Dependency
		dependencies = append([]string{target}, dependencies...)
	}

	// Resolve the file list
	for _, innerTarget := range dependencies {
		globFiles := glob(globs, fmt.Sprintf("%s/%s", config.SrcDir, innerTarget))
		for _, file := range globFiles {
			relativePath := strings.Replace(file, fmt.Sprintf("%s/%s", config.SrcDir, innerTarget), "", -1)
			if _, ok := fileCache[relativePath]; !ok {
				keyOrder = append(keyOrder, relativePath)
			}
			fileCache[relativePath] = file
		}
	}

	for _, key := range keyOrder {
		files = append(files, fileCache[key])
	}

	return files
}

func glob(globs []string, baseDir string) []string {
	var foundFiles []string
	baseDirLen := len(baseDir)

	for _, glob := range globs {
		exclusion := []rune(glob)[0] == []rune("!")[0]
		glob = strings.Replace(glob, ".", "\\.", -1)
		glob = strings.Replace(glob, "**", "__double-star-placeholder__", -1) // Have to use a placeholder so that the single asterisk replacement doesn't affect this
		glob = strings.Replace(glob, "*", "[^\\/]*", -1)
		glob = strings.Replace(glob, "__double-star-placeholder__", ".*", -1)
		glob = fmt.Sprintf("%s%s", glob, "$")

		if exclusion {
			glob = glob[1:]
		}

		r, err := regexp.Compile(glob)
		if err != nil {
			errorMsg("Invalid regular expression in glob.", err)
			continue
		} else if exclusion {
			for j := 0; j < len(foundFiles); j++ {
				if r.MatchString(foundFiles[j][baseDirLen:]) {
					foundFiles = append(foundFiles[:j], foundFiles[j+1:]...)
					j--
				}
			}
		} else {
			for _, file := range srcFiles {
				if len(file) <= baseDirLen || file[:baseDirLen] != baseDir {
					continue
				} else if r.MatchString(file[baseDirLen:]) {
					foundFiles = append(foundFiles, file)
				}
			}
		}
	}

	return foundFiles
}

func targetPathRegex() (*regexp.Regexp, error) {
	expression := fmt.Sprintf("%s/(", config.SrcDir)
	count := 0
	for k := range config.Targets {
		if count > 0 {
			expression += "|"
		}
		expression += k
		count++
	}
	expression += ")"
	return regexp.Compile(expression)
}

func checkValidTarget(targetName string, c Config) bool {
	for target := range c.Targets {
		if target == targetName {
			return true
		}
	}
	return false
}

func watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		errorMsg("Error creating file watcher.", err)
		return
	}
	defer watcher.Close()

	file, _ := filepath.Abs(configFile)
	err = watcher.Add(file)
	if err != nil {
		errorMsg(fmt.Sprintf("Could not add file '%s'", file), err)
		os.Exit(1)
	}

	watches := make(map[string]bool)
	for _, dir := range srcDirs {
		watches[dir] = true
		err = watcher.Add(dir)
		if err != nil {
			errorMsg(fmt.Sprintf("Could not add file '%s'", dir), err)
			os.Exit(1)
		}
	}

	go handleWatcherEvent(watcher, &watches)
	done := make(chan bool)
	<-done
}

func handleWatcherEvent(watcher *fsnotify.Watcher, watchesMap *map[string]bool) {
	watches := *watchesMap
	busy := false
	done := make(chan bool)

	for {
		select {
		case event := <-watcher.Events:
			info, err := os.Stat(event.Name)
			if err != nil {
				errorMsg("Could not stat file in watcher", err)
				continue
			} else if info.IsDir() {
				if event.Op&fsnotify.Remove == fsnotify.Remove {
					watcher.Remove(event.Name)
					delete(watches, event.Name)
				} else if event.Op&fsnotify.Create == fsnotify.Create {
					if _, ok := watches[event.Name]; !ok {
						watches[event.Name] = true
						watcher.Add(event.Name)
					}
				}
			}

			if busy {
				continue
			}
			busy = true
			go run(done, false)
		case <-done:
			busy = false
		case err := <-watcher.Errors:
			errorMsg("Error while watching files", err)
		}
	}
}
