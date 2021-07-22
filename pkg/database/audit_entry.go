// Copyright 2020 the Exposure Notifications Verification Server authors
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
	"time"

	"github.com/google/exposure-notifications-verification-server/pkg/pagination"
	"github.com/jinzhu/gorm"
)

// AuditEntry represents an event in the system. These records are purged after
// a configurable number of days by the cleanup job. The AuditEntry specifically
// does NOT make use of foreign keys or relationships to avoid breaking an audit
// entry if the upstream data which was audited is removed or changed. These
// records should be considered immutable.
type AuditEntry struct {
	Errorable

	// ID is the entry's ID.
	ID uint `gorm:"primary_key;"`

	// RealmID is the ID of the realm against which this event took place, if the
	// event took place against a realm. This can be 0 in situations where the
	// event took place outside of a realm (e.g. user creation), which means it's
	// a system event.
	RealmID uint `gorm:"column:realm_id; type:integer; not null;"`

	// ActorID is the ID of the actor which performed this event. It's usually of
	// the form `model:id` (e.g. users:1), but there's no guarantee that the
	// underlying resource still exists when the audit is read. It's primarily
	// used for sorting/filtering where an audit viewer wants to see all events a
	// particular entity took.
	//
	// ActorDisplay is the display name of the actor. The actor defines how it
	// will be displayed in audit logs.
	ActorID      string `gorm:"column:actor_id; type:text; not null;"`
	ActorDisplay string `gorm:"column:actor_display; type:text; not null;"`

	// Action is the auditable action.
	Action string `gorm:"column:action; type:text; not null;"`

	// TargetID and TargetDisplay are the same as the actor, but are for the
	// target of the action.
	TargetID      string `gorm:"column:target_id; type:text; not null;"`
	TargetDisplay string `gorm:"column:target_display; type:text; not null;"`

	// Diff is the change of structure that occurred, if any.
	Diff string `gorm:"column:diff; type:text;"`

	// CreatedAt is when the entry was created.
	CreatedAt time.Time
}

// BeforeSave runs validations. If there are errors, the save fails.
func (a *AuditEntry) BeforeSave(tx *gorm.DB) error {
	if a.ActorID == "" {
		a.AddError("actor_id", "cannot be blank")
	}
	if a.ActorDisplay == "" {
		a.AddError("actor_display", "cannot be blank")
	}

	if a.Action == "" {
		a.AddError("action", "cannot be blank")
	}

	if a.TargetID == "" {
		a.AddError("target_id", "cannot be blank")
	}
	if a.TargetDisplay == "" {
		a.AddError("target_display", "cannot be blank")
	}

	return a.ErrorOrNil()
}

// SaveAuditEntry saves the audit entry.
func (db *Database) SaveAuditEntry(a *AuditEntry) error {
	return db.db.Save(a).Error
}

// PurgeAuditEntries will delete audit entries which were created longer than
// maxAge ago.
func (db *Database) PurgeAuditEntries(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	createdBefore := time.Now().UTC().Add(maxAge)

	result := db.db.
		Unscoped().
		Where("created_at < ?", createdBefore).
		Delete(&AuditEntry{})
	return result.RowsAffected, result.Error
}

// ListAudits returns the list audit events which match the given criteria.
// Warning: This list may be large. Use Realm.Audits() to get users scoped to a realm.
func (db *Database) ListAudits(p *pagination.PageParams, scopes ...Scope) ([]*AuditEntry, *pagination.Paginator, error) {
	var entries []*AuditEntry

	query := db.db.
		Model(&AuditEntry{}).
		Scopes(scopes...).
		Order("created_at DESC")

	if p == nil {
		p = new(pagination.PageParams)
	}

	paginator, err := Paginate(query, &entries, p.Page, p.Limit)
	if err != nil {
		if IsNotFound(err) {
			return entries, nil, nil
		}
		return nil, nil, err
	}

	return entries, paginator, nil
}
