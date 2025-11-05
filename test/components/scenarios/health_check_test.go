package scenarios

import (
	"encoding/json"
	"testing"

	"cim-backend/test/components/helpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (suite *ComponentTestSuite) TestHealthCheck() {
	t := suite.T()
	t.Run("Health Check", func(t *testing.T) {

		// Make request to health endpoint
		resp, err := helpers.MakeRequest(t, "GET", suite.sharedTestContainer.BaseURL+"/health", "", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.Equal(t, "ok", result["status"])
	})
}
