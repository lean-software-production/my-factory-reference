// Package steps runs the homework's feature files as the factory's test suite.
package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
)

// Built once for the whole suite by TestMain.
var (
	factoryBinary string // the factory, copied into each example's workspace
	agentDouble   string // the double chosen as the agent on the command line
	fakePiDir     string // holds a double called pi-rpc-acp, and pi, put first on the PATH
	spyDouble     string // wraps the real pi-rpc-acp, noting whether it changed the target
	bridgeDir     string // the starter's tools/pi-rpc-acp, which runs pi as an ACP agent
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "factory-suite-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := 1
	if err := buildSuite(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		code = m.Run()
	}
	os.RemoveAll(dir)
	os.Exit(code)
}

// buildSuite builds the factory and the doubles into dir, and gives every
// git the suite runs, the factory's included, the same quiet identity.
func buildSuite(dir string) error {
	bin := filepath.Join(dir, "bin")
	fakePiDir = filepath.Join(dir, "pi")
	factoryBinary = filepath.Join(dir, "factory")
	agentDouble = filepath.Join(bin, "agent")
	spyDouble = filepath.Join(bin, "spy")
	builds := [][]string{
		{"-o", factoryBinary, "../cmd/factory"},
		{"-o", agentDouble, "./doubles/agent"},
		{"-o", filepath.Join(fakePiDir, "pi-rpc-acp"), "./doubles/agent"},
		// The factory checks the pi that pi-rpc-acp runs is there. The
		// double never runs it.
		{"-o", filepath.Join(fakePiDir, "pi"), "./doubles/agent"},
		{"-o", spyDouble, "./doubles/spy"},
	}
	for _, args := range builds {
		build := exec.Command("go", append([]string{"build"}, args...)...)
		if out, err := build.CombinedOutput(); err != nil {
			return fmt.Errorf("go build %v: %v\n%s", args, err, out)
		}
	}

	var err error
	if bridgeDir, err = filepath.Abs(filepath.Join("..", "..", "tools", "pi-rpc-acp")); err != nil {
		return err
	}
	gitconfig := filepath.Join(dir, "gitconfig")
	config := "[user]\n\tname = Factory Suite\n\temail = suite@example.com\n" +
		"[init]\n\tdefaultBranch = main\n[commit]\n\tgpgsign = false\n"
	if err := os.WriteFile(gitconfig, []byte(config), 0o644); err != nil {
		return err
	}
	os.Setenv("GIT_CONFIG_GLOBAL", gitconfig)
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	return nil
}

// TestFeatures runs ../spec/features. FACTORY_TAGS picks the examples; by
// default it leaves out the ones that need a real agent. FACTORY_FEATURES,
// if set, names the feature files (or file:line) to run instead of them all.
func TestFeatures(t *testing.T) {
	tags := os.Getenv("FACTORY_TAGS")
	if tags == "" {
		tags = "~@real-agent"
	}
	paths := []string{"../spec/features"}
	if features := os.Getenv("FACTORY_FEATURES"); features != "" {
		paths = strings.Fields(features)
	}
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    paths,
			Tags:     tags,
			Strict:   true,
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("the feature suite failed")
	}
}
