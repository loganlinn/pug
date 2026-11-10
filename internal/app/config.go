package app

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/leg100/pug/internal"
	"github.com/leg100/pug/internal/cliconfig"
	"github.com/leg100/pug/internal/logging"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"github.com/peterbourgon/ff/v4/ffyaml"
)

type Config struct {
	Program                 string
	MaxTasks                int
	PluginCache             bool
	Debug                   bool
	DisableReloadAfterApply bool
	Workdir                 internal.Workdir
	DataDir                 string
	Envs                    []string
	Args                    []string
	Terragrunt              bool
	Logging                 logging.Options
	Includes                []string
	Excludes                []string

	Version bool
}

var (
	defaultIncludes = []string{"**/*"}
	defaultExcludes = []string{"**/.terraform", "**/.terragrunt-cache"}
)

// set config in order of precedence:
// 1. flags > 2. env vars > 3. config file
func Parse(stderr io.Writer, args []string) (Config, error) {
	var cfg Config

	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("retrieving user's home directory: %w", err)
	}
	defaultDataDir := filepath.Join(home, ".pug")
	defaultConfigFile := filepath.Join(home, ".pug.yaml")

	fs := ff.NewFlagSet("pug")
	fs.StringVar(&cfg.Program, 'p', "program", "terraform", "The default program to use with pug.")
	workdir := fs.String('w', "workdir", ".", "The working directory containing modules.")
	fs.IntVar(&cfg.MaxTasks, 't', "max-tasks", 2*runtime.NumCPU(), "The maximum number of parallel tasks.")
	fs.StringVar(&cfg.DataDir, 0, "data-dir", defaultDataDir, "Directory in which to store plan files.")
	fs.StringListVar(&cfg.Envs, 'e', "env", "Environment variable to pass to terraform process. Can set more than once.")
	fs.StringListVar(&cfg.Args, 'a', "arg", "CLI arg to pass to terraform process. Can set more than once.")
	fs.BoolVar(&cfg.Debug, 'd', "debug", "Log bubbletea messages to messages.log")
	fs.BoolVar(&cfg.Version, 'v', "version", "Print version.")
	_ = fs.String('c', "config", defaultConfigFile, "Path to config file.")

	fs.BoolVar(&cfg.DisableReloadAfterApply, 0, "disable-reload-after-apply", "Disable automatic reload of state following an apply.")
	fs.StringListVar(&cfg.Includes, 0, "include", fmt.Sprintf("Glob pattern of directories in which to search for modules. Can set more than once. (default: %s)", strings.Join(defaultIncludes, ",")))
	fs.StringListVar(&cfg.Excludes, 0, "exclude", fmt.Sprintf("Glob pattern of directories to exclude from search for modules. Takes precedence over --include. Can set more than once. (default: %s)", strings.Join(defaultExcludes, ",")))

	{
		usage := fmt.Sprintf("Logging level (valid: %s).", strings.Join(logging.ValidLevels(), ","))
		fs.StringEnumVar(&cfg.Logging.Level, 'l', "log-level", usage, logging.ValidLevels()...)
	}

	// Plugin cache is enabled not via pug flags but via terraform config
	tfcfg, _ := cliconfig.LoadConfig()
	cfg.PluginCache = (tfcfg.PluginCacheDir != "")

	err = ff.Parse(fs, args,
		ff.WithEnvVarPrefix("PUG"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ffyaml.Parse),
		ff.WithConfigAllowMissingFile(),
	)
	if err != nil {
		// ff.Parse returns an error if there is an error or if -h/--help is
		// passed; in either case print flag usage in addition to error message.
		fmt.Fprintln(stderr, ffhelp.Flags(fs))
		return Config{}, err
	}

	// If user has specified terragrunt as the program executable then enable
	// terragrunt mode.
	if cfg.Program == "terragrunt" {
		cfg.Terragrunt = true
	}

	// Setting a list replaces its defaults, like any other flag.
	if len(cfg.Includes) == 0 {
		cfg.Includes = slices.Clone(defaultIncludes)
	}
	if len(cfg.Excludes) == 0 {
		cfg.Excludes = slices.Clone(defaultExcludes)
	}
	for _, patterns := range [][]string{cfg.Includes, cfg.Excludes} {
		for i, p := range patterns {
			if p == "" || !doublestar.ValidatePattern(p) {
				return Config{}, fmt.Errorf("invalid glob pattern: %q", p)
			}
			// Patterns match paths relative to workdir, e.g. "./modules/" -> "modules".
			patterns[i] = path.Clean(p)
		}
	}

	// Perform any conversions from the flag parsed primitive types to pug
	// defined types.
	cfg.Workdir, err = internal.NewWorkdir(*workdir)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}
