package execute

import "fmt"

// RecipeError a recipe ran and failed. Exit 1.
type RecipeError struct {
	Target string
	Err    error // *shell.ExitError or other
}

func (e *RecipeError) Error() string { return fmt.Sprintf("target %q failed: %v", e.Target, e.Err) }
func (e *RecipeError) Unwrap() error { return e.Err }
func (e *RecipeError) ExitCode() int { return 1 }

// EnvError the environment lacks something Loom needs. Exit 3.
type EnvError struct{ Msg, Hint string }

func (e *EnvError) Error() string { return e.Msg }
func (e *EnvError) ExitCode() int { return 3 }
