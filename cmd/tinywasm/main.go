package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/tinywasm/app"
	"github.com/tinywasm/devbrowser"
	"github.com/tinywasm/devflow"
	"github.com/tinywasm/devtui"
	"github.com/tinywasm/kvdb"
	"github.com/tinywasm/server"
)

var Version = "dev"

func main() {
	// NEW: Parse debug flag for unfiltered logging
	debugFlag := flag.Bool("debug", false, "Enable debug mode for unfiltered logs")
    mcpFlag := flag.Bool("mcp", false, "Run as MCP Daemon")
	flag.Parse()

	// Initialize start directory
	startDir, err := os.Getwd()
	if err != nil {
		log.Fatal("Error getting current working directory:", err)
		return
	}

	// Create a Logger instance
	logger := app.NewLogger()

	// Initialize GoMod Handler
	goModHandler := devflow.NewGoModHandler()
	// Initialize Git Handler
	gitHandler, err := devflow.NewGit()
	if err != nil {
		logger.Logger("Error initializing Git handler:", err)
	}

	if projectRoot, err := devflow.FindProjectRoot(startDir); err == nil {
		gitHandler.SetRootDir(projectRoot)
		goModHandler.SetRootDir(projectRoot)
		logger.SetRootDir(projectRoot)
	} else {
		gitHandler.SetRootDir(startDir)
		goModHandler.SetRootDir(startDir)
		logger.SetRootDir(startDir)
	}

	goModHandler.SetLog(logger.Logger)

	// Initialize DB
	db, err := kvdb.New(filepath.Join(startDir, ".env"), logger.Logger, &app.FileStore{})
	if err != nil {
		logger.Logger("Failed to initialize database:", err)
		return
	}

    // Configure Bootstrap
    cfg := app.BootstrapConfig{
        StartDir: startDir,
        McpMode: *mcpFlag,
        Debug: *debugFlag,
        Version: Version,
        AppName: "tinywasm",
        Logger: logger.Logger,
        DB: db,
        GitHandler: gitHandler,
        GoModHandler: goModHandler,
        GitHubAuth: devflow.NewGitHubAuth(),

        TuiFactory: func(exitChan chan bool, clientMode bool, clientURL string) app.TuiInterface {
            return devtui.NewTUI(&devtui.TuiConfig{
				AppName:    "TINYWASM",
				AppVersion: Version,
				ExitChan:   exitChan,
				Color:      devtui.DefaultPalette(),
				Logger:     func(messages ...any) { logger.Logger(messages...) },
				Debug:      *debugFlag,
				ClientMode: clientMode,
				ClientURL:  clientURL,
			})
        },

        BrowserFactory: func(ui app.TuiInterface, exitChan chan bool) app.BrowserInterface {
            browser := devbrowser.New(ui, db, exitChan)
	        browser.SetLog(func(messages ...any) { logger.Logger(messages...) })
            return browser
        },

        ServerFactory: func(exitChan chan bool, ui app.TuiInterface, browser app.BrowserInterface) app.ServerInterface {
            // We use DB to decide which server implementation to use
            serverType, err := db.Get("TINYWASM_SERVER")
            if err != nil {
                serverType = "server" // Default fallback
            }

            switch serverType {
            case "wasi":
                 // Not implemented yet or just return nil/log
                 return nil
            default:
                // Default Server implementation
                return server.New().
                    SetLogger(logger.Logger).
                    SetExitChan(exitChan).
                    SetStore(db).
                    SetUI(ui).
                    SetOpenBrowser(browser.OpenBrowser).
                    SetGitIgnoreAdd(gitHandler.GitIgnoreAdd)
            }
        },
    }

    app.Bootstrap(cfg)
}
