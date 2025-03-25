package events

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEvents(t *testing.T) {
	utils.SetLogger()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Events Suite")
}
