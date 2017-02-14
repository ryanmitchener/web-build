package main

import (
	"fmt"
)

type InvalidTargetError struct {
	target string
}

func (e *InvalidTargetError) Error() string {
	return fmt.Sprintf("Target '%s' is not defined in %s.", e.target, configFile)
}
