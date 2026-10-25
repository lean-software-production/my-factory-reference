// Package steps runs the homework's feature files as the factory's test suite.
package steps

import (
	"os"
	"testing"

	"github.com/cucumber/godog"
)

// TestFeatures runs ../spec/features. FACTORY_TAGS picks the examples; by
// default it leaves out the ones that need a real agent.
func TestFeatures(t *testing.T) {
	tags := os.Getenv("FACTORY_TAGS")
	if tags == "" {
		tags = "~@real-agent"
	}
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../spec/features"},
			Tags:     tags,
			Strict:   true,
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("the feature suite failed")
	}
}

// InitializeScenario registers the step definitions.
func InitializeScenario(sc *godog.ScenarioContext) {}
