// SPDX-License-Identifier: MIT
// Copyright (c) Furkan Türkal

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Dentrax/falco-gpt/pkg/model"
	"github.com/Dentrax/falco-gpt/pkg/openai"
	"github.com/Dentrax/falco-gpt/pkg/slack"
	"github.com/avast/retry-go"
	"golang.org/x/time/rate"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natsgo "github.com/nats-io/nats.go"
)

var (
	natsClient   *natsgo.Conn
	limiter      *rate.Limiter
	slackClient  *slack.Client
	openaiClient *openai.Client
	minPriority  string
	ctx          context.Context
	older        float64
)

func init() {
	ctx = context.Background()
}

func main() {
	flagPort := flag.Int("port", 8080, "port to listen on")
	flagChannel := flag.String("channel", "", "Slack channel")
	flagQPH := flag.Int("qph", 10, "max queries per HOUR to OpenAI")
	flagMinPriority := flag.String("min-priority", "warning", "minimum priority to analyse")
	flagGPTModel := flag.String("model", "gpt-3.5-turbo", "Backend AI model")
	flatIgnoreOlder := flag.Int("ignore-older", 1, "Ignore events in queue older than X hour(s)")
	flagTemplateFile := flag.String("template-file", "", "path custom template file to use for the ChatGPT")
	flag.Parse()

	minPriority = *flagMinPriority

	if *flagChannel == "" {
		log.Fatal("Please specify slack channel")
	}

	slackToken := os.Getenv("SLACK_TOKEN")
	if slackToken == "" {
		log.Fatal("SLACK_TOKEN is not set")
	}
	slackClient, _ = slack.NewClient(*flagChannel, slackToken)

	openaiToken := os.Getenv("OPENAI_TOKEN")
	if openaiToken == "" {
		log.Fatal("OPENAI_TOKEN is not set")
	}

	older = float64(*flatIgnoreOlder)

	// client is the OpenAI client for the ChatGPT.
	var err error
	openaiClient, err = openai.NewClient(openaiToken, *flagGPTModel, *flagTemplateFile)
	if err != nil {
		log.Fatal(err)
	}

	// Set up a rate limiter in order to not exceed the API rate limit.
	limiter = rate.NewLimiter(rate.Every(time.Hour), *flagQPH)

	// Initialize nats server with options
	ns, err := natsserver.NewServer(&natsserver.Options{})
	if err != nil {
		log.Fatal(err)
	}
	defer ns.Shutdown()
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		log.Fatal("NATS server not ready")
	}
	natsClient, err = natsgo.Connect(ns.ClientURL())
	if err != nil {
		log.Fatal(err)
	}
	defer natsClient.Close()

	go subscribeSlackQueue()
	go subscribeGPTQueue()

	http.HandleFunc("/", handler())

	log.Printf("Listening on port %d", *flagPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *flagPort), nil))
}

func handler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Body != nil {
			defer r.Body.Close()

			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				w.Write([]byte(err.Error()))
				return
			}

			if body == nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				w.Write([]byte("empty body"))
				return
			}

			var payload model.FalcoPayload
			if err := json.Unmarshal(body, &payload); err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				w.Write([]byte(err.Error()))
				return
			}
			payload.Raw = string(body)

			if payload.Source == "" {
				payload.Source = "syscalls"
			}

			// Put the payload onto the inflight queue for processing
			// if the priority is high enough.
			if model.ToPriority(payload.Priority) >= model.ToPriority(minPriority) {
				msg, err := json.Marshal(payload)
				if err != nil {
					w.WriteHeader(http.StatusUnprocessableEntity)
					w.Write([]byte(err.Error()))
					return
				}
				if err := do(func() error {
					return natsClient.Publish("slack", msg)
				}); err != nil {
					w.WriteHeader(http.StatusServiceUnavailable)
					w.Write([]byte(err.Error()))
					return
				}
				w.WriteHeader(http.StatusCreated)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// processGPTQueue is a goroutine that processes entries from the queue for ChatGPT.
func subscribeGPTQueue() {
	natsClient.Subscribe("gpt", func(msg *natsgo.Msg) {
		// Print message data
		// Wait for the rate limiter.
		var payload model.FalcoPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Println(err)
			return
		}
		if time.Since(payload.Time).Hours() > older {
			return
		}

		if err := limiter.Wait(ctx); err != nil {
			return
		}

		// Print the payload to stdout.
		truncatedPayload := payload
		truncatedPayload.Raw = ""
		truncatedPayload.ThreadTS = ""
		truncatedPayload.Channel = ""
		p, _ := json.Marshal(truncatedPayload)
		log.Printf("processing payload: %#v\n", string(p))
		if response, err := analyze(ctx, payload.Raw); err != nil {
			log.Println(fmt.Errorf("error process: %w", err))
			return
		} else {
			if err := do(func() error {
				_, err := slackClient.SendMessage(response, nil, payload.ThreadTS)
				if err != nil {
					return err
				}
				return nil
			}); err != nil {
				log.Println(fmt.Errorf("slack error: %w", err))
				return
			}

		}
	})
}

// processSlackQueue is a goroutine that processes entries from the queue for Slack.
func subscribeSlackQueue() {
	natsClient.Subscribe("slack", func(msg *natsgo.Msg) {
		var payload model.FalcoPayload
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Println(fmt.Errorf("error process: %w", err))
			return
		}
		if err := do(func() error {
			attachment := slack.NewAttachment(payload)
			var err error
			payload.ThreadTS, err = slackClient.SendMessage(payload.Output, attachment, "")
			if err != nil {
				return err
			}
			return nil
		}); err != nil {
			log.Println(fmt.Errorf("slack error: %w", err))
			return
		}
		p, err := json.Marshal(payload)
		if err != nil {
			log.Println(fmt.Errorf("error process: %w", err))
			return
		}
		if err := do(func() error {
			return natsClient.Publish("gpt", p)
		}); err != nil {
			log.Println(fmt.Errorf("nats error: %w", err))
			return
		}
	})
}

// analyze analyzes the given payload with ChatGPT and posts the response to Slack.
func analyze(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var response string

	if err := do(func() error {
		var err error
		response, err = openaiClient.GetCompletion(ctx, prompt)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("get completion: %w", err)
	}

	return response, nil
}

// do is a wrapper around retry.Do that retry the given function.
func do(doer func() error) error {
	const maxRetries = 3
	if err := retry.Do(
		doer,
		retry.Attempts(maxRetries),
		retry.DelayType(retry.BackOffDelay),
		retry.Delay(10*time.Second),
		retry.OnRetry(func(n uint, err error) {
			log.Println("retrying after error:", err)
		}),
	); err != nil {
		return fmt.Errorf("do failed after %d retries: %w", maxRetries, err)
	}
	return nil
}
