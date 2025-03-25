package podgroup

import (
	"testing"

	"github.com/NVIDIA/KAI-scheduler/test/e2e/modules/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPodGroups(t *testing.T) {
	utils.SetLogger()
	RegisterFailHandler(Fail)
	RunSpecs(t, "PodGroup Suite")
}
