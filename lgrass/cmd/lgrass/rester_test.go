package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseResterArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    resterCommand
		wantErr string
	}{
		{name: "get", args: []string{"abc", "get", "/orders/42"}, want: resterCommand{shortID: "abc", method: "GET", path: "/orders/42"}},
		{name: "verb case", args: []string{"abc", "PATCH", "/orders/42"}, want: resterCommand{shortID: "abc", method: "PATCH", path: "/orders/42"}},
		{name: "post with body and user", args: []string{"abc", "post", "/orders", "--user", "alice", "--body", `{"a":1}`}, want: resterCommand{shortID: "abc", method: "POST", path: "/orders", user: "alice", body: `{"a":1}`}},
		{name: "flags before path", args: []string{"abc", "post", "--user", "alice", "/orders"}, want: resterCommand{shortID: "abc", method: "POST", path: "/orders", user: "alice"}},
		{name: "equals form", args: []string{"abc", "post", "/orders", "--user=alice", `--body={"a":1}`}, want: resterCommand{shortID: "abc", method: "POST", path: "/orders", user: "alice", body: `{"a":1}`}},
		{name: "full url path", args: []string{"abc", "get", "https://api.example.com/orders?x=1"}, want: resterCommand{shortID: "abc", method: "GET", path: "https://api.example.com/orders?x=1"}},
		{name: "info", args: []string{"abc", "info"}, want: resterCommand{shortID: "abc", info: true}},
		{name: "users", args: []string{"abc", "users"}, want: resterCommand{shortID: "abc", users: true}},
		{name: "flush all", args: []string{"abc", "flush"}, want: resterCommand{shortID: "abc", flush: true}},
		{name: "flush one user", args: []string{"abc", "flush", "--user", "alice"}, want: resterCommand{shortID: "abc", flush: true, user: "alice"}},
		{name: "flush equals form", args: []string{"abc", "FLUSH", "--user=alice"}, want: resterCommand{shortID: "abc", flush: true, user: "alice"}},
		{name: "flush with path", args: []string{"abc", "flush", "/x"}, wantErr: "flush takes only --user"},
		{name: "flush with body", args: []string{"abc", "flush", "--body", "{}"}, wantErr: "flush takes only --user"},
		{name: "users extra argument", args: []string{"abc", "users", "--user", "x"}, wantErr: "users takes no other arguments"},
		{name: "info extra argument", args: []string{"abc", "info", "/x"}, wantErr: "info takes no other arguments"},
		{name: "missing command", args: []string{"abc"}, wantErr: "expected a channel id and a command"},
		{name: "unknown command", args: []string{"abc", "fetch", "/x"}, wantErr: `unknown command "fetch"`},
		{name: "old method flag", args: []string{"abc", "--user", "alice", "--method", "GET", "--path", "/x"}, wantErr: `unknown command "--user"`},
		{name: "old path flag after verb", args: []string{"abc", "get", "--path", "/x"}, wantErr: "unknown flag --path"},
		{name: "missing path", args: []string{"abc", "get"}, wantErr: "get needs exactly one path"},
		{name: "two paths", args: []string{"abc", "get", "/a", "/b"}, wantErr: "get needs exactly one path"},
		{name: "flag without value", args: []string{"abc", "get", "/a", "--user"}, wantErr: "--user needs a value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseResterArgs(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
