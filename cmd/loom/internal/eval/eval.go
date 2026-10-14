package eval

// Options are the complete inputs to Evaluate. Tests inject every field.
type Options struct {
	File      string            // path to the Loomfile as given (relative or absolute)
	Overrides map[string]string // CLI VAR=val
	Environ   []string          // snapshot of os.Environ(); nil = empty environment
	Funcs     map[string]Func   // nil = DefaultFuncs()
	LoomPath  string            // value of the $loom built-in (absolute path of the running binary)
	GOOS      string            // "" = runtime.GOOS
	GOARCH    string            // "" = runtime.GOARCH
}

// evaluator holds the state of one Evaluate call. T07 adds the remaining fields.
type evaluator struct {
	opts  Options
	root  string // absolute, forward slashes
	scope *scope
	funcs map[string]Func
}
