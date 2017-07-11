package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func fmtCyan(a ...interface{}) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprint(a...)
	}
	return fmt.Sprint("\x1b[36m", fmt.Sprint(a...), "\x1b[30m")
}

func fmtGreen(a ...interface{}) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprint(a...)
	}
	return fmt.Sprint("\x1b[32m", fmt.Sprint(a...), "\x1b[30m")
}

func debug(val interface{}) {
	fmt.Printf("%+v\n", val)
}

func channelWait(c chan bool, num int) {
	for i := 0; i < num; i++ {
		<-c
	}
	close(c)
}

func timestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func commonPath(paths []string, index int) string {
	if len(paths) < 2 {
		return ""
	} else if index >= len(paths[0]) {
		return paths[0][:index-1]
	}

	value := paths[0][index]
	for i, innerPath := range paths {
		if index >= len(paths[0]) {
			return paths[0][0 : index-1]
		} else if innerPath[index] != value {
			return paths[0][0 : index-1]
		} else if innerPath[index] == value && i == len(paths)-1 {
			return commonPath(paths, index+1)
		}
	}
	return ""
}

func filesInPath(path string) (files, dirs []string, err error) {
	filepath.Walk(path,
		func(path string, f os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if strings.HasPrefix(f.Name(), ".") {
				if f.IsDir() {
					return filepath.SkipDir
				}
			} else if f.IsDir() {
				dirs = append(dirs, filepath.ToSlash(path))
			} else {
				files = append(files, filepath.ToSlash(path))
			}
			return nil
		})

	return files, dirs, err
}

func errorMsg(message string, err error) {
	if err != nil {
		fmt.Printf("\nError: %s\n     %s\n\n", message, err)
	} else {
		fmt.Printf("\nError: %s\n\n", message)
	}
}

func stringInSlice(value string, slice []string) bool {
	for _, item := range slice {
		if value == item {
			return true
		}
	}
	return false
}
