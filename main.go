// Copyright (c) 2021 Timo Savola. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package make is a simple build system.
package make

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const (
	GOARCH = runtime.GOARCH
	GOOS   = runtime.GOOS
)

// Println prints space-separated strings and a newline.  The arguments will be
// Flatten'ed.
func Println(strs ...interface{}) {
	fmt.Println(strings.Join(Flatten(strs), " "))
}

// Getenv is like os.Getenv(), with default value support.
func Getenv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// Setenv is like os.Setenv(), but program is terminated on error.
func Setenv(key, value string) {
	if err := os.Setenv(key, value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Base is path.Base().
func Base(filename string) string {
	return path.Base(filename)
}

// Dir is path.Dir().
func Dir(filename string) string {
	return path.Dir(filename)
}

// Join is path.Join().
func Join(elem ...string) string {
	return path.Join(elem...)
}

// Fields is strings.Fields().
func Fields(s string) []string {
	return strings.Fields(s)
}

// Exists path?
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !os.IsNotExist(err)
}

// LookPath is like exec.LookPath(), but the first argument that is found is
// returned on success (not the expanded path).  Empty string is returned on
// error.
func LookPath(executables ...string) string {
	for _, file := range executables {
		if path, err := exec.LookPath(file); err == nil && path != "" {
			return file
		}
	}
	return ""
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

// Globber returns a function which globs or terminates program on error.
// Results of multiple pattern will be concatenated.
func Globber(patterns ...string) func() []string {
	return func() []string {
		return Glob(patterns...)
	}
}

// Touch file.  Directories are created as needed.
func Touch(filename string) error {
	os.MkdirAll(path.Dir(filename), 0777)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	return f.Close()
}

// ReplaceSuffix replaces the dot-separated suffix of the filename part of a
// path, or panics.
func ReplaceSuffix(s, newSuffix string) string {
	i := strings.LastIndex(s, ".")
	if i <= 0 || strings.Contains(s[i:], "/") {
		panic(s)
	}
	return s[:i] + newSuffix
}

// Run command.
func Run(command ...string) error {
	Println("Running", command)
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunIO command.
func RunIO(input io.Reader, command ...string) (output []byte, err error) {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = input
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// Vars specified on the command-line.
var Vars = make(map[string]string)
var varDefaults = make(map[string]string)

// Getvar specified on the command-line.
func Getvar(key, defaultValue string) string {
	if value, exist := varDefaults[key]; exist && value != defaultValue {
		panic(fmt.Sprintf("Variable %s accessed with different default values", key))
	}
	varDefaults[key] = defaultValue

	if value, ok := Vars[key]; ok {
		return value
	}
	return defaultValue
}

// Flatten strings and string slices into single string slice.  Flatten("foo",
// []string{"bar", "baz"}) returns []string{"foo", "bar", "baz"}.  Flatten will
// panic if called with a type that is not string, []string, func() []string or
// []interface{}.
func Flatten(strings ...interface{}) []string {
	return flatten(nil, strings)
}

// Wrap is like Flatten, but the first argument is not included if it's empty.
func Wrap(optional string, strings ...interface{}) []string {
	if optional != "" {
		strings = append([]interface{}{optional}, strings...)
	}
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

		case func() []string:
			for _, s := range x() {
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

// Flattener is a lazy version of Flatten.
func Flattener(strings ...interface{}) func() []string {
	return func() []string {
		return Flatten(strings)
	}
}

// TargetDefault tasks.
func TargetDefault(name string, tasks ...Task) Task {
	return Task{
		name:      name,
		isDefault: true,
		tasks:     tasks,
		tag:       new(tag),
	}
}

// Target tasks.
func Target(name string, tasks ...Task) Task {
	return Task{
		name:  name,
		tasks: tasks,
		tag:   new(tag),
	}
}

// Command task.
func Command(command ...interface{}) Task {
	return Env(nil).Command(command...)
}

// CommandWrap task.
func CommandWrap(optionalWrapper string, command ...interface{}) Task {
	return Env(nil).CommandWrap(optionalWrapper, command...)
}

// System task.
func System(commandline string) Task {
	return Env(nil).System(commandline)
}

// Func task.
func Func(f func() error) Task {
	return Task{
		function: f,
		tag:      new(tag),
	}
}

// If task.
func If(cond func() bool, tasks ...Task) Task {
	return Task{
		tasks: tasks,
		cond:  cond,
		tag:   new(tag),
	}
}

// Group tasks.
func Group(tasks ...Task) Task {
	return Task{
		tasks: tasks,
		tag:   new(tag),
	}
}

// Directory creation task.
func Directory(dirpath string) Task {
	return Func(func() error {
		return os.MkdirAll(dirpath, 0777)
	})
}

// DirectoryOf creation task.
func DirectoryOf(filename string) Task {
	return Directory(path.Dir(filename))
}

// Removal task.  Tries to os.RemoveAll the directory trees, and returns the
// first error.
func Removal(directories ...string) Task {
	return Func(func() (err error) {
		for _, path := range directories {
			if e := os.RemoveAll(path); err == nil {
				err = e
			}
		}
		return
	})
}

// Installation task.
func Installation(destName, sourceName string, executable bool) Task {
	return Func(func() error {
		return Install(destName, sourceName, executable)
	})
}

// Install file.
func Install(destination, sourceName string, executable bool) error {
	destName := destination
	if strings.HasSuffix(destName, "/") {
		destName = Join(destName, Base(sourceName))
	}

	source, err := os.Open(sourceName)
	if err != nil {
		return err
	}
	defer source.Close()

	return InstallData(destName, source, executable)
}

// InstallData file.
func InstallData(destName string, source io.Reader, executable bool) error {
	Println("Installing", destName)

	dir := Dir(destName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	temp := Base(destName) + ".*"
	if !strings.HasPrefix(temp, ".") {
		temp = "." + temp
	}

	var (
		ok     bool
		closed bool
	)

	dest, err := ioutil.TempFile(dir, temp)
	if err != nil {
		return err
	}
	defer func() {
		if !ok {
			os.Remove(dest.Name())
		}
		if !closed {
			dest.Close()
		}
	}()

	if _, err := io.Copy(dest, source); err != nil {
		return err
	}

	var perm os.FileMode = 0644
	if executable {
		perm = 0755
	}
	if err := dest.Chmod(perm); err != nil {
		return err
	}

	if err := dest.Sync(); err != nil {
		return err
	}

	err = dest.Close()
	closed = true
	if err != nil {
		return err
	}

	if err := os.Rename(dest.Name(), destName); err != nil {
		return err
	}

	return nil
}

// Env variables.
type Env map[string]string

// Command task.
func (env Env) Command(command ...interface{}) Task {
	return Task{
		command: Flatten(command),
		env:     env,
		tag:     new(tag),
	}
}

// CommandWrap task.
func (env Env) CommandWrap(optional string, command ...interface{}) Task {
	return Task{
		command: Wrap(optional, command),
		env:     env,
		tag:     new(tag),
	}
}

// System task.
func (env Env) System(commandline string) Task {
	return Task{
		command: strings.Fields(commandline),
		env:     env,
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

var globalDeps []string

// Outdated condition.
func Outdated(target string, sources func() []string) func() bool {
	return func() bool {
		info, err := os.Stat(target)
		if err != nil {
			return true
		}

		targetTime := info.ModTime()

		deps := globalDeps
		if sources != nil {
			deps = append([]string(nil), deps...)
			deps = append(deps, sources()...)
		}

		for _, source := range deps {
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

// Missing condition.
func Missing(path string) func() bool {
	return func() bool {
		return !Exists(path)
	}
}

// Thunk returns a function which returns the string in a slice.
func Thunk(strings ...string) func() []string {
	return func() []string {
		return strings
	}
}

type tag struct {
	dummy func()
}

// Task to run.
type Task struct {
	name      string
	isDefault bool
	tasks     []Task
	command   []string
	env       Env
	function  func() error
	cond      func() bool

	tag *tag
}

func (task Task) commandline() string {
	var cmd []string
	for _, s := range task.command {
		cmd = append(cmd, maybeQuote(s))
	}
	line := strings.Join(cmd, " ")
	if len(task.env) > 0 {
		line = task.env.String() + " " + line
	}
	return line
}

func (task Task) environ() []string {
	if task.env == nil {
		return nil
	}

	e := os.Environ()
	for k, v := range task.env {
		e = append(e, k+"="+v)
	}

	return e
}

// Tasks slice.
type Tasks []Task

// Add task at the end of the slice.  Returns a copy.
func (ptr *Tasks) Add(task Task) Task {
	*ptr = append(*ptr, task)
	return task
}

func run(task Task, cache map[*tag]struct{}) bool {
	if task.tag == nil {
		fmt.Fprintln(os.Stderr, "Task values must not be created directly")
		os.Exit(1)
	}
	if _, done := cache[task.tag]; done {
		return false
	}
	cache[task.tag] = struct{}{}

	if task.cond != nil && !task.cond() {
		return false
	}

	var worked bool

	for _, subtask := range task.tasks {
		if run(subtask, cache) {
			worked = true
		}
	}

	if len(task.command) > 0 {
		Println("Running", task.commandline())
		cmd := exec.Command(task.command[0], task.command[1:]...)
		cmd.Env = task.environ()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		worked = true
	}

	if task.function != nil {
		if err := task.function(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		worked = true
	}

	return worked
}

// Main program.
func Main(getTargets func() Tasks, main string, deps ...string) {
	if main != "" {
		globalDeps = append(globalDeps, main)
	}
	globalDeps = append(globalDeps, deps...)

	args := os.Args[1:]

	for _, arg := range args {
		if strings.Contains(arg, "=") && !strings.HasPrefix(arg, "-") {
			ss := strings.SplitN(arg, "=", 2)
			Vars[ss[0]] = ss[1]
		}
	}

	available := getTargets()
	defaults := validateTargets(available)

	for _, arg := range args {
		if strings.Contains(arg, "=") && !strings.HasPrefix(arg, "-") {
			ss := strings.SplitN(arg, "=", 2)
			if _, ok := varDefaults[ss[0]]; !ok {
				fmt.Fprintln(os.Stderr, "Unknown variable:", ss[0])
				os.Exit(2)
			}
		}
	}

	usage := func(exitcode int) {
		metaTarget := "target"
		if defaults {
			metaTarget = "[TARGET]..."
		}

		prog := os.Args[0]
		if main != "" {
			prog = "go run " + main
		}

		fmt.Fprintf(os.Stderr, "Usage: %s %s [VAR=value]...\n", prog, metaTarget)
		fmt.Fprintf(os.Stderr, "       %s -h|--help\n", prog)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Targets:")

		for _, task := range available {
			if task.name != "" {
				if task.isDefault {
					fmt.Fprintf(os.Stderr, "  %s (default)\n", task.name)
				} else {
					fmt.Fprintf(os.Stderr, "  %s\n", task.name)
				}
			}
		}

		if len(varDefaults) > 0 {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Variables:")

			var names []string
			for name := range varDefaults {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				value, found := Vars[name]
				if !found {
					value = varDefaults[name]
				}

				if value == "" {
					fmt.Fprintf(os.Stderr, "  %s\n", name)
				} else {
					fmt.Fprintf(os.Stderr, "  %s (%s)\n", name, value)
				}
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
		_, ok := names[task.name]
		if ok || (len(names) == 0 && task.isDefault) {
			targets = append(targets, task)
			found[task.name] = struct{}{}
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
		if !run(task, cache) {
			fmt.Println("Nothing to be done for", task.name)
		}
	}

	os.Exit(0)
}

func validateTargets(targets []Task) (defaults bool) {
	names := make(map[string]struct{})

	for _, task := range targets {
		if task.isDefault {
			defaults = true
		}

		if task.name != "" {
			if task.name == "help" {
				panic(task.name)
			}

			if _, exist := names[task.name]; exist {
				panic(task.name)
			}
			names[task.name] = struct{}{}
		}
	}

	return
}

func maybeQuote(s string) string {
	if strings.Contains(s, `'`) {
		return strconv.Quote(s)
	}

	switch strings.Count(s, `"`) {
	case 0:
		space := strings.Index(s, ` `)
		if space < 0 {
			return s
		}

		equal := strings.Index(s, `=`)
		if equal < 0 || equal > space {
			return `"` + s + `"`
		}

		return s[:equal+1] + `"` + s[equal+1:] + `"`

	case 2:
		beg := strings.IndexAny(s, `" `)
		if i := strings.Index(s, `=`); i >= 0 && i < beg {
			beg = i + 1
		}

		end := strings.LastIndexAny(s, `" `) + 1

		return s[:beg] + `'` + s[beg:end] + `'` + s[end:]

	default:
		return `'` + s + `'`
	}
}
