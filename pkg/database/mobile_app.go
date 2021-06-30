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

package database

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-verification-server/internal/project"
	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/jinzhu/gorm"
)

type OSType int

// Display prints the name of the OSType. Note this CANNOT be named String()
// because, if it is, Go's text/template package will automatically call
// String() and cause you to lose hours upon hours of your life debuggin when
// forms are suddenly broken.
func (o OSType) Display() string {
	switch o {
	case OSTypeUnknown:
		return "Unknown"
	case OSTypeIOS:
		return "iOS"
	case OSTypeAndroid:
		return "Android"
	default:
		return "Unknown"
	}
}

func (o OSType) Len() int {
	return 3
}

func (o OSType) IsAndroid() bool {
	return o == OSTypeAndroid
}

func (o OSType) IsIOS() bool {
	return o == OSTypeIOS
}

const (
	OSTypeUnknown OSType = iota
	OSTypeIOS
	OSTypeAndroid
)

var _ Auditable = (*MobileApp)(nil)

type MobileApp struct {
	gorm.Model
	Errorable

	// Name is the name of the app.
	Name string `gorm:"column:name; type:citext;"`

	// RealmID is the id of the mobile app.
	RealmID uint `gorm:"column:realm_id;"`

	// URL is the link to the app in it's appstore.
	URL    string  `gorm:"-"`
	URLPtr *string `gorm:"column:url; type:text"`

	// DisableRedirect disables URL redirection in the redirector service for this
	// app.
	DisableRedirect bool `gorm:"column:disable_redirect; type:bool; default:false; not null"`

	// OS is the type of the application we're using (eg, iOS, Android).
	OS OSType `gorm:"column:os; type:int;"`

	// Headless indicates that this an and android EN Express headless app.
	// This is only settable through the app sync service.
	Headless bool `gorm:"column:headless; type:bool; default:false; not null"`

	// AppID is a unique string representing the app.
	//
	// For iOS this should include the team ID or app ID prefix followed by
	// the bundle ID. eg. ABCD1234.com.google.test.application
	AppID string `gorm:"column:app_id; type:varchar(512);"`

	// SHA is a unique hash of the app.
	// It is only present for Android devices, and should be of the form:
	//   AA:BB:CC:DD...
	SHA string `gorm:"column:sha; type:text;"`
}

func (a *MobileApp) BeforeSave(tx *gorm.DB) error {
	a.Name = project.TrimSpace(a.Name)
	if a.Name == "" {
		a.AddError("name", "cannot be blank")
	}

	if a.RealmID == 0 {
		a.AddError("realm_id", "is required")
	}

	a.AppID = project.TrimSpace(a.AppID)
	if a.AppID == "" {
		a.AddError("app_id", "cannot be blank")
	}

	a.URL = project.TrimSpace(a.URL)
	a.URLPtr = stringPtr(a.URL)

	// Ensure OS is valid
	if a.OS < OSTypeIOS || a.OS > OSTypeAndroid {
		a.AddError("os", "is invalid")
	}

	// SHA is required for Android
	a.SHA = project.TrimSpace(a.SHA)
	if a.OS == OSTypeAndroid {
		if a.SHA == "" {
			a.AddError("sha", "cannot be blank for Android apps")
		}
	}

	// Process and clean SHAs
	var shas []string
	for _, line := range strings.Split(a.SHA, "\n") {
		for _, entry := range strings.Split(line, ",") {
			entry = strings.ToUpper(project.TrimSpace(entry))
			if entry == "" {
				continue
			}

			if got, want := len(entry), 95; got != want {
				a.AddError("sha", fmt.Sprintf("must be %d characters (got %d)", want, got))
				continue
			}

			if entry != "" {
				shas = append(shas, entry)
			}
		}
	}
	a.SHA = strings.Join(shas, "\n")

	return a.ErrorOrNil()
}

func (a *MobileApp) AfterFind(tx *gorm.DB) error {
	a.URL = stringValue(a.URLPtr)
	return nil
}

// ExtendedMobileApp combines a MobileApp with its Realm
type ExtendedMobileApp struct {
	MobileApp
	Realm
}

// ListActiveAppsWithRealm finds all active mobile apps with their associated realm.
func (db *Database) ListActiveAppsWithRealm(p *pagination.PageParams) ([]*ExtendedMobileApp, *pagination.Paginator, error) {
	return db.SearchActiveAppsWithRealm(p, "")
}

// SearchActiveAppsWithRealm finds all active mobile apps with their associated realm.
func (db *Database) SearchActiveAppsWithRealm(p *pagination.PageParams, q string) ([]*ExtendedMobileApp, *pagination.Paginator, error) {
	query := db.db.Table("mobile_apps").
		Select("mobile_apps.*, realms.*").
		Joins("left join realms on realms.id = mobile_apps.realm_id")

	q = project.TrimSpace(q)
	if q != "" {
		q = `%` + q + `%`
		query = query.Where("(mobile_apps.name ILIKE ? OR realms.name ILIKE ?)", q, q)
	}

	if p == nil {
		p = new(pagination.PageParams)
	}

	apps := make([]*ExtendedMobileApp, 0)

	paginator, err := PaginateFn(query, p.Page, p.Limit, func(query *gorm.DB, offset uint64) error {
		rows, err := query.
			Limit(p.Limit).
			Offset(offset).
			Rows()
		if err != nil || rows == nil {
			return nil
		}
		defer rows.Close()

		for rows.Next() {
			app := &ExtendedMobileApp{}
			if err := db.db.ScanRows(rows, &app); err != nil {
				return err
			}
			apps = append(apps, app)
		}
		return nil
	})

	return apps, paginator, err
}

// ListActiveApps finds mobile apps by their realm.
func (db *Database) ListActiveApps(realmID uint, scopes ...Scope) ([]*MobileApp, error) {
	// Find the apps.
	var apps []*MobileApp
	if err := db.db.
		Model(&MobileApp{}).
		Scopes(scopes...).
		Where("realm_id = ?", realmID).
		Find(&apps).
		Error; err != nil {
		if IsNotFound(err) {
			return apps, nil
		}
		return nil, err
	}
	return apps, nil
}

// SaveMobileApp saves the mobile app.
func (db *Database) SaveMobileApp(a *MobileApp, actor Auditable) error {
	if a == nil {
		return fmt.Errorf("provided mobile app is nil")
	}

	if actor == nil {
		return fmt.Errorf("auditing actor is nil")
	}

	return db.db.Transaction(func(tx *gorm.DB) error {
		var audits []*AuditEntry

		var existing MobileApp
		if err := tx.
			Unscoped().
			Model(&MobileApp{}).
			Where("id = ?", a.ID).
			First(&existing).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to get existing mobile app: %w", err)
		}

		// Save the app
		if err := tx.Unscoped().Save(a).Error; err != nil {
			return err
		}

		// Brand new app?
		if existing.ID == 0 {
			audit := BuildAuditEntry(actor, "created mobile app", a, a.RealmID)
			audits = append(audits, audit)
		} else {
			if existing.Name != a.Name {
				audit := BuildAuditEntry(actor, "updated mobile app name", a, a.RealmID)
				audit.Diff = stringDiff(existing.Name, a.Name)
				audits = append(audits, audit)
			}

			if existing.OS != a.OS {
				audit := BuildAuditEntry(actor, "updated mobile app os", a, a.RealmID)
				audit.Diff = stringDiff(existing.OS.Display(), a.OS.Display())
				audits = append(audits, audit)
			}

			if existing.AppID != a.AppID {
				audit := BuildAuditEntry(actor, "updated mobile app appID", a, a.RealmID)
				audit.Diff = stringDiff(existing.AppID, a.AppID)
				audits = append(audits, audit)
			}

			if existing.SHA != a.SHA {
				audit := BuildAuditEntry(actor, "updated mobile app sha", a, a.RealmID)
				audit.Diff = stringDiff(existing.SHA, a.SHA)
				audits = append(audits, audit)
			}

			if existing.DeletedAt != a.DeletedAt {
				audit := BuildAuditEntry(actor, "updated mobile app enabled", a, a.RealmID)
				audit.Diff = boolDiff(existing.DeletedAt == nil, a.DeletedAt == nil)
				audits = append(audits, audit)
			}
		}

		// Save all audits
		for _, audit := range audits {
			if err := tx.Save(audit).Error; err != nil {
				return fmt.Errorf("failed to save audits: %w", err)
			}
		}

		return nil
	})
}

func (a *MobileApp) AuditID() string {
	return fmt.Sprintf("mobile_apps:%d", a.ID)
}

func (a *MobileApp) AuditDisplay() string {
	return fmt.Sprintf("%s (%s)", a.Name, a.OS.Display())
}

// PurgeMobileApps will delete mobile apps that have been deleted for more than
// the specified time.
func (db *Database) PurgeMobileApps(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)

	result := db.db.
		Unscoped().
		Where("deleted_at IS NOT NULL AND deleted_at < ?", deleteBefore).
		Delete(&MobileApp{})
	return result.RowsAffected, result.Error
}
