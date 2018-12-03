package main

import (
	"testing"
	"os"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {

	type flagDevListTestCase struct {
		args []string
		shouldError bool
		DevList []DevConfig
	}

	testcases := []flagDevListTestCase {
		{
			args:        []string{},
			shouldError: true,
			DevList:     []DevConfig{},
		},
		{
			args:        []string{"--devs", "/dev/mem"},
			shouldError: true,
			DevList:     []DevConfig{},
		},
		{
			args:        []string{"--devs", "/dev/mem:rwx"},
			shouldError: true,
			DevList:     []DevConfig{},
		},
		{
			args:        []string{"--devs", "/dev/mem:rrw"},
			shouldError: true,
			DevList:     []DevConfig{},
		},
		{
			args:        []string{"--devs", "/dev/mem:rwr"},
			shouldError: true,
			DevList:     []DevConfig{},
		},
		{
			args:        []string{"-devs", "/dev/mem:rwm"},
			shouldError: false,
			DevList:     []DevConfig {
				{DevName: "/dev/mem", Permissions: "rwm"},
			},
		},
		{
			args:        []string{"-devs", "/dev/mem:rw,/dev/cuse:rwm"},
			shouldError: false,
			DevList:     []DevConfig {
				{DevName: "/dev/mem", Permissions: "rw"},
				{DevName: "/dev/cuse", Permissions: "rwm"},
			},
		},
		{
			args:        []string{"--devs", "/dev/mem:rwm"},
			shouldError: false,
			DevList:     []DevConfig {
				{DevName: "/dev/mem", Permissions: "rwm"},
			},
		},
		{
			args:        []string{"--devs", "/dev/mem:rw,/dev/cuse:rwm"},
			shouldError: false,
			DevList:     []DevConfig {
				{DevName: "/dev/mem", Permissions: "rw"},
				{DevName: "/dev/cuse", Permissions: "rwm"},
			},
		},
		{
			args:        []string{"--devs=/dev/mem:rwm"},
			shouldError: false,
			DevList:     []DevConfig {
				{DevName: "/dev/mem", Permissions: "rwm"},
			},
		},
		{
			args:        []string{"--devs=/dev/mem:rw,/dev/cuse:rwm"},
			shouldError: false,
			DevList:     []DevConfig {
				{DevName: "/dev/mem", Permissions: "rw"},
				{DevName: "/dev/cuse", Permissions: "rwm"},
			},
		},

	}

	for _, testcase := range testcases {
		cfg, err := LoadConfigImpl(testcase.args)
		if (testcase.shouldError) {
			assert.NotNil(t, err, "shoud has error for %v", testcase.args)
			continue
		} else {
			assert.NoError(t, err, "LoadConfigImpl fail")
		}

		r := make([]DevConfig, 0, len(testcases))
		for _, cfgPointer := range cfg.DevList {
			cfg := *cfgPointer
			r = append(r, cfg)
		}
		assert.ElementsMatch(t, testcase.DevList, r)
	}
}

func TestNomalizeName(t *testing.T) {
	type normalizeNameTestCase struct {
		DevName string
		Error bool
		NormalizedName string
	}
	testcases := [] normalizeNameTestCase {
		{DevName: "dev/mem", Error: true, NormalizedName: "dev_mem"},
		{DevName: "/dev/mem", Error: false, NormalizedName: "dev_mem"},
		{DevName: "/dev/xx/yy", Error: false, NormalizedName: "dev_xx_yy"},
	}

	for _, testcase := range testcases {
		n, err := NomalizeDevName(testcase.DevName)
		if testcase.Error {
			assert.Error(t, err, "should error for testcase %v", testcase)
			continue
		}
		assert.NoError(t, err)
		assert.Equal(t, testcase.NormalizedName, n)
	}

}


func TestMain(m *testing.M) {
	//flag.Parse()
	os.Exit(m.Run())
}