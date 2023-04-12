// SPDX-License-Identifier: MIT
// Copyright (c) Furkan Türkal

package main

import (
	"container/ring"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/avast/retry-go"
	"github.com/slack-go/slack"
	"golang.org/x/time/rate"
)

func main() {
	flagPort := flag.Int("port", 8080, "port to listen on")
	flagBuffer := flag.Int("buffer", 1000, "falco log buffer size")
	flagQPS := flag.Int("qps", 10, "queries per HOUR to OpenAI and Slack")
	flagTemplateFile := flag.String("template-file", "", "path custom template file to use for the ChatGPT")
	flagMinPriority := flag.String("min-priority", "warning", "minimum priority to analyse")
	flagGPTModel := flag.String("model", "gpt-3.5-turbo", "Backend AI model")
	flag.Parse()

	token := os.Getenv("OPENAI_TOKEN")
	if token == "" {
		log.Fatal("$OPENAI_TOKEN is not set")
	}

	webhook := os.Getenv("SLACK_WEBHOOK_URL")
	if webhook == "" {
		log.Fatal("$SLACK_WEBHOOK_URL is not set")
	}

	// client is the OpenAI client for the ChatGPT.
	client, err := NewOpenAIClient(os.Getenv("OPENAI_TOKEN"), *flagGPTModel, *flagTemplateFile)
	if err != nil {
		log.Fatal(err)
	}

	// Inflight queue is a ring buffer to prevent the consecutive requests to the API.
	// bufferSize is the size of the inflight ring buffer.
	inflightQueue := ring.New(*flagBuffer)

	// Set up a rate limiter in order to not exceed the API rate limit.
	limiter := rate.NewLimiter(rate.Every(time.Hour), *flagQPS)

	go processQueue(client, inflightQueue, limiter)

	http.HandleFunc("/", handler(inflightQueue, *flagMinPriority))

	log.Printf("Listening on port %d", *flagPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *flagPort), nil))
}

func handler(queue *ring.Ring, minPriority string) func(http.ResponseWriter, *http.Request) {
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

			var payload FalcoPayload
			if err := json.Unmarshal(body, &payload); err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				w.Write([]byte(err.Error()))
				return
			}
			payload.Raw = string(body)

			// Put the payload onto the inflight queue for processing
			// if the priority is high enough.
			if ToPriority(payload.Priority) >= ToPriority(minPriority) {
				queue.Value = payload
				queue = queue.Next()

				// We return 200 OK to the client, but this does NOT
				// mean that the request has been processed.
				w.WriteHeader(http.StatusCreated)
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// processQueue is a goroutine that processes entries from the queue.
func processQueue(client *OpenAIClient, queue *ring.Ring, limiter *rate.Limiter) {
	for {
		entry := queue.Value
		if entry == nil {
			// If there are no events to process, cool down for a bit.
			time.Sleep(1 * time.Second)
			continue
		}

		payload, ok := entry.(FalcoPayload)
		if !ok {
			log.Println("unprocessable entry", entry)
			continue
		}

		if err := process(context.Background(), client, limiter, payload); err != nil {
			continue
		}

		// Remove the payload from the queue if it has been processed successfully.
		queue.Value = nil
		queue = queue.Next()
	}
}

// process processes the given payload.
func process(ctx context.Context, client *OpenAIClient, limiter *rate.Limiter, payload FalcoPayload) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Wait for the rate limiter.
	if err := limiter.Wait(ctx); err != nil {
		return err
	}

	// Print the payload to stdout.
	log.Printf("processing payload: %s\n", payload)

	if err := analyze(ctx, client, payload); err != nil {
		log.Println(fmt.Errorf("process: %w", err))
		return err
	}

	return nil
}

// analyze analyzes the given payload with ChatGPT and posts the response to Slack.
func analyze(ctx context.Context, client *OpenAIClient, payload FalcoPayload) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var response string

	if err := do(func() error {
		// TODO: Generate unique ID for the payload and upsert it to the in-mem cache to avoid unnecessary API calls.
		resp, err := client.GetCompletion(ctx, payload.Raw)
		if err != nil {
			return err
		}
		response = resp
		return nil
	}); err != nil {
		return fmt.Errorf("get completion: %w", err)
	}

	if err := do(func() error {
		return slack.PostWebhookContext(ctx, os.Getenv("SLACK_WEBHOOK_URL"), newSlackWebhookMessage(payload, response))
	}); err != nil {
		return fmt.Errorf("post to Slack: %w", err)
	}

	return nil
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
