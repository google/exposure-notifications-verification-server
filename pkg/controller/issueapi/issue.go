package issueapi

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mikehelmick/tek-verification-server/pkg/api"
	"github.com/mikehelmick/tek-verification-server/pkg/controller"
	"github.com/mikehelmick/tek-verification-server/pkg/database"
)

type IssueAPI struct {
	database database.Database
}

func New(db database.Database) controller.Controller {
	return &IssueAPI{db}
}

func (iapi *IssueAPI) Execute(c *gin.Context) {
	var request api.IssuePINRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response := api.IssuePINResponse{}

	// Generate PIN
	source := make([]byte, 6)
	_, err := rand.Read(source)
	if err != nil {
		response.Error = err.Error()
		c.JSON(http.StatusInternalServerError, response)
		return
	}
	pinCode := base64.RawStdEncoding.EncodeToString(source)

	claims := map[string]string{
		"lab":   "test r us",
		"batch": "test batch number",
	}
	_, err = iapi.database.InsertPIN(pinCode, request.Risks, claims, request.ValidFor)
	if err != nil {
		response.Error = err.Error()
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	response.PIN = pinCode
	c.JSON(http.StatusOK, response)
}
