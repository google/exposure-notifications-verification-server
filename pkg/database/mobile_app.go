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
	"github.com/jinzhu/gorm"
)

type OSType int

const (
	OSTypeIOS OSType = iota
	OSTypeAndroid
)

type MobileApp struct {
	gorm.Model
	Errorable

	// Name is the name of the app.
	Name string `gorm:"name,varchar(512);unique_index:realm_app_name"`

	// RealmID is the id of the mobile app.
	RealmID uint `gorm:"unique_index:realm_app_name"`

	// OS is the type of the application we're using (eg, iOS, Android).
	OS OSType `gorm:"os,type:int"`

	// IOSAppID is a unique string representing the app.
	AppID string `gorm:"app_id,type:varchar(512)"`

	// SHA is a unique hash of the app.
	// It is only present for Android devices, and should be of the form:
	//   AA:BB:CC:DD...
	SHA string `gorm:"sha,type:text"`
}

// ListActiveAppsByOS finds all authorized by their OS.
func (db *Database) ListActiveAppsByOS(os OSType) ([]*MobileApp, error) {
	// Find the apps.
	var apps []*MobileApp
	if err := db.db.
		Model(&MobileApp{}).
		Where("os = ?", os).
		Where("deleted_at IS NULL").
		Find(&apps).
		Error; err != nil {
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
	if r.Model.ID == 0 {
		return db.db.Create(r).Error
	}
	return db.db.Save(r).Error
}

// Disable disables the mobile app.
func (a *MobileApp) Disable(db *Database) error {
	if err := db.db.Delete(a).Error; err != nil {
		return err
	}
	return nil
}

// Enable enables the mobile app.
func (a *MobileApp) Enable(db *Database) error {
	if err := db.db.Unscoped().Model(a).Update("deleted_at", nil).Error; err != nil {
		return err
	}
	return nil
}
