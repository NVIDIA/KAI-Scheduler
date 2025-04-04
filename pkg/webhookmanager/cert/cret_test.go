// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package cert

import (
	"crypto/x509"
	"encoding/pem"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCert(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cert Suite")
}

var _ = Describe("Cert", func() {
	Context("GenerateSelfSignedCert", func() {
		It("should generate self signed cert", func() {
			cert, key, err := GenerateSelfSignedCert("domain.com", []string{"domain.com", "www.domain.com"})
			Expect(err).To(BeNil())
			Expect(cert).ToNot(BeNil())
			Expect(key).ToNot(BeNil())

			// Expect the certificate to be valid PEM
			block, _ := pem.Decode(cert)
			Expect(block).ToNot(BeNil())
			Expect(block.Type).To(Equal("CERTIFICATE"))

			// Expect the certificate to be valid and self signed
			parsedCert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(parsedCert.Issuer).To(Equal(parsedCert.Subject))

			// Expect the certificate to have a valid signature
			err = parsedCert.CheckSignature(parsedCert.SignatureAlgorithm, parsedCert.RawTBSCertificate, parsedCert.Signature)
			Expect(err).To(BeNil())

			// Expect the certificate to have the correct cn and dns names
			Expect(parsedCert.Subject.CommonName).To(Equal("domain.com"))
			Expect(parsedCert.DNSNames).To(Equal([]string{"domain.com", "www.domain.com"}))
		})
	})
})
