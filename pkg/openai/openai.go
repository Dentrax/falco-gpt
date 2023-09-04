// SPDX-License-Identifier: MIT
// Copyright (c) Furkan Türkal

package openai

import (
	"context"
	"fmt"
	"os"

	openaisdk "github.com/sashabaranov/go-openai"
)

const (
	defaultPromptTemplate = `You are a very intelligent audit log expert. All the input I provide you is belong to Falco security tool from Sysdig. You are talking to a Linux expert.

Read and considering the following audit log input, create 2 root bullet points with "Problem:" and "Remediation:" prefix, respectively. And nothing else. Each root must have only one bullet point. Wrap important files, keywords and commands with backticks.

* Simplify the audit log and create an one liner simple message. Append the critical information from the log. Use the important keywords in the message.
* Provide possible scenarios for remediation. Talk technical as possible. Prefer to put remediation commands, bash scripts, etc. in the first place. Show the way of solution.

Your JSON input is:

%s
`
)

var (
	client *Client
)

// Client is the client for OpenAI API.
type Client struct {
	client       *openaisdk.Client
	model        string
	templateFile string
}

func init() {
	client = new(Client)
}

// NewClient initializes the OpenAI client.
func NewClient(token, model, templateFile string) (*Client, error) {
	c := openaisdk.NewClient(token)
	if c == nil {
		return nil, fmt.Errorf("error creating OpenAI client")
	}

	client.client = c
	client.model = model
	client.templateFile = getTemplate(templateFile)

	return client, nil
}

// GetClient retrieves the OpenAI client
func GetClient() *Client {
	return client
}

// getTemplate returns the template for the prompt. If the template file is not
// specified, the default template is returned.
func getTemplate(f string) string {
	// open file and sprintf
	if f != "" {
		bytes, err := os.ReadFile(f)
		if err != nil {
			panic(fmt.Errorf("read template file: %w", err))
		}
		return string(bytes)
	}
	return defaultPromptTemplate
}

// GetCompletion returns the completion from OpenAI API for the given prompt.
func (c *Client) GetCompletion(ctx context.Context, prompt string) (string, error) {
	resp, err := c.client.CreateChatCompletion(ctx, openaisdk.ChatCompletionRequest{
		Model:       c.model,
		Temperature: 0, // To make the output deterministic.
		Messages: []openaisdk.ChatCompletionMessage{
			{
				Role:    "user",
				Content: fmt.Sprintf(c.templateFile, prompt),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("create chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response choices from OpenAI")
	}
	return resp.Choices[0].Message.Content, nil
}
