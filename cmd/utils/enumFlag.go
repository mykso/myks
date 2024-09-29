package utils

import (
	"fmt"

	"github.com/spf13/cobra"
)

type EnumFlag struct {
	enumMap map[string]string
	value   string
}

func NewEnumFlag(enumMap map[string]string) *EnumFlag {
	return &EnumFlag{
		enumMap: enumMap,
	}
}

func (e *EnumFlag) Set(s string) error {
	if _, ok := e.enumMap[s]; !ok {
		return fmt.Errorf("unknown value: %s", s)
	}
	e.value = string(s)
	return nil
}

func (e *EnumFlag) Type() string {
	return "enumFoo"
}

func (e *EnumFlag) String() string {
	return fmt.Sprintf("%v", e.value)
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
	c.Flags().StringP(name, short, def, help)
	c.RegisterFlagCompletionFunc(name, e.Completion())
}
