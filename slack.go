// SPDX-License-Identifier: MIT
// Copyright (c) Furkan Türkal

package main

import (
	"sort"
	"strings"

	"github.com/slack-go/slack"
)

func newSlackWebhookMessage(falcopayload FalcoPayload, openAIMessage string) *slack.WebhookMessage {
	var (
		attachment slack.Attachment
		fields     []slack.AttachmentField
		field      slack.AttachmentField
	)
	field.Title = Rule
	field.Value = falcopayload.Rule
	field.Short = true
	fields = append(fields, field)
	field.Title = Priority
	field.Value = falcopayload.Priority
	field.Short = true
	fields = append(fields, field)
	field.Title = Source
	field.Value = falcopayload.Source
	field.Short = true
	fields = append(fields, field)
	if falcopayload.Hostname != "" {
		field.Title = Hostname
		field.Value = falcopayload.Hostname
		field.Short = true
		fields = append(fields, field)
	}
	for _, i := range getSortedStringKeys(falcopayload.OutputFields) {
		field.Title = i
		field.Value = falcopayload.OutputFields[i].(string)
		if len([]rune(falcopayload.OutputFields[i].(string))) < 36 {
			field.Short = true
		} else {
			field.Short = false
		}
		fields = append(fields, field)
	}
	field.Title = Time
	field.Short = false
	field.Value = falcopayload.Time.String()
	fields = append(fields, field)

	attachment.Footer = DefaultFooter
	attachment.Fields = fields

	var color string
	switch strings.ToLower(falcopayload.Priority) {
	case "emergency":
		color = "red"
	case "alert":
		color = "orange"
	case "critical":
		color = "orange"
	case "error":
		color = "red"
	case "warning":
		color = "yellow"
	case "notice":
		color = "lightcyan"
	case "informational":
		color = "ligthblue"
	case "debug":
		color = "palecyan"
	}
	attachment.Color = color

	return &slack.WebhookMessage{
		Text:     openAIMessage,
		Username: "falco-gpt",
		IconURL:  "https://raw.githubusercontent.com/cncf/artwork/b4216a91b2c1976c2e7fd25f62ee4d3b2126b4a6/projects/falco/icon/color/falco-icon-color.png",
		Attachments: []slack.Attachment{
			attachment,
		},
	}
}

func getSortedStringKeys(m map[string]interface{}) []string {
	var keys []string
	for i, j := range m {
		switch j.(type) {
		case string:
			keys = append(keys, i)
		default:
			continue
		}
	}
	sort.Strings(keys)
	return keys
}
