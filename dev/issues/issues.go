//go:build targ

// Package issues provides issue list tooling for targ.
package issues

import (
	"github.com/toejough/targ"
	"github.com/toejough/targ/dev/issues/internal"
)

func init() {
	targ.Register(
		Create,
		Dedupe,
		List,
		Move,
		Update,
		Validate,
	)
}

// Exported variables.
var (
	Create   = targ.Targ(internal.Create).Description("Create a new issue locally or on GitHub")
	Dedupe   = targ.Targ(internal.Dedupe).Description("Remove duplicate done issues from backlog")
	List     = targ.Targ(internal.List).Description("List issues from local file and/or GitHub")
	Move     = targ.Targ(internal.Move).Description("Move a local or GitHub issue to a new status")
	Update   = targ.Targ(internal.Update).Description("Update a local or GitHub issue")
	Validate = targ.Targ(internal.Validate).Description("Validate issue formatting and structure")
)

type CreateArgs = internal.CreateArgs

type DedupeArgs = internal.DedupeArgs

type ListArgs = internal.ListArgs

type MoveArgs = internal.MoveArgs

type UpdateArgs = internal.UpdateArgs

type ValidateArgs = internal.ValidateArgs
