package tfjob

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTFJobIntegration(t *testing.T) {
	utils.SetLogger()
	RegisterFailHandler(Fail)
	RunSpecs(t, "tfjob integration suite")
}
