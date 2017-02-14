package main

import (
	"fmt"
)

type invalidTargetError struct {
	target string
}

func (e *invalidTargetError) Error() string {
	return fmt.Sprintf("Target '%s' is not defined in %s.", e.target, configFile)
}
