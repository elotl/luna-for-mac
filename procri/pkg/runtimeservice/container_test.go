package runtimeservice

import (
	"sort"
	"strings"
	"testing"

	cri "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"github.com/stretchr/testify/assert"
)

func TestIsPathAllowed(t *testing.T) {
	testCases := []struct {
		p      string
		result bool
	}{
		{
			p:      "/etc/hosts",
			result: false,
		},
		{
			p:      "/usr/local/bin/foo",
			result: false,
		},
		{
			p:      "/var/kubernetes/secrets/token",
			result: true,
		},
		{
			p:      "/foobar",
			result: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.p, func(t *testing.T) {
			result := isPathAllowed(tc.p)
			assert.Equal(t, tc.result, result)
		})
	}
}

func TestMakeEnvList(t *testing.T) {
	envs := []*cri.KeyValue{
		{
			Key:   "MY_ENV",
			Value: "dummy",
		},
	}
	envStrings := makeEnvList(envs)
	assert.Len(t, envStrings, 5)
	sort.Strings(envStrings)
	assert.True(t, strings.HasPrefix(envStrings[0], "HOME="))
	assert.True(t, strings.HasPrefix(envStrings[1], "HOSTNAME="))
	assert.Equal(t, "MY_ENV=dummy", envStrings[2])
	assert.True(t, strings.HasPrefix(envStrings[3], "PATH="))
	assert.True(t, strings.HasPrefix(envStrings[4], "TERM="))
}
