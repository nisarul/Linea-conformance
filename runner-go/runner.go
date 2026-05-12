// SPDX-License-Identifier: AGPL-3.0-or-later

package runner

import (
	"context"
	"encoding/json"
	"fmt"

	lerrors "github.com/nisarul/Linea-core/errors"
	"github.com/nisarul/Linea-core/governance"
	"github.com/nisarul/Linea-core/model"
	"github.com/nisarul/Linea-core/query"
	"github.com/nisarul/Linea-core/store"
	"github.com/nisarul/Linea-core/store/memory"
)

// Result is the outcome of running a single vector.
type Result struct {
	Vector Vector
	Pass   bool
	Reason string // empty on Pass
}

// Run executes one vector and returns a Result. It does not
// throw on failure; assertions live in the caller's test fixture.
func Run(ctx context.Context, v Vector) Result {
	st := memory.New()
	defer st.Close()

	aliases := newAliasMap()

	// Apply setup. Aliases are translated to fresh real IDs.
	if err := applySetup(ctx, st, &v.Setup, aliases); err != nil {
		return failed(v, "setup: "+err.Error())
	}

	// Dispatch on operation kind.
	var meta struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(v.Operation, &meta); err != nil {
		return failed(v, "decode operation kind: "+err.Error())
	}

	switch meta.Kind {
	case "CertaintyAlgebra":
		return runCertaintyAlgebra(v)
	case "FindPaths":
		return runFindPaths(ctx, v, st, aliases)
	case "NKCA":
		return runNKCA(ctx, v, st, aliases)
	case "ProposalTransition":
		return runProposalTransition(ctx, v, st, aliases)
	case "AddRelationshipViaProposal":
		return runAddRelationshipViaProposal(ctx, v, st, aliases)
	case "CreatePersonViaProposal":
		return runCreatePersonViaProposal(ctx, v, st, aliases)
	default:
		return failed(v, "unsupported operation kind: "+meta.Kind)
	}
}

// ---------- helpers ----------

type aliasMap struct {
	persons       map[string]model.ID
	relationships map[string]model.ID
	sources       map[string]model.ID
	proposals     map[string]model.ID
}

func newAliasMap() *aliasMap {
	return &aliasMap{
		persons:       make(map[string]model.ID),
		relationships: make(map[string]model.ID),
		sources:       make(map[string]model.ID),
		proposals:     make(map[string]model.ID),
	}
}

func failed(v Vector, reason string) Result { return Result{Vector: v, Pass: false, Reason: reason} }
func passed(v Vector) Result                { return Result{Vector: v, Pass: true} }

func parseCertainty(s string) (model.Certainty, error) {
	switch s {
	case "Certain":
		return model.CertaintyCertain, nil
	case "Probable":
		return model.CertaintyProbable, nil
	case "Uncertain":
		return model.CertaintyUncertain, nil
	}
	return 0, fmt.Errorf("unknown certainty %q", s)
}

func parseRelType(s string) (model.RelationshipType, error) {
	switch s {
	case "ParentChild":
		return model.RelTypeParentChild, nil
	case "Marriage":
		return model.RelTypeMarriage, nil
	}
	return 0, fmt.Errorf("unknown relationship type %q", s)
}

func parseContinuity(c SetupContinuity) (model.Continuity, error) {
	switch c.State {
	case "Continuous":
		return model.NewContinuous(), nil
	case "Gapped":
		if !c.GapKnown {
			return model.NewGapped(model.UnknownGap()), nil
		}
		gg, err := model.KnownGap(c.GapSize)
		if err != nil {
			return model.Continuity{}, err
		}
		return model.NewGapped(gg), nil
	}
	return model.Continuity{}, fmt.Errorf("unknown continuity state %q", c.State)
}

func parseProposalState(s string) (model.ProposalState, error) {
	switch s {
	case "Draft":
		return model.ProposalDraft, nil
	case "Submitted":
		return model.ProposalSubmitted, nil
	case "UnderReview":
		return model.ProposalUnderReview, nil
	case "Accepted":
		return model.ProposalAccepted, nil
	case "Rejected":
		return model.ProposalRejected, nil
	case "Withdrawn":
		return model.ProposalWithdrawn, nil
	}
	return 0, fmt.Errorf("unknown proposal state %q", s)
}

func parseAction(s string) (model.ProposalAction, error) {
	switch s {
	case "Create":
		return model.ProposalActionCreate, nil
	case "Update":
		return model.ProposalActionUpdate, nil
	case "Retract":
		return model.ProposalActionRetract, nil
	case "Merge":
		return model.ProposalActionMerge, nil
	case "SameAsLink":
		return model.ProposalActionSameAsLink, nil
	}
	return 0, fmt.Errorf("unknown proposal action %q", s)
}

func parseKind(s string) (model.EntityKind, error) {
	switch s {
	case "Person":
		return model.EntityKindPerson, nil
	case "Relationship":
		return model.EntityKindRelationship, nil
	case "Source":
		return model.EntityKindSource, nil
	}
	return 0, fmt.Errorf("unknown entity kind %q", s)
}

func parseNameType(s string) model.NameType {
	if s == "" {
		return model.NameTypeFull
	}
	return model.NameType(s)
}

func errCode(err error) string {
	var e *lerrors.Error
	if !asErr(err, &e) {
		return ""
	}
	return string(e.Code)
}

// asErr is a tiny errors.As shim that avoids pulling errors.As
// into call sites with an unused alias.
func asErr(err error, target **lerrors.Error) bool {
	for err != nil {
		if e, ok := err.(*lerrors.Error); ok {
			*target = e
			return true
		}
		type unwrap interface{ Unwrap() error }
		u, ok := err.(unwrap)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// applySetup creates persons / relationships / sources / proposals
// recorded in v.Setup, mapping each alias to a fresh real ID.
func applySetup(ctx context.Context, st store.Store, s *Setup, aliases *aliasMap) error {
	if s == nil {
		return nil
	}
	_, err := st.Update(ctx, func(tx store.WriteTx) error {
		// Persons first.
		for _, sp := range s.Persons {
			id := model.NewID()
			aliases.persons[sp.ID] = id
			if sp.Unknown {
				ua, err := model.NewUnknownAncestor(id)
				if err != nil {
					return err
				}
				if err := tx.PutPerson(ua); err != nil {
					return err
				}
				continue
			}
			names := make([]model.Name, 0, len(sp.Names))
			if len(sp.Names) == 0 {
				// Vectors that don't supply a name use the alias as the name.
				n, err := model.NewName(sp.ID, "en", "Latn", model.NameTypeFull, true)
				if err != nil {
					return err
				}
				names = append(names, n)
			}
			for _, sn := range sp.Names {
				n, err := model.NewName(sn.Text, sn.Language, sn.Script, parseNameType(sn.Type), sn.Preferred)
				if err != nil {
					return err
				}
				names = append(names, n)
			}
			gender, err := model.ParseGender(sp.Gender, true)
			if err != nil {
				return err
			}
			p, err := model.NewPerson(id, model.PersonOptions{
				Names:  names,
				Gender: gender,
				Notes:  sp.Notes,
			})
			if err != nil {
				return err
			}
			if err := tx.PutPerson(p); err != nil {
				return err
			}
		}
		// Relationships next.
		for _, sr := range s.Relationships {
			id := model.NewID()
			aliases.relationships[sr.ID] = id
			from, ok := aliases.persons[sr.From]
			if !ok {
				return fmt.Errorf("relationship %s: unknown person alias %q", sr.ID, sr.From)
			}
			to, ok := aliases.persons[sr.To]
			if !ok {
				return fmt.Errorf("relationship %s: unknown person alias %q", sr.ID, sr.To)
			}
			rt, err := parseRelType(sr.Type)
			if err != nil {
				return err
			}
			c, err := parseCertainty(sr.Certainty)
			if err != nil {
				return err
			}
			cont, err := parseContinuity(sr.Continuity)
			if err != nil {
				return err
			}
			r, err := model.NewRelationship(id, from, to, rt, c, cont, model.RelationshipOptions{})
			if err != nil {
				return err
			}
			if err := tx.PutRelationship(r); err != nil {
				return err
			}
		}
		// Sources.
		for _, ss := range s.Sources {
			id := model.NewID()
			aliases.sources[ss.ID] = id
			src, err := model.NewSource(id, model.SourceType(ss.Type), ss.Citation, model.SourceOptions{})
			if err != nil {
				return err
			}
			if err := tx.PutSource(src); err != nil {
				return err
			}
		}
		// Proposals — registered at the requested state via WithStateUnchecked.
		for _, sp := range s.Proposals {
			id := model.NewID()
			aliases.proposals[sp.ID] = id
			act, err := parseAction(sp.Action)
			if err != nil {
				return err
			}
			kind, err := parseKind(sp.EntityKind)
			if err != nil {
				return err
			}
			pp, err := model.NewProposal(id, act, kind, model.ProposalOptions{
				TargetID: aliases.persons[sp.TargetID],
			})
			if err != nil {
				return err
			}
			st, err := parseProposalState(sp.State)
			if err != nil {
				return err
			}
			if st != model.ProposalDraft {
				pp = pp.WithStateUnchecked(st, model.ProposalTransition{
					From: model.ProposalDraft, To: st,
				})
			}
			if err := tx.PutProposal(pp); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// ----- operation handlers -----

func runCertaintyAlgebra(v Vector) Result {
	var op opCertaintyAlgebra
	if err := json.Unmarshal(v.Operation, &op); err != nil {
		return failed(v, err.Error())
	}
	cs := make([]model.Certainty, 0, len(op.Input))
	for _, s := range op.Input {
		c, err := parseCertainty(s)
		if err != nil {
			return failed(v, err.Error())
		}
		cs = append(cs, c)
	}
	got := model.CombineCertainties(cs...)
	if v.Expected.Outcome != "ok" {
		return failed(v, "expected error outcome but algebra produced "+got.String())
	}
	var want string
	if err := json.Unmarshal(v.Expected.Result, &want); err != nil {
		return failed(v, "expected.result: "+err.Error())
	}
	if got.String() != want {
		return failed(v, fmt.Sprintf("certainty: got %s, want %s", got, want))
	}
	return passed(v)
}

func runFindPaths(ctx context.Context, v Vector, st store.Store, a *aliasMap) Result {
	var op opFindPaths
	if err := json.Unmarshal(v.Operation, &op); err != nil {
		return failed(v, err.Error())
	}
	from, ok := a.persons[op.From]
	if !ok {
		return failed(v, "unknown person alias: "+op.From)
	}
	to, ok := a.persons[op.To]
	if !ok {
		return failed(v, "unknown person alias: "+op.To)
	}
	rtx, err := st.View(ctx)
	if err != nil {
		return failed(v, err.Error())
	}
	defer rtx.Close()
	paths, err := query.FindPaths(ctx, rtx, from, to, query.Options{
		IncludeAffinal: op.Options.IncludeAffinal,
		MaxDepth:       op.Options.MaxDepth,
		MaxPaths:       op.Options.MaxPaths,
	})
	return checkPathsOutcome(v, a, paths, err)
}

func runNKCA(ctx context.Context, v Vector, st store.Store, a *aliasMap) Result {
	var op opNKCA
	if err := json.Unmarshal(v.Operation, &op); err != nil {
		return failed(v, err.Error())
	}
	pa, ok := a.persons[op.A]
	if !ok {
		return failed(v, "unknown person alias: "+op.A)
	}
	pb, ok := a.persons[op.B]
	if !ok {
		return failed(v, "unknown person alias: "+op.B)
	}
	rtx, err := st.View(ctx)
	if err != nil {
		return failed(v, err.Error())
	}
	defer rtx.Close()
	res, err := query.NearestKnownCommonAncestor(ctx, rtx, pa, pb, query.Options{})
	if v.Expected.Outcome == "error" {
		got := errCode(err)
		if got != v.Expected.Code {
			return failed(v, fmt.Sprintf("nkca: got code %q, want %q", got, v.Expected.Code))
		}
		return passed(v)
	}
	if err != nil {
		return failed(v, "nkca: "+err.Error())
	}
	var want expectedNKCA
	if err := json.Unmarshal(v.Expected.Result, &want); err != nil {
		return failed(v, "expected.result: "+err.Error())
	}
	wantID, ok := a.persons[want.AncestorID]
	if !ok {
		return failed(v, "expected.ancestorId alias not found: "+want.AncestorID)
	}
	if res.AncestorID != wantID {
		return failed(v, fmt.Sprintf("nkca: ancestor mismatch (got %s, want alias %s)", res.AncestorID, want.AncestorID))
	}
	if res.Unknown != want.AncestorIsUnknown {
		return failed(v, fmt.Sprintf("nkca: unknown flag mismatch (got %v, want %v)", res.Unknown, want.AncestorIsUnknown))
	}
	if res.TotalGenerations != want.TotalGenerations {
		return failed(v, fmt.Sprintf("nkca: totalGenerations mismatch (got %d, want %d)",
			res.TotalGenerations, want.TotalGenerations))
	}
	if res.CombinedCertainty.String() != want.CombinedCertainty {
		return failed(v, fmt.Sprintf("nkca: combinedCertainty mismatch (got %s, want %s)",
			res.CombinedCertainty, want.CombinedCertainty))
	}
	return passed(v)
}

func runProposalTransition(ctx context.Context, v Vector, st store.Store, a *aliasMap) Result {
	var op opProposalTransition
	if err := json.Unmarshal(v.Operation, &op); err != nil {
		return failed(v, err.Error())
	}
	pid, ok := a.proposals[op.ProposalID]
	if !ok {
		return failed(v, "unknown proposal alias: "+op.ProposalID)
	}
	rtx, err := st.View(ctx)
	if err != nil {
		return failed(v, err.Error())
	}
	p, err := rtx.GetProposal(pid)
	rtx.Close()
	if err != nil {
		return failed(v, err.Error())
	}
	to, err := parseProposalState(op.To)
	if err != nil {
		return failed(v, err.Error())
	}
	updated, terr := governance.Transition(p, to, op.Actor, op.Timestamp, op.Reason)
	if v.Expected.Outcome == "error" {
		got := errCode(terr)
		if got != v.Expected.Code {
			return failed(v, fmt.Sprintf("transition: got code %q, want %q", got, v.Expected.Code))
		}
		return passed(v)
	}
	if terr != nil {
		return failed(v, "transition: "+terr.Error())
	}
	var want expectedProposalTransition
	if err := json.Unmarshal(v.Expected.Result, &want); err != nil {
		return failed(v, err.Error())
	}
	if updated.State().String() != want.NewState {
		return failed(v, fmt.Sprintf("transition: got %s, want %s", updated.State(), want.NewState))
	}
	return passed(v)
}

// runAddRelationshipViaProposal walks Submit→Claim→Accept on a
// freshly-built Create-Relationship proposal so the cycle/etc.
// checks in the apply layer get exercised end-to-end.
func runAddRelationshipViaProposal(ctx context.Context, v Vector, st store.Store, a *aliasMap) Result {
	var op opAddRelationship
	if err := json.Unmarshal(v.Operation, &op); err != nil {
		return failed(v, err.Error())
	}
	from, ok := a.persons[op.Relationship.From]
	if !ok {
		return failed(v, "unknown person alias: "+op.Relationship.From)
	}
	to, ok := a.persons[op.Relationship.To]
	if !ok {
		return failed(v, "unknown person alias: "+op.Relationship.To)
	}
	rt, err := parseRelType(op.Relationship.Type)
	if err != nil {
		return failed(v, err.Error())
	}
	c, err := parseCertainty(op.Relationship.Certainty)
	if err != nil {
		return failed(v, err.Error())
	}
	cont, err := parseContinuity(op.Relationship.Continuity)
	if err != nil {
		return failed(v, err.Error())
	}
	pl := governance.PayloadCreateRelationship{
		From: from, To: to, Type: rt, Certainty: c, Continuity: cont,
	}
	plBuf, _ := json.Marshal(pl)
	pp, err := model.NewProposal(model.NewID(), model.ProposalActionCreate, model.EntityKindRelationship,
		model.ProposalOptions{Payload: plBuf})
	if err != nil {
		return failed(v, err.Error())
	}
	_, err = st.Update(ctx, func(tx store.WriteTx) error { return tx.PutProposal(pp) })
	if err != nil {
		return failed(v, err.Error())
	}
	if _, err := governance.Submit(ctx, st, pp.ID(), "actor", 1); err != nil {
		return failed(v, err.Error())
	}
	if _, err := governance.Claim(ctx, st, pp.ID(), "actor", 2); err != nil {
		return failed(v, err.Error())
	}
	_, acceptErr := governance.Accept(ctx, st, pp.ID(), "actor", 3)
	return checkErrorOutcome(v, acceptErr)
}

// runCreatePersonViaProposal exercises Create-Person via the
// governance pipeline so the unknown-ancestor fabrication guard
// is hit end-to-end.
func runCreatePersonViaProposal(ctx context.Context, v Vector, st store.Store, a *aliasMap) Result {
	var op opCreatePerson
	if err := json.Unmarshal(v.Operation, &op); err != nil {
		return failed(v, err.Error())
	}
	pl := governance.PayloadCreatePerson{
		UnknownAncestor: op.Person.Unknown,
		Notes:           op.Person.Notes,
	}
	for _, sn := range op.Person.Names {
		n, err := model.NewName(sn.Text, sn.Language, sn.Script, parseNameType(sn.Type), sn.Preferred)
		if err != nil {
			return failed(v, err.Error())
		}
		pl.Names = append(pl.Names, n)
	}
	gender, err := model.ParseGender(op.Person.Gender, true)
	if err != nil {
		return failed(v, err.Error())
	}
	pl.Gender = gender
	plBuf, _ := json.Marshal(pl)
	pp, err := model.NewProposal(model.NewID(), model.ProposalActionCreate, model.EntityKindPerson,
		model.ProposalOptions{Payload: plBuf})
	if err != nil {
		return failed(v, err.Error())
	}
	_, err = st.Update(ctx, func(tx store.WriteTx) error { return tx.PutProposal(pp) })
	if err != nil {
		return failed(v, err.Error())
	}
	if _, err := governance.Submit(ctx, st, pp.ID(), "actor", 1); err != nil {
		return failed(v, err.Error())
	}
	if _, err := governance.Claim(ctx, st, pp.ID(), "actor", 2); err != nil {
		return failed(v, err.Error())
	}
	_, acceptErr := governance.Accept(ctx, st, pp.ID(), "actor", 3)
	return checkErrorOutcome(v, acceptErr)
}

// ----- shared assertion helpers -----

func checkErrorOutcome(v Vector, err error) Result {
	if v.Expected.Outcome == "error" {
		got := errCode(err)
		if got != v.Expected.Code {
			return failed(v, fmt.Sprintf("got code %q, want %q (err=%v)", got, v.Expected.Code, err))
		}
		return passed(v)
	}
	if err != nil {
		return failed(v, err.Error())
	}
	return passed(v)
}

func checkPathsOutcome(v Vector, a *aliasMap, paths []query.Path, err error) Result {
	if v.Expected.Outcome == "error" {
		got := errCode(err)
		if got != v.Expected.Code {
			return failed(v, fmt.Sprintf("findPaths: got code %q, want %q", got, v.Expected.Code))
		}
		return passed(v)
	}
	if err != nil {
		return failed(v, "findPaths: "+err.Error())
	}
	var want expectedFindPaths
	if err := json.Unmarshal(v.Expected.Result, &want); err != nil {
		return failed(v, err.Error())
	}
	if len(paths) != len(want.Paths) {
		return failed(v, fmt.Sprintf("findPaths: got %d paths, want %d", len(paths), len(want.Paths)))
	}
	for i, wp := range want.Paths {
		gp := paths[i]
		wantFrom, ok := a.persons[wp.From]
		if !ok {
			return failed(v, "expected.from alias not found: "+wp.From)
		}
		wantTo, ok := a.persons[wp.To]
		if !ok {
			return failed(v, "expected.to alias not found: "+wp.To)
		}
		if gp.From() != wantFrom || gp.To() != wantTo {
			return failed(v, fmt.Sprintf("path %d: endpoints (%s -> %s) want (%s -> %s)",
				i, gp.From(), gp.To(), wp.From, wp.To))
		}
		if gp.Length != wp.Length {
			return failed(v, fmt.Sprintf("path %d: length %d, want %d", i, gp.Length, wp.Length))
		}
		if gp.Certainty.String() != wp.Certainty {
			return failed(v, fmt.Sprintf("path %d: certainty %s, want %s",
				i, gp.Certainty, wp.Certainty))
		}
		if gp.TotalGap != wp.TotalGap {
			return failed(v, fmt.Sprintf("path %d: totalGap %d, want %d", i, gp.TotalGap, wp.TotalGap))
		}
		if gp.GapEdges != wp.GapEdges {
			return failed(v, fmt.Sprintf("path %d: gapEdges %d, want %d", i, gp.GapEdges, wp.GapEdges))
		}
		if gp.Classification.String() != wp.Classification {
			return failed(v, fmt.Sprintf("path %d: classification %s, want %s",
				i, gp.Classification, wp.Classification))
		}
	}
	return passed(v)
}
