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

package realmadmin

import (
	"context"
	"encoding/csv"
	"net/http"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/cache"
	"github.com/google/exposure-notifications-verification-server/pkg/controller"
	"github.com/google/exposure-notifications-verification-server/pkg/database"
)

var cacheTimeout = 5 * time.Minute

// ResultType specifies which type of renderer you want.
type ResultType int

const (
	HTML ResultType = iota
	JSON
	CSV
)

// wantUser returns true if we want per-user requests.
func wantUser(r *http.Request) bool {
	_, has := r.URL.Query()["user"]
	return has
}

// wantAudit returns true if we want audit requests.
func wantAudit(r *http.Request) bool {
	_, has := r.URL.Query()["audit"]
	return has
}

// getRealmStats returns the realm stats for a given date range.
func (c *Controller) getRealmStats(ctx context.Context, realm *database.Realm, now, past time.Time) ([]*database.RealmStats, error) {
	var stats []*database.RealmStats
	cacheKey := &cache.Key{
		Namespace: "stats:realm",
		Key:       strconv.FormatUint(uint64(realm.ID), 10),
	}
	if err := c.cacher.Fetch(ctx, cacheKey, &stats, cacheTimeout, func() (interface{}, error) {
		return realm.Stats(c.db, past, now)
	}); err != nil {
		return nil, err
	}

	return stats, nil
}

// getUserStats gets the per-user realm stats for a given date range.
func (c *Controller) getUserStats(ctx context.Context, realm *database.Realm, now, past time.Time) ([]*database.RealmUserStats, error) {
	var userStats []*database.RealmUserStats
	cacheKey := &cache.Key{
		Namespace: "stats:realm:per_user",
		Key:       strconv.FormatUint(uint64(realm.ID), 10),
	}
	if err := c.cacher.Fetch(ctx, cacheKey, &userStats, cacheTimeout, func() (interface{}, error) {
		return realm.CodesPerUser(c.db, past, now)
	}); err != nil {
		return nil, err
	}
	return userStats, nil
}

// getAuditStats gets the audit-id stats for a given date range.
func (c *Controller) getAuditStats(ctx context.Context, realm *database.Realm, now, past time.Time) ([]*database.AuditIDStat, error) {
	var stats []*database.AuditIDStat
	cacheKey := &cache.Key{
		Namespace: "stats:realm:audit_id",
		Key:       strconv.FormatUint(uint64(realm.ID), 10),
	}
	if err := c.cacher.Fetch(ctx, cacheKey, &stats, cacheTimeout, func() (interface{}, error) {
		return realm.AuditIDStats(c.db, past, now)
	}); err != nil {
		return nil, err
	}
	return stats, nil
}

func (c *Controller) HandleShow(result ResultType) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		now := time.Now().UTC()
		past := now.Add(-30 * 24 * time.Hour)

		realm := controller.RealmFromContext(ctx)
		if realm == nil {
			controller.MissingRealm(w, r, c.h)
			return
		}

		// Get the realm stats.
		stats, err := c.getRealmStats(ctx, realm, now, past)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// Also get the per-user stats.
		userStats, err := c.getUserStats(ctx, realm, now, past)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		// And the audit-id stats.
		auditStats, err := c.getAuditStats(ctx, realm, now, past)
		if err != nil {
			controller.InternalError(w, r, c.h, err)
			return
		}

		switch result {
		case CSV:
			err = c.renderCSV(r, w, stats, userStats, auditStats)
		case JSON:
			err = c.renderJSON(r, w, stats, userStats, auditStats)
		case HTML:
			err = c.renderHTML(ctx, w, realm, stats, userStats, auditStats)
		}
		if err != nil {
			controller.InternalError(w, r, c.h, err)
		}
	})
}

// formatPerUserStats formats a slice of RealmUserStats into a format more conducive
// to charting in Javascript.
func formatPerUserStats(userStats []*database.RealmUserStats) ([]string, [][]interface{}) {
	// We need to format the per-user-per-day data properly for the charts.
	// Create some LUTs to make this easier.
	nameLUT := make(map[string]int)
	datesLUT := make(map[time.Time]int)
	for _, stat := range userStats {
		if _, ok := nameLUT[stat.Name]; !ok {
			nameLUT[stat.Name] = len(nameLUT)
		}
		if _, ok := datesLUT[stat.Date]; !ok {
			datesLUT[stat.Date] = len(datesLUT)
		}
	}

	// Figure out the names.
	names := make([]string, len(nameLUT))
	for name, i := range nameLUT {
		names[i] = name
	}

	// And combine up the data we want to send as well.
	data := make([][]interface{}, len(datesLUT))
	for date, i := range datesLUT {
		data[i] = make([]interface{}, len(names)+1)
		data[i][0] = date.Format("Jan 2 2006")
	}
	for _, stat := range userStats {
		i := datesLUT[stat.Date]
		data[i][nameLUT[stat.Name]+1] = stat.CodesIssued
	}

	// Now, we need to format the data properly.
	return names, data
}

type AuditStatsPerDay struct {
	Date        time.Time
	CodesIssued uint
}

// formatAuditStats formats audit data for charting.
func formatAuditStats(stats []*database.AuditIDStat) ([]string, [][]interface{}, []AuditStatsPerDay) {
	idSet := make(map[string]int)
	dates := make(map[time.Time]int)
	for _, stat := range stats {
		if _, ok := idSet[stat.AuditID]; !ok {
			idSet[stat.AuditID] = len(idSet)
		}
		if _, ok := dates[stat.Date]; !ok {
			dates[stat.Date] = len(dates)
		}
	}

	// Get the ids
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}

	// Generate the charting data.
	data := make([][]interface{}, len(dates))
	perDate := make([]AuditStatsPerDay, len(dates))
	for date, i := range dates {
		data[i] = make([]interface{}, len(ids)+1)
		data[i][0] = date.Format("Jan 2 2006")
		perDate[i].Date = date
	}
	for _, stat := range stats {
		i := dates[stat.Date]
		data[i][idSet[stat.AuditID]+1] = stat.CodesIssued
		perDate[i].CodesIssued += stat.CodesIssued
	}

	return ids, data, perDate
}

func (c *Controller) renderHTML(ctx context.Context, w http.ResponseWriter, realm *database.Realm, stats []*database.RealmStats, userStats []*database.RealmUserStats, auditIDStats []*database.AuditIDStat) error {
	userNames, formattedUser := formatPerUserStats(userStats)
	auditIDs, formattedAudit, auditPerDay := formatAuditStats(auditIDStats)
	m := controller.TemplateMapFromContext(ctx)
	m.Title("Realm stats")
	m["user"] = realm
	m["stats"] = stats
	m["userNames"] = userNames
	m["userStats"] = formattedUser
	m["auditIDs"] = auditIDs
	m["auditStats"] = formattedAudit
	m["auditPerDay"] = auditPerDay
	c.h.RenderHTML(w, "realmadmin/show", m)

	return nil
}

// renderCSV renders a CSV response.
func (c *Controller) renderCSV(r *http.Request, w http.ResponseWriter, stats []*database.RealmStats, userStats []*database.RealmUserStats, auditIDStats []*database.AuditIDStat) error {
	wr := csv.NewWriter(w)
	defer wr.Flush()

	// Check which type of stats we are displaying.
	if wantUser(r) {
		if err := wr.Write(database.RealmUserStatsCSVHeader); err != nil {
			return err
		}

		for _, u := range userStats {
			if err := wr.Write(u.CSV()); err != nil {
				return err
			}
		}
	} else if wantAudit(r) {
		if err := wr.Write(database.AuditIDStatsCSVHeader); err != nil {
			return err
		}

		for _, a := range auditIDStats {
			if err := wr.Write(a.CSV()); err != nil {
				return err
			}
		}
	} else {
		if err := wr.Write(database.RealmStatsCSVHeader); err != nil {
			return err
		}

		for _, s := range stats {
			if err := wr.Write(s.CSV()); err != nil {
				return err
			}
		}
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=stats.csv")
	return nil
}

// renderJSON renders a JSON response.
func (c *Controller) renderJSON(r *http.Request, w http.ResponseWriter, stats []*database.RealmStats, userStats []*database.RealmUserStats, auditIDStats []*database.AuditIDStat) error {
	if wantUser(r) {
		c.h.RenderJSON(w, http.StatusOK, userStats)
	} else if wantAudit(r) {
		c.h.RenderJSON(w, http.StatusOK, auditIDStats)
	} else {
		c.h.RenderJSON(w, http.StatusOK, stats)
	}
	w.Header().Set("Content-Disposition", "attachment;filename=stats.json")
	return nil
}
