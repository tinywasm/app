package app

import (
	"context"
	"net"
	"os"
	"sync"
	"time"

	"github.com/tinywasm/mcp"
	"github.com/tinywasm/sse"
)

// runDaemon starts the global MCP daemon on port 3030
func runDaemon(cfg BootstrapConfig) {
	logger := cfg.Logger
	if logger == nil {
		logger = func(messages ...any) {}
	}

	logger("Starting TinyWASM Global MCP Daemon on port 3030...")

	mcpPort := "3030"
	if p := os.Getenv("TINYWASM_MCP_PORT"); p != "" {
		mcpPort = p
	}

	mcpConfig := mcp.Config{
		Port:          mcpPort,
		ServerName:    "TinyWasm - Global MCP Server",
		ServerVersion: "1.0.0",
		AppName:       cfg.AppName,
		AppVersion:    cfg.Version,
	}

	// Create an empty TUI stub for the daemon if not provided
	var ui TuiInterface
	exitChan := make(chan bool)
	if cfg.TuiFactory != nil {
		ui = cfg.TuiFactory(exitChan, false, "") // daemon has no TUI client, no SSE needed
	} else {
		ui = NewHeadlessTUI(logger)
	}

	// Create SSE server (tinywasm/sse) and inject into Handler (mcp.SSEHub interface)
	tinySSE := sse.New(&sse.Config{})
	sseHub := tinySSE.Server(&sse.ServerConfig{
		ChannelProvider:     &logChannelProvider{},
		ClientChannelBuffer: 256,
		HistoryReplayBuffer: 100,
		ReplayAllOnConnect:  true,
	})

	// Define the daemon tool provider that controls the project lifecycles
	dtp := newDaemonToolProvider(cfg, logger)

	// Create MCP handler (pure JSON-RPC, no HTTP server)
	toolProviders := append(cfg.McpToolHandlers, dtp)
	mcpHandler := mcp.NewHandler(mcpConfig, toolProviders)
	mcpHandler.SetLog(logger)
	mcpHandler.ConfigureIDEs()

	// Create HTTP server that owns /mcp, /logs, /action, /state, /version routes
	httpSrv := NewTinywasmHTTP(mcpPort, mcpHandler.HTTPHandler(), sseHub, cfg.Version)
	httpSrv.SetLog(logger)

	// Create the ProjectToolProxy — registered once, updated when projects start/stop
	proxy := NewProjectToolProxy()

	// Update dtp to store proxy and HTTP server
	dtp.toolProxy = proxy
	dtp.httpSrv = httpSrv

	// Handle action webhooks (e.g. from the TUI Client when user presses Ctrl+C sending "stop")
	httpSrv.OnAction(func(key, value string) {
		// Try project handlers first (shortcuts like "b" for browser, etc.)
		dtp.mu.Lock()
		projectTui := dtp.projectTui
		dtp.mu.Unlock()
		if projectTui != nil && projectTui.DispatchAction(key, value) {
			return
		}
		// Fall back to daemon-level UI dispatch
		if ui.DispatchAction(key, value) {
			return
		}
		switch key {
		case "start":
			if value != "" {
				logger("Start command received for path:", value)
				go dtp.startProject(value)
			}
		case "stop":
			logger("Stop command received from UI")
			dtp.stopProject()
		case "restart":
			logger("Restart command received from UI")
			dtp.restartCurrentProject()
		default:
			logger("Unknown UI action:", key)
		}
	})

	httpSrv.OnState(func() []byte {
		// Return project handlers state if a project is running, else daemon state
		dtp.mu.Lock()
		projectTui := dtp.projectTui
		dtp.mu.Unlock()
		if projectTui != nil {
			return projectTui.GetHandlerStates()
		}
		return ui.GetHandlerStates()
	})

	// Optional: auto-start the project in the current directory
	// so `tinywasm -mcp` behaves similarly to before for the local folder.
	if cfg.StartDir != "" && cfg.StartDir != "/" {
		go func() {
			// Give the server a moment to start
			time.Sleep(500 * time.Millisecond)
			dtp.startProject(cfg.StartDir)
		}()
	}

	// Block forever serving HTTP (which includes /mcp, /logs, /action, /state, /version)
	httpSrv.Serve(exitChan)
}

// daemonToolProvider implements mcp.ToolProvider to expose global daemon tools
// and manages the lifecycle of the running project instance.
type daemonToolProvider struct {
	cfg           BootstrapConfig
	httpSrv       *TinywasmHTTP
	toolProxy     *ProjectToolProxy // Updated when projects start/stop
	logger        func(messages ...any)
	projectCancel context.CancelFunc
	projectDone   chan struct{}
	projectTui    *HeadlessTUI // Current project's TUI (updated per project start)
	mu            sync.Mutex
	lastPath      string // Keep track of the last path for remote restarts
}

func newDaemonToolProvider(cfg BootstrapConfig, logger func(messages ...any)) *daemonToolProvider {
	return &daemonToolProvider{
		cfg:    cfg,
		logger: logger,
	}
}

func (d *daemonToolProvider) GetMCPTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "start_development",
			Description: "Start a TinyWASM project environment (Server, WASM compiler, Assets, Browser) in the specified directory. Will stop any currently running project first.",
			Parameters: []mcp.Parameter{
				{
					Name:        "ide_name",
					Description: "Name of the IDE or LLM client making the request (e.g., 'vsc', 'cursor')",
					Required:    true,
					Type:        "string",
				},
				{
					Name:        "project_path",
					Description: "Absolute or relative path to the TinyWASM project directory to start",
					Required:    true,
					Type:        "string",
				},
			},
			Execute: func(args map[string]any) {
				path, ok := args["project_path"].(string)
				if !ok {
					d.logger("Error: project_path is required and must be a string")
					return
				}

				ide, ok := args["ide_name"].(string)
				if !ok {
					ide = "unknown"
				}

				d.logger("Starting development environment for:", path, "(requested by:", ide, ")")
				d.startProject(path)
			},
		},
	}
}

func (d *daemonToolProvider) stopProject() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.projectCancel != nil {
		d.projectCancel()
		d.projectCancel = nil
	}
}

func (d *daemonToolProvider) restartCurrentProject() {
	d.mu.Lock()
	path := d.lastPath
	d.mu.Unlock()

	if path != "" {
		d.startProject(path)
	} else {
		d.logger("Cannot restart: no project has been started yet.")
	}
}

func (d *daemonToolProvider) startProject(projectPath string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.lastPath = projectPath

	// 1. Cancel previous project
	if d.projectCancel != nil {
		d.projectCancel()
	}

	// 2. Block until port 8080 unbinds
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

portLoop:
	for {
		select {
		case <-timeout:
			d.logger("Warning: Port 8080 still active after timeout, attempting to start anyway...")
			break portLoop
		case <-ticker.C:
			conn, err := net.Dial("tcp", "localhost:"+os.Getenv("PORT"))
			if err != nil {
				// We assume it's 8080 if PORT isn't set, this dial check uses the same env as Start
				// Actually, Start sets a default 8080 if empty, but we can't easily know here.
				// For safety, let's just dial 8080 as a heuristic.
				conn8080, err8080 := net.Dial("tcp", "localhost:8080")
				if err8080 != nil {
					break portLoop
				}
				conn8080.Close()
			} else {
				conn.Close()
			}
		}
	}

	d.logger("Project Restart logic: starting new project at", projectPath)

	ctx, cancel := context.WithCancel(context.Background())
	d.projectCancel = cancel
	d.projectDone = make(chan struct{})

	go func() {
		defer close(d.projectDone)
		d.runProjectLoop(ctx, projectPath)
	}()
}

func (d *daemonToolProvider) runProjectLoop(ctx context.Context, projectPath string) {
	// Create a separate run channel for this project
	runExitChan := make(chan bool)
	headlessTui := NewHeadlessTUI(d.logger)
	// Wire component loggers to the daemon SSE hub so the client TUI receives structured logs
	headlessTui.RelayLog = func(tabTitle, handlerName, color, msg string) {
		d.httpSrv.PublishTabLog(tabTitle, handlerName, color, msg)
	}

	// Register project TUI so /state and /action can reach project handlers
	d.mu.Lock()
	d.projectTui = headlessTui
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		d.projectTui = nil
		d.mu.Unlock()
	}()
	browser := d.cfg.BrowserFactory(headlessTui, runExitChan)

	// We wire context cancellation to the channels
	go func() {
		select {
		case <-ctx.Done():
			d.logger("Context cancelled, stopping project loop...")
			close(runExitChan)
		case <-runExitChan:
			// app stopped itself
		}
	}()

	// Clear proxy on exit
	defer func() {
		d.toolProxy.SetActive()
		d.logger("Project loop cleanup: proxy cleared")
	}()

	// Let's start the app loop here
	for {
		// Callback to set up proxy after handler is initialized
		onProjectReady := func(h *Handler) {
			d.toolProxy.SetActive(buildProjectProviders(h)...)
			d.logger("ProjectToolProxy activated with", len(buildProjectProviders(h)), "tool providers")
		}

		restart := Start(
			projectPath,
			d.logger,
			headlessTui,
			browser,
			d.cfg.DB,
			runExitChan,
			d.cfg.ServerFactory,
			d.cfg.GitHubAuth,
			d.cfg.GitHandler,
			d.cfg.GoModHandler,
			true,  // headless
			false, // clientMode
			onProjectReady,
			// Empty tools
		)
		if !restart || ctx.Err() != nil {
			break
		}
		d.logger("Restarting project loop...")
		// Recreate exit channel for next loop
		runExitChan = make(chan bool)
		go func(c chan bool) {
			select {
			case <-ctx.Done():
				close(c)
			case <-c:
			}
		}(runExitChan)
	}
	d.logger("Project loop ended for", projectPath)
}
