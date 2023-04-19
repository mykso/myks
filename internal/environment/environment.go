package environment

type Environment struct {
	// Path to the environment directory
	Path string
}

func New(path string) *Environment {
	return &Environment{
		Path: path,
	}
}
