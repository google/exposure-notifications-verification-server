// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package issue

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mikehelmick/tek-verification-server/pkg/api"
	"github.com/mikehelmick/tek-verification-server/pkg/controller"
	"github.com/mikehelmick/tek-verification-server/pkg/database"
	"github.com/mikehelmick/tek-verification-server/pkg/jsonclient"
)

type IssueController struct {
	apiServer string
	client    *http.Client
}

type issueForm struct {
	TRiskPrimary     int `form:"triskPrimary"`
	PrimaryPeriods   int `form:"primaryPeriods"`
	TRiskSecondary   int `form:"triskSecondary"`
	SecondaryPeriods int `form:"secondaryPeriods"`
}

func New(apiServer string) controller.Controller {
	return &IssueController{
		apiServer: apiServer,
		client:    &http.Client{},
	}
}

func (ic *IssueController) Execute(c *gin.Context) {
	var form issueForm
	// This will infer what binder to use depending on the content-type header.
	if err := c.ShouldBind(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	risks := []database.TransmissionRisk{
		{
			Risk:               form.TRiskPrimary,
			PastRollingPeriods: form.PrimaryPeriods,
		},
		{
			Risk:               form.TRiskSecondary,
			PastRollingPeriods: form.SecondaryPeriods,
		},
	}

	// Make the Issue API call.
	request := api.IssuePINRequest{
		ValidFor: 1 * time.Hour,
		Risks:    risks,
	}
	var response api.IssuePINResponse

	url := ic.apiServer + "/api/issue"
	if err := jsonclient.MakeRequest(ic.client, url, request, &response); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.HTML(http.StatusOK, "issue", gin.H{"pincode": response.PIN, "error": response.Error})
}
