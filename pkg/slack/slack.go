// SPDX-License-Identifier: MIT
// Copyright (c) Furkan Türkal

package slack

import (
	"fmt"
	"sort"

	"github.com/Dentrax/falco-gpt/pkg/model"
	slacksdk "github.com/slack-go/slack"
)

const (
	Rule                 = "rule"
	Priority             = "priority"
	Source               = "source"
	Time                 = "time"
	Hostname             = "hostname"
	DefaultFooter string = "https://github.com/Dentrax/falco-gpt"
)

const (
	username = "Falco-GPT"
	iconURL  = "https://raw.githubusercontent.com/cncf/artwork/b4216a91b2c1976c2e7fd25f62ee4d3b2126b4a6/projects/falco/icon/color/falco-icon-color.png"
)

var (
	client *Client
)

// Client is the client for Slack API.
type Client struct {
	client  *slacksdk.Client
	channel string
}

func init() {
	client = new(Client)
}

func NewClient(channel, token string) (*Client, error) {
	client.channel = channel
	client.client = slacksdk.New(token)

	return client, nil
}

func GetClient(token string) *Client {
	return client
}

func (client *Client) SendMessage(text string, attachment *slacksdk.Attachment, thread string) (string, error) {
	options := make([]slacksdk.MsgOption, 0)
	if text != "" {
		options = append(options, slacksdk.MsgOptionText(text, false))
	}
	if attachment != nil {
		options = append(options, slacksdk.MsgOptionAttachments(*attachment))
	}
	if thread != "" {
		options = append(options, slacksdk.MsgOptionTS(thread))
	}
	options = append(options, slacksdk.MsgOptionUsername(username))
	options = append(options, slacksdk.MsgOptionIconURL(iconURL))

	if client.client == nil {
		return "", fmt.Errorf("error with Slack client")
	}

	_, threadTS, err := client.client.PostMessage(client.channel, options...)
	return threadTS, err
}

func NewAttachment(falcopayload model.FalcoPayload) *slacksdk.Attachment {
	var (
		attachment slacksdk.Attachment
		fields     []slacksdk.AttachmentField
		field      slacksdk.AttachmentField
	)
	field.Title = Rule
	field.Value = falcopayload.Rule
	field.Short = false
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

	attachment.Color = model.GetPriorityColor(falcopayload.Priority)

	return &attachment
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
