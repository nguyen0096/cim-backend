package apptest

import (
	"cim-backend/pkg/testutil"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Health Check API", func() {
	It("should return healthy status", func() {
		client := testutil.NewClient(tenv, "")
		resp, err := client.MakeRequest("GET", "/health", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(200))
		result := testutil.ParseResponse(resp)
		Expect(result["status"]).To(Equal("ok"))
	})
})
