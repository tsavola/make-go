// Copyright (c) 2021 Timo Savola. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package make is a simple build system.
package make

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Println prints space-separated strings and a newline.
func Println(s ...string) {
	fmt.Println(strings.Join(s, " "))
}

// Getenv is os.Getenv with default value support.
func Getenv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// Glob terminates program on error.  Results of multiple pattern will be
// concatenated.
func Glob(patterns ...string) []string {
	var results []string

	for _, pat := range patterns {
		matches, err := filepath.Glob(pat)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		results = append(results, matches...)
	}

	return results
}

// Options specified on the command-line.
var Options = make(map[string]string)

var optionAccess = make(map[string]struct{})

// Option specified on the command-line.
func Option(key, defaultValue string) string {
	optionAccess[key] = struct{}{}

	if value, ok := Options[key]; ok {
		return value
	}
	return defaultValue
}

// Flatten strings and string slices into single string slice.  Flatten("foo",
// []string{"bar", "baz"}) returns []string{"foo", "bar", "baz"}.  Flatten will
// panic if called with a type that is not string, []string or []interface{}.
func Flatten(strings ...interface{}) []string {
	return flatten(nil, strings)
}

func flatten(dest []string, strings []interface{}) []string {
	for _, x := range strings {
		switch x := x.(type) {
		case string:
			dest = append(dest, x)

		case []string:
			for _, s := range x {
				dest = append(dest, s)
			}

		case []interface{}:
			dest = flatten(dest, x)

		default:
			panic(x)
		}
	}

	return dest
}

// TargetDefault tasks.
func TargetDefault(name string, tasks ...Task) Task {
	return Task{
		Name:    name,
		Default: true,
		Tasks:   tasks,
		tag:     new(tag),
	}
}

// Target tasks.
func Target(name string, tasks ...Task) Task {
	return Task{
		Name:  name,
		Tasks: tasks,
		tag:   new(tag),
	}
}

// Targets slice.
type Targets []Task

// TargetDefault tasks.
func (ts *Targets) TargetDefault(name string, tasks ...Task) Task {
	t := TargetDefault(name, tasks...)
	*ts = append(*ts, t)
	return t
}

// Target tasks.
func (ts *Targets) Target(name string, tasks ...Task) Task {
	t := Target(name, tasks...)
	*ts = append(*ts, t)
	return t
}

// Command task.
func Command(command ...interface{}) Task {
	return Env(nil).Command(command...)
}

// System task.
func System(commandline string) Task {
	return Env(nil).System(commandline)
}

// Func task.
func Func(f func() error) Task {
	return Task{
		Func: f,
		tag:  new(tag),
	}
}

// If task.
func If(cond func() bool, tasks ...Task) Task {
	return Task{
		Tasks: tasks,
		Cond:  cond,
		tag:   new(tag),
	}
}

// Join tasks.
func Join(tasks ...Task) Task {
	return Task{
		Tasks: tasks,
		tag:   new(tag),
	}
}

// Env variables.
type Env map[string]string

// Command task.
func (env Env) Command(command ...interface{}) Task {
	return Task{
		Command: Flatten(command),
		Env:     env,
		tag:     new(tag),
	}
}

// System task.
func (env Env) System(commandline string) Task {
	return Task{
		Command: strings.Fields(commandline),
		Env:     env,
		tag:     new(tag),
	}
}

// String of environment variables.
func (env Env) String() string {
	var pairs []string
	for k, v := range env {
		pairs = append(pairs, maybeQuote(k)+"="+maybeQuote(v))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, " ")
}

// All conditions.
func All(conds ...func() bool) func() bool {
	if len(conds) == 1 {
		return conds[0]
	}

	return func() bool {
		for _, cond := range conds {
			if !cond() {
				return false
			}
		}
		return true
	}
}

// Any condition.
func Any(conds ...func() bool) func() bool {
	if len(conds) == 1 {
		return conds[0]
	}

	return func() bool {
		for _, cond := range conds {
			if cond() {
				return true
			}
		}
		return false
	}
}

// Outdated condition.
func Outdated(target string, sources ...string) func() bool {
	return func() bool {
		info, err := os.Stat(target)
		if err != nil {
			return true
		}

		targetTime := info.ModTime()

		for _, source := range sources {
			info, err := os.Stat(source)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s dependency %s: %v\n", target, source, err)
				return true
			}

			if info.ModTime().After(targetTime) {
				return true
			}
		}

		return false
	}
}

type tag struct {
	dummy func()
}

// Task to run.
type Task struct {
	Name    string
	Default bool
	Tasks   []Task
	Command []string
	Env     Env
	Func    func() error
	Cond    func() bool

	tag *tag
}

// If returns conditional version of task.
func (task Task) If(cond func() bool) Task {
	task.Cond = cond
	return task
}

func (task Task) commandline() string {
	var cmd []string
	for _, s := range task.Command {
		cmd = append(cmd, maybeQuote(s))
	}
	line := strings.Join(cmd, " ")
	if len(task.Env) > 0 {
		line = task.Env.String() + " " + line
	}
	return line
}

func run(task Task, cache map[*tag]struct{}) {
	if task.tag == nil {
		fmt.Fprintln(os.Stderr, "Task values must not be created directly")
		os.Exit(1)
	}
	if _, done := cache[task.tag]; done {
		return
	}
	cache[task.tag] = struct{}{}

	if task.Cond != nil && !task.Cond() {
		return
	}

	for _, subtask := range task.Tasks {
		run(subtask, cache)
	}

	if len(task.Command) > 0 {
		Println("Running", task.commandline())

		cmd := exec.Command(task.Command[0], task.Command[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if task.Func != nil {
		if err := task.Func(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

// Main program.
func Main(getTargets func() Targets) {
	args := os.Args[1:]

	for _, arg := range args {
		if strings.Contains(arg, "=") && !strings.HasPrefix(arg, "-") {
			ss := strings.SplitN(arg, "=", 2)
			Options[ss[0]] = ss[1]
		}
	}

	available := getTargets()
	defaults := validateTargets(available)

	usage := func(exitcode int) {
		progname := "go run make.go"

		metaTarget := "target"
		if defaults {
			metaTarget = "[target...]"
		}

		fmt.Fprintf(os.Stderr, "Usage: %s %s [OPTION=value...]\n", progname, metaTarget)
		fmt.Fprintf(os.Stderr, "       %s -h|--help\n", progname)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Targets:")

		for _, task := range available {
			if task.Name != "" {
				if task.Default {
					fmt.Fprintf(os.Stderr, "  %s (default)\n", task.Name)
				} else {
					fmt.Fprintf(os.Stderr, "  %s\n", task.Name)
				}
			}
		}

		if len(optionAccess) > 0 {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Options:")

			var names []string
			for name := range optionAccess {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				fmt.Fprintf(os.Stderr, "  %s\n", name)
			}
		}

		fmt.Fprintln(os.Stderr)
		os.Exit(exitcode)
	}

	if len(args) == 1 && (args[0] == "-h" || args[0] == "-help" || args[0] == "--help") {
		usage(0)
	}

	names := make(map[string]struct{})
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			usage(2)
		}
		if !strings.Contains(arg, "=") {
			names[arg] = struct{}{}
		}
	}

	if !defaults && len(names) == 0 {
		usage(2)
	}

	var targets []Task
	found := make(map[string]struct{})

	for _, task := range available {
		_, ok := names[task.Name]
		if ok || (len(names) == 0 && task.Default) {
			targets = append(targets, task)
			found[task.Name] = struct{}{}
		}
	}

	for name := range names {
		if _, ok := found[name]; !ok {
			fmt.Fprintln(os.Stderr, "Unknown target:", name)
			os.Exit(2)
		}
	}

	cache := make(map[*tag]struct{})
	for _, task := range targets {
		run(task, cache)
	}

	os.Exit(0)
}

func validateTargets(targets []Task) (defaults bool) {
	names := make(map[string]struct{})

	for _, task := range targets {
		if task.Default {
			defaults = true
		}

		if task.Name != "" {
			if task.Name == "help" {
				panic(task.Name)
			}

			if _, exist := names[task.Name]; exist {
				panic(task.Name)
			}
			names[task.Name] = struct{}{}
		}
	}

	return
}

func maybeQuote(s string) string {
	q := strconv.Quote(s)
	if strings.Contains(s, " ") || len(s) != len(q)-2 {
		return q
	}
	return s
}
