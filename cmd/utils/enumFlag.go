package utils

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type EnumFlag struct {
	enumMap map[string]string
	kind    string
	value   string
}

func NewEnumFlag(kind string, defaultValue string, enumMap map[string]string) *EnumFlag {
	return &EnumFlag{
		enumMap: enumMap,
		kind:    kind,
		value:   defaultValue,
	}
}

func (e *EnumFlag) Set(s string) error {
	if _, ok := e.enumMap[s]; !ok {
		return fmt.Errorf("must be one of %s", e.validValue()) // cobra will prepend "invalid argument \"asdf\" for \"-k, --kind\" flag:"
	}
	e.value = string(s)
	return nil
}

func (e *EnumFlag) Type() string {
	return e.kind
}

func (e *EnumFlag) String() string {
	return fmt.Sprintf("%v", e.value)
}

func (e *EnumFlag) validValue() string {
	keys := make([]string, 0, len(e.enumMap))
	for k := range e.enumMap {
		keys = append(keys, k)
	}
	return strings.Join(keys, ", ")
}

type cobraCompletionFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

func (e *EnumFlag) Completion() cobraCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		completion := make([]string, len(e.enumMap))
		for k, v := range e.enumMap {
			completion = append(completion, fmt.Sprintf("%s\t%s", k, v))
		}
		return completion, cobra.ShellCompDirectiveNoFileComp
	}
}

func (e *EnumFlag) EnableFlag(c *cobra.Command, name string, short string, def string, help string) {
	usage := fmt.Sprintf("%s (%s)", help, e.validValue())
	c.Flags().VarP(e, name, short, usage)
	c.RegisterFlagCompletionFunc(name, e.Completion())
}
