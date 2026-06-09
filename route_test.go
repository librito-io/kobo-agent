package main

import (
	"reflect"
	"testing"
)

func TestRoute(t *testing.T) {
	known := []string{"pair", "autosync", "watch", "status", "about", "sync-now"}
	tests := []struct {
		name string
		args []string
		want routeResult
	}{
		{"no args -> default", nil, routeResult{kind: routeDefault}},
		{"flag first -> default", []string{"--dry-run"}, routeResult{kind: routeDefault, rest: []string{"--dry-run"}}},
		{"flag with value -> default", []string{"--db", "x"}, routeResult{kind: routeDefault, rest: []string{"--db", "x"}}},
		{"known pair", []string{"pair"}, routeResult{kind: routeSubcommand, name: "pair", rest: []string{}}},
		{"known watch + flag", []string{"watch", "--probe"}, routeResult{kind: routeSubcommand, name: "watch", rest: []string{"--probe"}}},
		{"known sync-now", []string{"sync-now"}, routeResult{kind: routeSubcommand, name: "sync-now", rest: []string{}}},
		{"help --help", []string{"--help"}, routeResult{kind: routeHelp}},
		{"help -h", []string{"-h"}, routeResult{kind: routeHelp}},
		{"help --h double dash", []string{"--h"}, routeResult{kind: routeHelp}},
		{"help -help single dash", []string{"-help"}, routeResult{kind: routeHelp}},
		{"help word", []string{"help"}, routeResult{kind: routeHelp}},
		{"help ignores trailing", []string{"help", "pair"}, routeResult{kind: routeHelp}},
		{"unknown sync stutter", []string{"sync"}, routeResult{kind: routeUnknown, name: "sync"}},
		{"unknown sync + flag", []string{"sync", "--dry-run"}, routeResult{kind: routeUnknown, name: "sync"}},
		{"unknown typo + flag", []string{"statas", "--url", "x"}, routeResult{kind: routeUnknown, name: "statas"}},
		{"unknown empty token", []string{""}, routeResult{kind: routeUnknown, name: ""}},
		{"bare dash -> unknown", []string{"-"}, routeResult{kind: routeUnknown, name: "-"}},
		{"bare dash + flag -> unknown", []string{"-", "--dry-run"}, routeResult{kind: routeUnknown, name: "-"}},
		{"subcommand help not top-level", []string{"watch", "--help"}, routeResult{kind: routeSubcommand, name: "watch", rest: []string{"--help"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := route(tt.args, known)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("route(%q) = %+v, want %+v", tt.args, got, tt.want)
			}
		})
	}
}
