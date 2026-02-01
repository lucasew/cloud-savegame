package main

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	rootpkg "github.com/lucasew/cloud-savegame"
	"github.com/lucasew/cloud-savegame/internal/backup"
	"github.com/lucasew/cloud-savegame/internal/config"
	"github.com/lucasew/cloud-savegame/internal/git"
	"github.com/lucasew/cloud-savegame/internal/rules"
	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	outputDir string
	verbose   bool
	useGit    bool
	backlink  bool
	maxDepth  int
)

var varRegex = regexp.MustCompile(`\$([a-z_]+)`)

var rootCmd = &cobra.Command{
	Use:   "cloud-savegame",
	Short: "Backs up games saved data",
	Run:   run,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	exe, _ := os.Executable()
	defaultCfg := filepath.Join(filepath.Dir(exe), "demo.cfg")

	rootCmd.Flags().StringVarP(&cfgFile, "config", "c", defaultCfg, "Configuration file")
	rootCmd.Flags().StringVarP(&outputDir, "output", "o", "", "Which folder to copy backed up files")
	_ = rootCmd.MarkFlagRequired("output")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Give more detail")
	rootCmd.Flags().BoolVarP(&useGit, "git", "g", false, "Use git for snapshot")
	rootCmd.Flags().BoolVarP(&backlink, "backlink", "b", false, "Create symlinks at the origin")
	rootCmd.Flags().IntVar(&maxDepth, "max-depth", 10, "Max depth for filesystem searches")
}

func run(cmd *cobra.Command, args []string) {
	// Setup logging
	lvl := slog.LevelInfo
	if verbose {
		lvl = slog.LevelDebug
	}
	opts := &slog.HandlerOptions{Level: lvl}
	logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
	slog.SetDefault(logger)

	// Validate args
	if useGit {
		_, err := exec.LookPath("git")
		if err != nil {
			slog.Error("git required but not available")
			os.Exit(1)
		}
	}

	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		slog.Error("Configuration file is not actually a file", "path", cfgFile)
		os.Exit(1)
	}

	outPath, _ := filepath.Abs(outputDir)
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		if err := os.MkdirAll(outPath, 0755); err != nil {
			slog.Error("Failed to create output dir", "error", err)
			os.Exit(1)
		}
	}

	cfg := config.New()
	slog.Debug("loading configuration file", "path", cfgFile)
	if err := cfg.Load(cfgFile); err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Setup Git
	var g *git.Wrapper
	if useGit {
		g = git.New(outPath) // Git operations in output dir
		if _, err := os.Stat(filepath.Join(outPath, ".git")); os.IsNotExist(err) {
			if err := g.Init(cmd.Context(), "master"); err != nil {
				slog.Error("git init failed", "error", err)
			}
		}
		dirty, _ := g.IsRepoDirty(cmd.Context())
		if dirty {
			host, _ := os.Hostname()
			_ = g.Exec(cmd.Context(), "add", "-A")
			_ = g.Exec(cmd.Context(), "stash", "push")
			_ = g.Exec(cmd.Context(), "stash", "pop")
			_ = g.Exec(cmd.Context(), "add", "-A")
			_ = g.Commit(cmd.Context(), fmt.Sprintf("dirty repo state from hostname %s", host))
		}
	}

	// Setup Engine
	embeddedRules, _ := fs.Sub(rootpkg.RulesFS, "rules")
	ruleSources := []fs.FS{embeddedRules}

	customRulesDir := filepath.Join(outPath, "__rules__")
	if err := os.MkdirAll(customRulesDir, 0755); err == nil {
		ruleSources = append(ruleSources, os.DirFS(customRulesDir))
	} else {
		slog.Error("Failed to mkdir custom rules", "error", err)
	}

	rl := rules.NewLoader(cfg, ruleSources)
	eng := backup.NewEngine(cfg, g, rl, outPath)
	eng.Backlink = backlink
	eng.Verbose = verbose
	eng.MaxDepth = maxDepth
	eng.IgnoredPaths = cfg.GetPaths("search", "ignore")

	// Pre-load variable usage
	varUsers := make(map[string][]string) // var -> []apps
	allApps, _ := rl.GetApps()

	// Parse all rules to find variable usage
	for app, rf := range allApps {
		r, err := rl.ParseRules(app, rf)
		if err != nil {
			continue
		}
		for _, rule := range r {
			matches := varRegex.FindAllStringSubmatch(rule.Path, -1)
			for _, m := range matches {
				if len(m) > 1 {
					v := m[1]
					varUsers[v] = append(varUsers[v], app)
				}
			}
			if len(matches) == 0 {
				eng.IngestPath(app, rule.Name, rule.Path, false, "")
			}
		}
	}

	startTime := time.Now()

	// Process installdir
	for _, app := range varUsers["installdir"] {
		installDirs := cfg.GetPaths(app, "installdir")
		if len(installDirs) == 0 {
			if cfg.GetStr(app, "not_installed") == "" {
				eng.WarningNews(fmt.Sprintf("installdir missing for game %s", app))
			}
			continue
		}

		for _, installDir := range installDirs {
			if _, err := os.Stat(installDir); os.IsNotExist(err) {
				eng.WarningNews(fmt.Sprintf("Game install dir for %s doesn't exist: %s", app, installDir))
				continue
			}
			if eng.IsPathIgnored(installDir) {
				continue
			}

			appRules, _ := rl.ParseRules(app, allApps[app])
			for _, r := range appRules {
				resolved := strings.ReplaceAll(r.Path, "$installdir", installDir)
				if resolved != r.Path {
					eng.IngestPath(app, r.Name, resolved, true, installDir)
				}
			}
		}
	}

	// Discover Homes
	extraHomes := cfg.GetPaths("search", "extra_homes")
	var homes []string
	for _, h := range extraHomes {
		if !eng.IsPathIgnored(h) {
			if _, err := os.Stat(h); err == nil {
				homes = append(homes, h)
			} else {
				eng.WarningNews(fmt.Sprintf("extra home '%s' does not exist", h))
			}
		}
	}

	searchPaths := cfg.GetPaths("search", "paths")
	for _, p := range searchPaths {
		homes = append(homes, eng.SearchForHomes(p, maxDepth)...)
	}

	// Process Homes
	for _, home := range homes {
		if eng.IsPathIgnored(home) {
			continue
		}
		slog.Debug("Looking for stuff", "home", home)

		// $home
		for _, app := range varUsers["home"] {
			appRules, _ := rl.ParseRules(app, allApps[app])
			for _, r := range appRules {
				resolved := strings.ReplaceAll(r.Path, "$home", home)
				if resolved != r.Path {
					eng.IngestPath(app, r.Name, resolved, true, home)
				}
			}
		}

		// $appdata
		appdata := filepath.Join(home, "AppData")
		for _, app := range varUsers["appdata"] {
			appRules, _ := rl.ParseRules(app, allApps[app])
			for _, r := range appRules {
				resolved := strings.ReplaceAll(r.Path, "$appdata", appdata)
				if resolved != r.Path {
					eng.IngestPath(app, r.Name, resolved, true, appdata)
				}
			}
		}

		// $program_files
		parent := filepath.Dir(home)
		grandparent := filepath.Dir(parent)

		entries, err := os.ReadDir(grandparent)
		if err == nil {
			for _, entry := range entries {
				pfCandidate := filepath.Join(grandparent, entry.Name())
				if _, err := os.Stat(filepath.Join(pfCandidate, "Common Files")); err == nil {

					// Process program_files
					for _, app := range varUsers["program_files"] {
						appRules, _ := rl.ParseRules(app, allApps[app])
						for _, r := range appRules {
							resolved := strings.ReplaceAll(r.Path, "$program_files", pfCandidate)
							if resolved != r.Path {
								eng.IngestPath(app, r.Name, resolved, true, pfCandidate)
							}
						}
					}

					// Ubisoft Logic
					ubiDir := filepath.Join(pfCandidate, "Ubisoft", "Ubisoft Game Launcher", "savegames")
					if _, err := os.Stat(ubiDir); err == nil {
						ubiUsers, _ := os.ReadDir(ubiDir)
						var ubiUserList []string
						for _, u := range ubiUsers {
							if u.IsDir() {
								ubiUserList = append(ubiUserList, u.Name())
							}
						}

						// Write users.txt
						ubiMetaDir := filepath.Join(outPath, "ubisoft")
						_ = os.MkdirAll(ubiMetaDir, 0755)
						_ = os.WriteFile(filepath.Join(ubiMetaDir, "users.txt"), []byte(strings.Join(ubiUserList, "\n")), 0644)

						// Process ubisoft
						for _, uUser := range ubiUserList {
							ubiVar := filepath.Join(ubiDir, uUser)
							for _, app := range varUsers["ubisoft"] {
								appRules, _ := rl.ParseRules(app, allApps[app])
								for _, r := range appRules {
									resolved := strings.ReplaceAll(r.Path, "$ubisoft", ubiVar)
									if resolved != r.Path {
										eng.IngestPath(app, r.Name, resolved, true, ubiVar)
									}
								}
							}
						}
					}
				}
			}
		}

		// $documents
		docCandidates := []string{"Documentos", "Documents"}
		for _, dc := range docCandidates {
			docs := filepath.Join(home, dc)
			if _, err := os.Stat(docs); err == nil {
				for _, app := range varUsers["documents"] {
					appRules, _ := rl.ParseRules(app, allApps[app])
					for _, r := range appRules {
						resolved := strings.ReplaceAll(r.Path, "$documents", docs)
						if resolved != r.Path {
							eng.IngestPath(app, r.Name, resolved, true, docs)
						}
					}
				}
			}
		}
	}

	// Finish
	slog.Info("Finishing up")
	finishTime := time.Now()
	metaDir := filepath.Join(outPath, "__meta__", eng.Hostname)
	_ = os.MkdirAll(metaDir, 0755)

	_ = os.WriteFile(filepath.Join(metaDir, "last_run.txt"), []byte(fmt.Sprintf("%d", finishTime.Unix())), 0644)

	duration := finishTime.Sub(startTime).Seconds()
	f, _ := os.OpenFile(filepath.Join(metaDir, "run_times.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f != nil {
		_, _ = fmt.Fprintf(f, "%d,%f\n", startTime.Unix(), duration)
		_ = f.Close()
	}

	if useGit && g != nil {
		_ = g.Exec(cmd.Context(), "add", "-A")
		_ = g.Commit(cmd.Context(), fmt.Sprintf("run report for %s", eng.Hostname))
		_ = g.Exec(cmd.Context(), "pull", "--rebase")
		_ = g.Exec(cmd.Context(), "push")
	}

	if len(eng.NewsList) > 0 {
		slog.Warn("=== IMPORTANT INFORMATION ABOUT THE RUN ===")
		for _, item := range eng.NewsList {
			slog.Warn("- " + item)
		}
	}
}
