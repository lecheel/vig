package wig

type CmdDefinition struct {
	Desc       string
	Fn         interface{}
	Repeatable bool
}

var AllCommands = map[string]CmdDefinition{}
