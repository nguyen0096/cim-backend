package apptest

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Health Check API", func() {
	It("should return healthy status", func() {
		client := NewClient(tenv, "")
		resp, err := client.MakeRequest("GET", "/health", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(200))
		result := ParseResponse(resp)
		Expect(result["status"]).To(Equal("ok"))
	})
})
