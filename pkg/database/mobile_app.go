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

	"github.com/jinzhu/gorm"
)

type OSType int

// Display prints the name of the OSType. Note this CANNOT be named String()
// because, if it is, Go's text/template package will automatically call
// String() and cause you to lose hours upon hours of your life debuggin when
// forms are suddenly broken.
func (o OSType) Display() string {
	switch o {
	case OSTypeIOS:
		return "iOS"
	case OSTypeAndroid:
		return "Android"
	default:
		return "Unknown"
	}
}

const (
	OSTypeInvalid OSType = iota
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

	// OS is the type of the application we're using (eg, iOS, Android).
	OS OSType `gorm:"column:os; type:int;"`

	// IOSAppID is a unique string representing the app.
	AppID string `gorm:"column:app_id; type:varchar(512);"`

	// SHA is a unique hash of the app.
	// It is only present for Android devices, and should be of the form:
	//   AA:BB:CC:DD...
	SHA string `gorm:"column:sha; type:text;"`
}

func (a *MobileApp) BeforeSave(tx *gorm.DB) error {
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" {
		a.AddError("name", "is required")
	}

	if a.RealmID == 0 {
		a.AddError("realm_id", "is required")
	}

	a.AppID = strings.TrimSpace(a.AppID)
	if a.AppID == "" {
		a.AddError("app_id", "is required")
	}

	a.URL = strings.TrimSpace(a.URL)
	a.URLPtr = stringPtr(a.URL)
	if a.URL == "" {
		a.AddError("url", "is required")
	}

	// Ensure OS is valid
	if a.OS < OSTypeIOS || a.OS > OSTypeAndroid {
		a.AddError("os", "is invalid")
	}

	// SHA is required for Android
	a.SHA = strings.TrimSpace(a.SHA)
	if a.OS == OSTypeAndroid {
		if a.SHA == "" {
			a.AddError("sha", "is required for Android apps")
		}
	}

	// Process and clean SHAs
	var shas []string
	for _, line := range strings.Split(a.SHA, "\n") {
		for _, entry := range strings.Split(line, ",") {
			entry = strings.ToUpper(strings.TrimSpace(entry))
			if entry == "" {
				continue
			}

			if len(entry) != 95 {
				a.AddError("sha", "is not 95 characters")
				continue
			}

			if entry != "" {
				shas = append(shas, entry)
			}
		}
	}
	a.SHA = strings.Join(shas, "\n")

	if len(a.Errors()) > 0 {
		return fmt.Errorf("validation failed")
	}

	return nil
}

// ListActiveAppsByOS finds all mobile apps by their OS.
func (db *Database) ListActiveAppsByOS(os OSType) ([]*MobileApp, error) {
	// Find the apps.
	var apps []*MobileApp
	if err := db.db.
		Model(&MobileApp{}).
		Where("os = ?", os).
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
			return fmt.Errorf("failed to get existing mobile app")
		}

		// Save the app
		if err := tx.Unscoped().Save(a).Error; err != nil {
			return fmt.Errorf("failed to save mobile app: %w", err)
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

func (a *MobileApp) AfterFind(tx *gorm.DB) error {
	a.URL = stringValue(a.URLPtr)
	return nil
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
