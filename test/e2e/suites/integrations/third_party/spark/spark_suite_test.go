package spark

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSparkIntegration(t *testing.T) {
	utils.SetLogger()
	RegisterFailHandler(Fail)
	RunSpecs(t, "spark integration suite")
}
