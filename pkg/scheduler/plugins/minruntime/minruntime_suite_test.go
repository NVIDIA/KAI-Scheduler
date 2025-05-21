package minruntime

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMinruntime(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Minruntime Plugin Suite")
}
