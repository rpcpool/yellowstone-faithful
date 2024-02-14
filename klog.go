package main

import (
	"flag"
	"fmt"

	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"
)

func NewKlogFlagSet() []cli.Flag {
	fs := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(fs)

	fs.Set("v", "2")
	fs.Set("log_file_max_size", "1800")
	fs.Set("logtostderr", "true")

	return []cli.Flag{
		// "log_dir", "", "If non-empty, write log files in this directory (no effect when -logtostderr=true)")
		&cli.StringFlag{
			Name:    "log_dir",
			Usage:   "If non-empty, write log files in this directory (no effect when -logtostderr=true)",
			EnvVars: []string{"FAITHFUL_LOG_DIR"},
			Action: func(cctx *cli.Context, v string) error {
				if v != "" {
					fs.Set("log_dir", v)
				}
				return nil
			},
		},
		// "log_file", "", "If non-empty, use this log file (no effect when -logtostderr=true)")
		&cli.StringFlag{
			Name:    "log_file",
			Usage:   "If non-empty, use this log file (no effect when -logtostderr=true)",
			EnvVars: []string{"FAITHFUL_LOG_FILE"},
			Action: func(cctx *cli.Context, v string) error {
				if v != "" {
					fs.Set("log_file", v)
				}
				return nil
			},
		},
		// "log_file_max_size", 1800,
		&cli.Uint64Flag{
			Name:        "log_file_max_size",
			Usage:       "Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited.",
			EnvVars:     []string{"FAITHFUL_LOG_FILE_MAX_SIZE"},
			DefaultText: "1800",
			Action: func(cctx *cli.Context, v uint64) error {
				fs.Set("log_file_max_size", fmt.Sprint(v))
				return nil
			},
		},

		// "logtostderr", true, "log to standard error instead of files")
		&cli.BoolFlag{
			Name:        "logtostderr",
			Usage:       "log to standard error instead of files",
			EnvVars:     []string{"FAITHFUL_LOGTOSTDERR"},
			DefaultText: "true",
			Action: func(cctx *cli.Context, v bool) error {
				fs.Set("logtostderr", fmt.Sprint(v))
				return nil
			},
		},
		// "alsologtostderr", false, "log to standard error as well as files (no effect when -logtostderr=true)")
		&cli.BoolFlag{
			Name:        "alsologtostderr",
			Usage:       "log to standard error as well as files (no effect when -logtostderr=true)",
			EnvVars:     []string{"FAITHFUL_ALSOLOGTOSTDERR"},
			DefaultText: "false",
			Action: func(cctx *cli.Context, v bool) error {
				fs.Set("alsologtostderr", fmt.Sprint(v))
				return nil
			},
		},
		// "v", "number for the log level verbosity")
		&cli.IntFlag{
			Name:    "v",
			Usage:   "number for the log level verbosity",
			EnvVars: []string{"FAITHFUL_V"},
			Value:   2,
			Action: func(cctx *cli.Context, v int) error {
				fs.Set("v", fmt.Sprint(v))
				return nil
			},
		},
		// "add_dir_header", false, "If true, adds the file directory to the header of the log messages")
		&cli.BoolFlag{
			Name:    "add_dir_header",
			Usage:   "If true, adds the file directory to the header of the log messages",
			EnvVars: []string{"FAITHFUL_ADD_DIR_HEADER"},
			Action: func(cctx *cli.Context, v bool) error {
				fs.Set("add_dir_header", fmt.Sprint(v))
				return nil
			},
		},

		// "skip_headers", false, "If true, avoid header prefixes in the log messages")
		&cli.BoolFlag{
			Name:    "skip_headers",
			Usage:   "If true, avoid header prefixes in the log messages",
			EnvVars: []string{"FAITHFUL_SKIP_HEADERS"},
			Action: func(cctx *cli.Context, v bool) error {
				fs.Set("skip_headers", fmt.Sprint(v))
				return nil
			},
		},
		// "one_output", false, "If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)")
		&cli.BoolFlag{
			Name:    "one_output",
			Usage:   "If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)",
			EnvVars: []string{"FAITHFUL_ONE_OUTPUT"},
			Action: func(cctx *cli.Context, v bool) error {
				fs.Set("one_output", fmt.Sprint(v))
				return nil
			},
		},
		// "skip_log_headers", false, "If true, avoid headers when opening log files (no effect when -logtostderr=true)")
		&cli.BoolFlag{
			Name:    "skip_log_headers",
			Usage:   "If true, avoid headers when opening log files (no effect when -logtostderr=true)",
			EnvVars: []string{"FAITHFUL_SKIP_LOG_HEADERS"},
			Action: func(cctx *cli.Context, v bool) error {
				fs.Set("skip_log_headers", fmt.Sprint(v))
				return nil
			},
		},
		// "stderrthreshold", "logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=false)")
		&cli.StringFlag{
			Name:    "stderrthreshold",
			Usage:   "logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=false)",
			EnvVars: []string{"FAITHFUL_STDERRTHRESHOLD"},
			Action: func(cctx *cli.Context, v string) error {
				if v != "" {
					fs.Set("stderrthreshold", v)
				}
				return nil
			},
		},
		// "vmodule", "comma-separated list of pattern=N settings for file-filtered logging")
		&cli.StringFlag{
			Name:    "vmodule",
			Usage:   "comma-separated list of pattern=N settings for file-filtered logging",
			EnvVars: []string{"FAITHFUL_VMODULE"},
			Action: func(cctx *cli.Context, v string) error {
				if v != "" {
					fs.Set("vmodule", v)
				}
				return nil
			},
		},
		// "log_backtrace_at", "when logging hits line file:N, emit a stack trace")
		&cli.StringFlag{
			Name:    "log_backtrace_at",
			Usage:   "when logging hits line file:N, emit a stack trace",
			EnvVars: []string{"FAITHFUL_LOG_BACKTRACE_AT"},
			Action: func(cctx *cli.Context, v string) error {
				if v != "" {
					fs.Set("log_backtrace_at", v)
				}
				return nil
			},
		},
	}
}
