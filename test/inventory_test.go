package apptest

import (
	"cim-backend/internal/models"
	"cim-backend/pkg/testutil"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Inventory API", func() {
	Describe("Inventory Operations", func() {
		It("should create and list inventories", func() {
			client := testutil.NewClient(tenv, models.RoleAdmin)

			// Create an inventory
			inventoryData := map[string]interface{}{
				"name":        "Test Inventory",
				"description": "Test Inventory Description",
				"location":    "Test Location",
			}

			resp, err := client.MakeRequest("POST", "/api/v1/inventories", inventoryData, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(201))

			inventoryResp := testutil.ParseResponse(resp)
			Expect(inventoryResp["id"]).NotTo(BeNil())
			Expect(inventoryResp["name"]).To(Equal("Test Inventory"))

			// List inventories
			resp, err = client.MakeRequest("GET", "/api/v1/inventories", nil, testutil.WithAuth())
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			// ListInventory returns an array directly, not wrapped in a map
			listResp, err := testutil.ParseResponseArray(resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(listResp)).To(BeNumerically(">=", 1))
		})
	})
})
