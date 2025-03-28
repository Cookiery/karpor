// Copyright The Karpor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ai

import (
	"context"
	"errors"
	"io"

	"github.com/sashabaranov/go-openai"
)

type OpenAIClient struct {
	client      *openai.Client
	model       string
	temperature float32
	topP        float32
}

func (c *OpenAIClient) Configure(cfg AIConfig) error {
	defaultConfig := openai.DefaultConfig(cfg.AuthToken)
	if cfg.BaseURL != "" {
		defaultConfig.BaseURL = cfg.BaseURL
	}

	if cfg.ProxyEnabled {
		defaultConfig.HTTPClient.Transport = GetProxyHTTPClient(cfg)
	}

	client := openai.NewClientWithConfig(defaultConfig)
	if client == nil {
		return errors.New("error creating OpenAI client")
	}

	c.client = client
	c.model = cfg.Model
	c.temperature = cfg.Temperature
	c.topP = cfg.TopP
	return nil
}

func (c *OpenAIClient) Generate(ctx context.Context, prompt string) (string, error) {
	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: c.temperature,
		TopP:        c.topP,
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("no completion choices returned from response")
	}
	return resp.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) GenerateStream(ctx context.Context, prompt string) (<-chan string, error) {
	// Create chat completion stream with streaming enabled
	stream, err := c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
		Temperature: c.temperature,
		TopP:        c.topP,
		Stream:      true,
	})
	if err != nil {
		return nil, err
	}

	// Create buffered channel for response chunks
	resultChan := make(chan string, 100)

	// Start goroutine to handle streaming response
	go func() {
		defer close(resultChan)
		defer stream.Close()

		for {
			response, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				// Send error as a special message
				resultChan <- "ERROR: " + err.Error()
				return
			}

			// Send non-empty content chunks
			if len(response.Choices) > 0 {
				chunk := response.Choices[0].Delta.Content
				if chunk != "" {
					resultChan <- chunk
				}
			}
		}
	}()

	return resultChan, nil
}
