package targ_test

import (
	"strings"
)

// --- Args struct types for Target functions ---

type CustomFlagNameArgs struct {
	User string `targ:"name=user_name"`
}

type CustomTypeFlagsArgs struct {
	Name TextValue   `targ:"flag"`
	Nick SetterValue `targ:"flag"`
}

type DefaultEnvFlagsArgs struct {
	Name string `targ:"default=Alice,env=TEST_DEFAULT_NAME_FLAG"`
}

type DefaultFlagsArgs struct {
	Name    string `targ:"default=Alice"`
	Count   int    `targ:"default=42"`
	Enabled bool   `targ:"default=true"`
}

// --- Custom unmarshaler types (these stay - they're not commands) ---

type SetterValue struct {
	Value string
}

func (s *SetterValue) Set(value string) error {
	s.Value = value + "!"
	return nil
}

type ShortBoolFlagsArgs struct {
	Verbose bool `targ:"flag,short=v"`
	Force   bool `targ:"flag,short=f"`
}

type ShortFlagsArgs struct {
	Name string `targ:"flag,short=n"`
	Age  int    `targ:"flag,short=a"`
}

type ShortMixedFlagsArgs struct {
	Verbose bool   `targ:"flag,short=v"`
	Name    string `targ:"flag,short=n"`
}

type TextValue struct {
	Value string
}

func (tv *TextValue) UnmarshalText(text []byte) error {
	tv.Value = strings.ToUpper(string(text))
	return nil
}
