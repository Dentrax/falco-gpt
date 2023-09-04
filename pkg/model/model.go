// SPDX-License-Identifier: MIT
// Copyright (c) Furkan Türkal

package model

import (
	"strings"
	"time"
)

// Ref: https://github.com/falcosecurity/falcosidekick/blob/master/types

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

// Colors
const (
	PaleCyan  string = "#ccfff2"
	Yellow    string = "#ffc700"
	Red       string = "#e20b0b"
	LigthBlue string = "#68c2ff"
	Lightcyan string = "#5bffb5"
	Orange    string = "#ff5400"
)

// FalcoPayload is a struct to map falco event json
type FalcoPayload struct {
	Raw          string                 `json:"raw,omitempty"`
	ThreadTS     string                 `json:"thread_ts,omitempty"`
	Channel      string                 `json:"channel,omitempty"`
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

func GetPriorityColor(p string) string {
	switch strings.ToLower(p) {
	case "emergency":
		return Red
	case "alert":
		return Orange
	case "critical":
		return Orange
	case "error":
		return Red
	case "warning":
		return Yellow
	case "notice":
		return Lightcyan
	case "informational":
		return LigthBlue
	case "debug":
		return PaleCyan
	default:
		return PaleCyan
	}
}
