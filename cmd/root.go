// Package cmd provides the CLI commands for FoxRay.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/PinkyFrog0o0/foxray/internal/agent"
	"github.com/PinkyFrog0o0/foxray/internal/api"
	"github.com/PinkyFrog0o0/foxray/internal/auth"
	"github.com/PinkyFrog0o0/foxray/internal/config"
	"github.com/PinkyFrog0o0/foxray/internal/extension"
	"github.com/PinkyFrog0o0/foxray/internal/input"
	"github.com/PinkyFrog0o0/foxray/internal/mcp"
	"github.com/PinkyFrog0o0/foxray/internal/output"
	"github.com/PinkyFrog0o0/foxray/internal/prompt"
	"github.com/PinkyFrog0o0/foxray/internal/tools"
	"github.com/spf13/cobra"
)

var (
	version = "dev"

	prompt_             string
	model               string
	outputFormat        string
	files               []string
	timeout             time.Duration
	debug               bool
	rawOutput           bool
	acceptRawOutputRisk bool
	maxTurns            int
	yolo                bool
	sandbox             bool
	noAgent             bool
	apiKey              string
	backend_            string
	location            string
	apiURL              string
)

const (
	envAPIKey         = "FOXRAY_API_KEY"
	legacyEnvAPIKey   = "GMN_API_KEY"
	envModel          = "FOXRAY_MODEL"
	legacyEnvModel    = "GMN_MODEL"
	envBackend        = "FOXRAY_BACKEND"
	legacyEnvBackend  = "GMN_BACKEND"
	envLocation       = "FOXRAY_LOCATION"
	legacyEnvLocation = "GMN_LOCATION"
	envAPIURL         = "FOXRAY_API_URL"
	legacyEnvAPIURL   = "GMN_API_URL"
)

var rootCmd = &cobra.Command{
	Use:   "foxray [prompt]",
	Short: "FoxRay is a lightweight, non-interactive Gemini CLI fork",
	Long: `FoxRay is a lightweight reimplementation of Google's Gemini CLI
focused on non-interactive use cases.

By default, it reuses OAuth credentials from the official Gemini CLI (~/.gemini/).
Use -k/--api-key (or set FOXRAY_API_KEY) to use API key authentication instead.

Examples:
  foxray "Hello, world"
  foxray "Explain Go generics" -m gemini-2.5-pro
  foxray "Review this code" -f main.go -k YOUR_API_KEY
  foxray "Summarize" --backend vertex -k YOUR_API_KEY
  cat file.go | foxray "Review this code"
  foxray "Fix the tests" --yolo`,
	RunE:    run,
	Version: version,
	Args:    cobra.MaximumNArgs(1),
}

func init() {
	config.LoadDotEnv()

	rootCmd.Flags().StringVarP(&prompt_, "prompt", "p", "", "Prompt to send to Gemini (required)")
	rootCmd.Flags().StringVarP(&model, "model", "m", envOrFallback(envModel, legacyEnvModel, "gemini-2.5-flash"), "Model (default: $FOXRAY_MODEL, fallback: $GMN_MODEL)")
	rootCmd.Flags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: text, json, stream-json")
	rootCmd.Flags().StringArrayVarP(&files, "file", "f", nil, "Files to include in context")
	rootCmd.Flags().DurationVarP(&timeout, "timeout", "t", 5*time.Minute, "API timeout")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug output")
	rootCmd.Flags().BoolVar(&rawOutput, "raw-output", false, "Disable sanitization of model output (allow ANSI escape sequences)")
	rootCmd.Flags().BoolVar(&acceptRawOutputRisk, "accept-raw-output-risk", false, "Suppress security warning when using --raw-output")
	rootCmd.Flags().IntVar(&maxTurns, "max-turns", 25, "Maximum agent loop turns")
	rootCmd.Flags().BoolVar(&yolo, "yolo", false, "Auto-approve shell commands (no confirmation)")
	rootCmd.Flags().BoolVar(&sandbox, "sandbox", false, "Restrict file writes to working directory")
	rootCmd.Flags().BoolVar(&noAgent, "no-agent", false, "Disable agent mode (single-turn, no tools)")
	rootCmd.Flags().StringVarP(&apiKey, "api-key", "k", firstEnv(envAPIKey, legacyEnvAPIKey), "API key (default: $FOXRAY_API_KEY, fallback: $GMN_API_KEY)")
	rootCmd.Flags().StringVar(&backend_, "backend", envOrFallback(envBackend, legacyEnvBackend, "gemini"), "API backend: gemini, vertex (default: $FOXRAY_BACKEND, fallback: $GMN_BACKEND)")
	rootCmd.Flags().StringVar(&location, "location", firstEnv(envLocation, legacyEnvLocation), "Vertex AI location (default: $FOXRAY_LOCATION, fallback: $GMN_LOCATION)")
	rootCmd.Flags().StringVar(&apiURL, "api-url", firstEnv(envAPIURL, legacyEnvAPIURL), "Custom API base URL (default: $FOXRAY_API_URL, fallback: $GMN_API_URL)")
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

func envOrFallback(primary, legacy, fallback string) string {
	if v := firstEnv(primary, legacy); v != "" {
		return v
	}
	return fallback
}

// SetVersion sets the version string
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

func run(cmd *cobra.Command, args []string) error {
	// Handle positional argument as prompt
	if len(args) > 0 {
		prompt_ = args[0]
	}
	// Setup context with timeout and signal handling
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Upstream ref: 799007354 - sanitize ANSI escape sequences in non-interactive output
	sanitize := !rawOutput && !acceptRawOutputRisk
	if rawOutput && !acceptRawOutputRisk && outputFormat == "text" {
		fmt.Fprintln(os.Stderr, "[WARNING] --raw-output is enabled. Model output is not sanitized and may contain harmful ANSI sequences (e.g. for phishing or command injection). Use --accept-raw-output-risk to suppress this warning.")
	}

	// Create formatter
	formatter, err := output.NewFormatter(outputFormat, os.Stdout, os.Stderr, sanitize)
	if err != nil {
		return err
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		formatter.WriteError(fmt.Errorf("failed to load config: %w", err))
		return err
	}

	// Prepare input
	inputText, err := input.PrepareInput(prompt_, files)
	if err != nil {
		formatter.WriteError(err)
		return err
	}

	if inputText == "" {
		err := fmt.Errorf("no input provided")
		formatter.WriteError(err)
		return err
	}

	// Create API client (API key or OAuth)
	var apiClient *api.Client
	var projectID string

	if apiKey != "" {
		// API key mode: no OAuth, no Code Assist project
		var backendType api.Backend
		switch backend_ {
		case "gemini":
			backendType = api.BackendGeminiAPI
		case "vertex":
			backendType = api.BackendVertexAI
		default:
			err := fmt.Errorf("unknown backend %q: use 'gemini' or 'vertex'", backend_)
			formatter.WriteError(err)
			return err
		}
		apiClient = api.NewAPIKeyClient(apiKey, backendType, location, apiURL)
		if debug {
			fmt.Fprintf(os.Stderr, "Using API key auth (backend=%s, api-url=%s)\n", backend_, apiClient.BaseURL())
		}
	} else {
		// OAuth mode (existing behavior)
		authMgr, err := auth.NewManager()
		if err != nil {
			formatter.WriteError(fmt.Errorf("failed to initialize auth: %w", err))
			return err
		}

		creds, err := authMgr.LoadCredentials()
		if err != nil {
			formatter.WriteError(err)
			return err
		}

		if creds.IsExpired() {
			if debug {
				fmt.Fprintln(os.Stderr, "Token expired, refreshing...")
			}
			creds, err = authMgr.RefreshToken(creds)
			if err != nil {
				formatter.WriteError(err)
				return err
			}
		}

		httpClient := authMgr.HTTPClient(creds)
		apiClient = api.NewClient(httpClient)

		// Load Code Assist project ID
		cachedState, _ := config.LoadCachedState()
		projectID = cachedState.ProjectID

		if projectID == "" {
			if debug {
				fmt.Fprintln(os.Stderr, "Loading Code Assist status...")
			}
			loadResp, err := apiClient.LoadCodeAssist(ctx)
			if err != nil {
				formatter.WriteError(fmt.Errorf("failed to load Code Assist: %w", err))
				return err
			}
			projectID = loadResp.CloudAICompanionProject

			// Upstream ref: f4e73191d - fix tier eligibility for unlicensed users
			if projectID == "" {
				if len(loadResp.IneligibleTiers) > 0 {
					var reasons []string
					for _, tier := range loadResp.IneligibleTiers {
						if tier.ReasonMessage != "" {
							reasons = append(reasons, tier.ReasonMessage)
						}
					}
					if len(reasons) > 0 {
						errMsg := fmt.Errorf("unable to use Gemini: %s", strings.Join(reasons, ", "))
						formatter.WriteError(errMsg)
						return errMsg
					}
				}
				errMsg := fmt.Errorf("unable to use Gemini: no project ID available. Please run 'gemini' to set up your account")
				formatter.WriteError(errMsg)
				return errMsg
			}

			userTier := ""
			if loadResp.CurrentTier != nil {
				userTier = loadResp.CurrentTier.ID
			}
			_ = config.SaveCachedState(&config.CachedState{
				ProjectID: projectID,
				UserTier:  userTier,
			})

			if debug {
				fmt.Fprintf(os.Stderr, "Project ID: %s (cached)\n", projectID)
				if loadResp.CurrentTier != nil {
					fmt.Fprintf(os.Stderr, "Tier: %s\n", loadResp.CurrentTier.ID)
				}
			}
		} else if debug {
			fmt.Fprintf(os.Stderr, "Using cached Project ID: %s\n", projectID)
		}
	}

	// Generate a simple user prompt ID
	userPromptID := fmt.Sprintf("foxray-%d", time.Now().UnixNano())

	// Get working directory
	workDir, _ := os.Getwd()

	// Build request (Code Assist API format)
	req := &api.GenerateRequest{
		Model:        model,
		Project:      projectID,
		UserPromptID: userPromptID,
		Request: api.InnerRequest{
			Contents: []api.Content{{
				Role:  "user",
				Parts: []api.Part{{Text: inputText}},
			}},
			Config: api.GenerationConfig{
				Temperature:     1.0,
				TopP:            0.95,
				MaxOutputTokens: 65536,
			},
		},
	}

	// --- Agent mode ---
	if !noAgent {
		// Create web search callback
		webSearchFn := func(ctx context.Context, query string) (string, []tools.WebSource, error) {
			resp, err := apiClient.WebSearch(ctx, projectID, model, query)
			if err != nil {
				return "", nil, err
			}
			// Extract text from response
			var text string
			var sources []tools.WebSource
			if len(resp.Response.Candidates) > 0 {
				cand := resp.Response.Candidates[0]
				for _, part := range cand.Content.Parts {
					text += part.Text
				}
				// Extract grounding sources
				if cand.GroundingMetadata != nil {
					for _, chunk := range cand.GroundingMetadata.GroundingChunks {
						if chunk.Web != nil {
							sources = append(sources, tools.WebSource{
								Title: chunk.Web.Title,
								URI:   chunk.Web.URI,
							})
						}
					}
				}
			}
			return text, sources, nil
		}

		// Load extensions and merge MCP servers
		extensions, extErr := extension.LoadAll(workDir)
		if extErr != nil && debug {
			fmt.Fprintf(os.Stderr, "[ext] failed to load extensions: %v\n", extErr)
		}
		if cfg != nil {
			for _, ext := range extensions {
				for serverName, serverCfg := range ext.MCPServers {
					if _, exists := cfg.MCPServers[serverName]; !exists {
						cfg.MCPServers[serverName] = serverCfg
						if debug {
							fmt.Fprintf(os.Stderr, "[ext] loaded MCP server %q from extension %q\n", serverName, ext.Name)
						}
					} else if debug {
						fmt.Fprintf(os.Stderr, "[ext] MCP server %q from extension %q skipped (already configured)\n", serverName, ext.Name)
					}
				}
			}
		}

		// Create tool registry
		registry := tools.NewRegistry(tools.RegistryOptions{
			WorkDir:     workDir,
			AutoApprove: yolo,
			Sandbox:     sandbox,
			Debug:       debug,
			WebSearch:   webSearchFn,
		})

		// Initialize MCP clients and register tools
		mcpClients := make(agent.MCPClients)
		var mcpDecls []api.FunctionDecl

		if cfg != nil {
			for serverName, serverCfg := range cfg.MCPServers {
				if serverCfg.Command == "" {
					continue // Skip HTTP/SSE (not yet supported)
				}
				client, err := mcp.NewClient(serverCfg.Command, serverCfg.Args, serverCfg.Env, serverCfg.CWD)
				if err != nil {
					if debug {
						fmt.Fprintf(os.Stderr, "[mcp] failed to create client for %s: %v\n", serverName, err)
					}
					continue
				}
				if err := client.Initialize(ctx); err != nil {
					if debug {
						fmt.Fprintf(os.Stderr, "[mcp] failed to initialize %s: %v\n", serverName, err)
					}
					client.Close()
					continue
				}
				mcpClients[serverName] = client
				defer client.Close()

				for _, tool := range client.Tools {
					prefixedName := serverName + "__" + tool.Name
					registry.RegisterMCPTool(serverName, prefixedName)
					mcpDecls = append(mcpDecls, api.FunctionDecl{
						Name:        prefixedName,
						Description: tool.Description,
						Parameters:  json.RawMessage(tool.InputSchema),
					})
					if debug {
						fmt.Fprintf(os.Stderr, "[mcp] registered tool: %s\n", prefixedName)
					}
				}
			}
		}

		// Collect extension context files
		var extContextFiles []string
		for _, ext := range extensions {
			extContextFiles = append(extContextFiles, ext.ContextFiles...)
		}

		// Set system instruction
		req.Request.SystemInstruction = prompt.BuildSystemInstruction(prompt.Options{
			WorkDir:           workDir,
			ExtensionContexts: extContextFiles,
		})

		// Set tools
		allDecls := registry.AllDeclarations()
		allDecls = append(allDecls, mcpDecls...)
		req.Request.Tools = []api.Tool{{FunctionDeclarations: allDecls}}

		if debug {
			fmt.Fprintf(os.Stderr, "[agent] %d built-in tools, %d MCP tools registered\n",
				len(registry.AllDeclarations()), len(mcpDecls))
		}

		// Run agent loop
		streaming := outputFormat != "json"
		loop := agent.NewLoop(apiClient, registry, mcpClients, formatter, agent.Config{
			MaxTurns:  maxTurns,
			Streaming: streaming,
			Debug:     debug,
		})

		if err := loop.Run(ctx, req); err != nil {
			formatter.WriteError(err)
			return err
		}
		return nil
	}

	// --- Legacy single-turn mode (--no-agent) ---
	switch outputFormat {
	case "json":
		return runNonStreaming(ctx, apiClient, req, formatter)
	default:
		return runStreaming(ctx, apiClient, req, formatter)
	}
}

func runNonStreaming(ctx context.Context, client *api.Client, req *api.GenerateRequest, formatter output.Formatter) error {
	resp, err := client.Generate(ctx, req)
	if err != nil {
		formatter.WriteError(err)
		return err
	}
	return formatter.WriteResponse(resp)
}

func runStreaming(ctx context.Context, client *api.Client, req *api.GenerateRequest, formatter output.Formatter) error {
	stream, err := client.GenerateStream(ctx, req)
	if err != nil {
		formatter.WriteError(err)
		return err
	}

	for event := range stream {
		if event.Type == "error" {
			formatter.WriteError(fmt.Errorf(event.Error))
			return fmt.Errorf(event.Error)
		}
		if err := formatter.WriteStreamEvent(&event); err != nil {
			return err
		}
	}

	return nil
}
