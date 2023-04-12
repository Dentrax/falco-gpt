// SPDX-License-Identifier: MIT
// Copyright (c) Furkan Türkal

package main

import (
	"strings"
	"time"
)

// Ref: https://github.com/falcosecurity/falcosidekick/blob/master/types

const (
	Rule                 = "rule"
	Priority             = "priority"
	Source               = "source"
	Time                 = "time"
	Hostname             = "hostname"
	DefaultFooter string = "https://github.com/Dentrax/falco-gpt"
)

// FalcoPayload is a struct to map falco event json
type FalcoPayload struct {
	Raw          string
	UUID         string                 `json:"uuid,omitempty"`
	Output       string                 `json:"output"`
	Priority     string                 `json:"priority"`
	Rule         string                 `json:"rule"`
	Time         time.Time              `json:"time"`
	OutputFields map[string]interface{} `json:"output_fields"`
	Source       string                 `json:"source"`
	Tags         []string               `json:"tags,omitempty"`
	Hostname     string                 `json:"hostname,omitempty"`
}

type PriorityType int

const (
	Default = iota // ""
	Debug
	Informational
	Notice
	Warning
	Error
	Critical
	Alert
	Emergency
)

func ToPriority(p string) PriorityType {
	switch strings.ToLower(p) {
	case "emergency":
		return Emergency
	case "alert":
		return Alert
	case "critical":
		return Critical
	case "error":
		return Error
	case "warning":
		return Warning
	case "notice":
		return Notice
	case "informational":
		return Informational
	case "info":
		return Informational
	case "debug":
		return Debug
	default:
		return Default
	}
}
