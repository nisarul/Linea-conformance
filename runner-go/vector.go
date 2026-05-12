// SPDX-License-Identifier: AGPL-3.0-or-later

package runner

import "encoding/json"

// Vector is the on-disk shape of a single conformance test.
type Vector struct {
	ID          string          `json:"id"`
	Spec        string          `json:"spec"`
	Section     string          `json:"section"`
	Description string          `json:"description"`
	Setup       Setup           `json:"setup"`
	Operation   json.RawMessage `json:"operation"`
	Expected    Expected        `json:"expected"`
}

// Setup describes the initial graph for a vector.
type Setup struct {
	Persons       []SetupPerson       `json:"persons,omitempty"`
	Relationships []SetupRelationship `json:"relationships,omitempty"`
	Sources       []SetupSource       `json:"sources,omitempty"`
	Proposals     []SetupProposal     `json:"proposals,omitempty"`
}

type SetupName struct {
	Text      string `json:"text"`
	Language  string `json:"language,omitempty"`
	Script    string `json:"script,omitempty"`
	Type      string `json:"type,omitempty"`
	Preferred bool   `json:"preferred,omitempty"`
}

type SetupPerson struct {
	ID      string      `json:"id"`
	Names   []SetupName `json:"names,omitempty"`
	Gender  string      `json:"gender,omitempty"`
	Notes   string      `json:"notes,omitempty"`
	Unknown bool        `json:"unknown,omitempty"`
}

type SetupContinuity struct {
	State    string `json:"state"`
	GapKnown bool   `json:"gapKnown,omitempty"`
	GapSize  int    `json:"gapSize,omitempty"`
}

type SetupRelationship struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	From       string          `json:"from"`
	To         string          `json:"to"`
	Certainty  string          `json:"certainty"`
	Continuity SetupContinuity `json:"continuity"`
}

type SetupSource struct {
	ID       string `json:"id"`
	Type     string `json:"type,omitempty"`
	Citation string `json:"citation"`
}

type SetupProposal struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	Action     string `json:"action"`
	EntityKind string `json:"entityKind"`
	TargetID   string `json:"targetId,omitempty"`
}

// Expected captures the success-or-error outcome of a vector.
type Expected struct {
	Outcome string          `json:"outcome"` // "ok" | "error"
	Code    string          `json:"code,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

// --- Operation-specific raw payloads (decoded by dispatch). ---

type opCertaintyAlgebra struct {
	Kind  string   `json:"kind"`
	Input []string `json:"input"`
}

type opFindPaths struct {
	Kind    string `json:"kind"`
	From    string `json:"from"`
	To      string `json:"to"`
	Options struct {
		IncludeAffinal bool `json:"includeAffinal,omitempty"`
		MaxDepth       int  `json:"maxDepth,omitempty"`
		MaxPaths       int  `json:"maxPaths,omitempty"`
	} `json:"options"`
}

type opNKCA struct {
	Kind string `json:"kind"`
	A    string `json:"a"`
	B    string `json:"b"`
}

type opProposalTransition struct {
	Kind       string `json:"kind"`
	ProposalID string `json:"proposalId"`
	To         string `json:"to"`
	Actor      string `json:"actor"`
	Timestamp  int64  `json:"timestamp"`
	Reason     string `json:"reason"`
}

type opAddRelationship struct {
	Kind         string            `json:"kind"`
	Relationship SetupRelationship `json:"relationship"`
}

type opCreatePerson struct {
	Kind   string      `json:"kind"`
	Person SetupPerson `json:"person"`
}

// --- Expected result shapes ---

type expectedFindPaths struct {
	Paths []expectedPath `json:"paths"`
}

type expectedPath struct {
	From           string `json:"from"`
	To             string `json:"to"`
	Length         int    `json:"length"`
	Certainty      string `json:"certainty"`
	TotalGap       int    `json:"totalGap"`
	GapEdges       int    `json:"gapEdges"`
	Classification string `json:"classification"`
}

type expectedNKCA struct {
	AncestorID        string `json:"ancestorId"`
	AncestorIsUnknown bool   `json:"ancestorIsUnknown"`
	TotalGenerations  int    `json:"totalGenerations"`
	CombinedCertainty string `json:"combinedCertainty"`
}

type expectedProposalTransition struct {
	NewState string `json:"newState"`
}
