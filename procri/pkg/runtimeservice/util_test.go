package runtimeservice

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitPath(t *testing.T) {
	testCases := []struct {
		p      string
		result []string
	}{
		{
			p:      "/usr/bin/ls",
			result: []string{"usr", "bin", "ls"},
		},
		{
			p:      "/",
			result: []string{},
		},
		{
			p:      "//usr/bin",
			result: []string{"usr", "bin"},
		},
		{
			p:      "/usr/local/bin/foo",
			result: []string{"usr", "local", "bin", "foo"},
		},
		{
			p:      "//usr///local/bin//foo//",
			result: []string{"usr", "local", "bin", "foo"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.p, func(t *testing.T) {
			result := splitPath(tc.p)
			assert.Equal(t, tc.result, result)
		})
	}
}

func TestIsInsidePath(t *testing.T) {
	testCases := []struct {
		p1     string
		p2     string
		result bool
	}{
		{
			"/usr/bin",
			"/usr",
			true,
		},
		{
			"//usr/bin",
			"/usr",
			true,
		},
		{
			"/usr/bin",
			"//usr",
			true,
		},
		{
			"/usr",
			"/usr/bin",
			false,
		},
		{
			"/usrx/bin",
			"/usr",
			false,
		},
		{
			"/etc/hosts",
			"/etc",
			true,
		},
		{
			"/etc/hosts//",
			"/etc",
			true,
		},
		{
			"/etc/hosts",
			"/etc//",
			true,
		},
		{
			"/private/etc/hosts",
			"/etc/",
			false,
		},
		{
			"/",
			"/usr",
			false,
		},
		{
			"//",
			"/usr",
			false,
		},
		{
			"/",
			"//usr",
			false,
		},
		{
			"/usr/local/bin/foo",
			"/usr",
			true,
		},
		{
			"/usr/local/bin/foo/",
			"/usr",
			true,
		},
		{
			"/usr/local/bin/foo//",
			"/usr",
			true,
		},
		{
			"/usr/local/bin/foo",
			"/usr/",
			true,
		},
		{
			"/usr/local/bin/foo",
			"/usr//",
			true,
		},
		{
			"/usr/local/bin",
			"/usr/local",
			true,
		},
		{
			"/usr",
			"/",
			true,
		},
		{
			"//usr",
			"/",
			true,
		},
		{
			"/usr",
			"//",
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.p1+" in "+tc.p2, func(t *testing.T) {
			msg := fmt.Sprintf("test case %v failed", tc)
			result := isInsidePath(tc.p1, tc.p2)
			assert.Equal(t, tc.result, result, msg)
		})
	}
}
