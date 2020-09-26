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

const (
	OSTypeInvalid OSType = iota
	OSTypeIOS
	OSTypeAndroid
)

type MobileApp struct {
	gorm.Model
	Errorable

	// Name is the name of the app.
	Name string `gorm:"column:name; type:citext; unique_index:realm_app_name;"`

	// RealmID is the id of the mobile app.
	RealmID uint `gorm:"column:realm_id; unique_index:realm_app_name;"`

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

	// Ensure OS is valid
	if a.OS < OSTypeIOS || a.OS > OSTypeAndroid {
		a.AddError("os", "is invalid")
	}

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

// ListActiveAppsByOS finds all authorized by their OS.
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

// CreateMobileApp adds a mobile app to the DB.
func (r *Realm) CreateMobileApp(db *Database, app *MobileApp) error {
	app.RealmID = r.ID
	return db.db.Save(&app).Error
}

// SaveMobileApp saves the authorized app.
func (db *Database) SaveMobileApp(r *MobileApp) error {
	return db.db.Save(r).Error
}

// Disable disables the mobile app.
func (a *MobileApp) Disable(db *Database) error {
	return db.db.Delete(a).Error
}

// Enable enables the mobile app.
func (a *MobileApp) Enable(db *Database) error {
	return db.db.Unscoped().Model(a).Update("deleted_at", nil).Error
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
