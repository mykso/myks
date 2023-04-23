package kwhoosh

import (
	"errors"
	"os"
	"path/filepath"
)

type Application struct {
	// Name of the application
	Name string
	// Application prototype name
	Prototype string
	// Environment
	e *Environment
}

func NewApplication(e *Environment, name string, prototype string) (*Application, error) {
	if prototype == "" {
		prototype = name
	}

	app := &Application{
		Name:      name,
		Prototype: prototype,
		e:         e,
	}

	// Check if the application prototype exists
	if _, err := os.Stat(filepath.Join(e.k.PrototypesDir, prototype)); err != nil {
		return nil, errors.New("Application prototype does not exist")
	}

	return app, nil
}
