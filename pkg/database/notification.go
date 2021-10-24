// Copyright 2021 the Exposure Notifications Verification Server authors
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
	"github.com/jinzhu/gorm"
)

var _ Auditable = (*Notification)(nil)

type NotificationCategory uint

const (
	NotificationGeneric NotificationCategory = iota
	NotificationAbuseLimitReached

	// This entry must always be last.
	notificationCeiling
)

// Notification respresents a realm notification and the delivery status
type Notification struct {
	gorm.Model
	Errorable

	// RealmID indicates the realm this notification is for (will be send to NotificationPhones)
	RealmID uint `gorm:"column:realm_id; type: integer;"`

	// Category represents the category of this notification, used for ensuring the same
	// category isn't sent too close together.
	Category NotificationCategory `gorm:"column:category; type: integer;"`

	// NotBefore indicates the earliest time that another notifications for this {realm, category}
	// can be delivered at.
	NotBefore *time.Time `gorm:"column:not_before; type: timestamp"`

	// Message is the actual text that will be sent.
	Message string `gorm:"column:message; type: text;"`

	// Delivered indicates if this message
	Delivered bool `gorm:"column:delivered; type: boolean"`

	// DeliveryStatus indicates individual delivery status for phone numbers
	DeliveryStatus string `gorm:"column:delivery_status; type text;"`
}

func notBeforeTime(cat NotificationCategory) *time.Time {
	now := time.Now().UTC()
	switch cat {
	case NotificationGeneric:
		return nil
	case NotificationAbuseLimitReached:
		// If the abuse limit triggers, send that at most once an hour.
		nbf := now.Add(time.Hour)
		return &nbf
	default:
		return nil
	}
}

// NewNotifcation creates a notification that can be schedule into a specific realm.
// The category determines the not before time.
func NewNotification(r *Realm, cat NotificationCategory, message string) *Notification {
	return &Notification{
		RealmID:        r.ID,
		Category:       cat,
		NotBefore:      notBeforeTime(cat),
		Message:        message,
		Delivered:      false,
		DeliveryStatus: "",
	}
}

func (n *Notification) BeforeSave(tx *gorm.DB) error {
	if n.RealmID == 0 {
		n.AddError("realm_id", "must be set")
	}

	n.Message = project.TrimSpace(n.Message)
	if n.Message == "" {
		n.AddError("message", "cannot be blank")
	}

	if n.Category < NotificationGeneric || n.Category >= notificationCeiling {
		n.AddError("category", "invalid category")
	}

	return n.ErrorOrNil()
}

func (n *Notification) AuditID() string {
	return fmt.Sprintf("notification:%d", n.ID)
}

func (n *Notification) AuditDisplay() string {
	return fmt.Sprintf("%q", n.Message)
}

// MarkDelivered will set a notification as delivered=true and update
// the delivery status to the provided status.
func (n *Notification) MarkDelivered(db *Database, status []string) error {
	if n == nil {
		return fmt.Errorf("provided notification is nil")
	}

	n.Delivered = true
	n.DeliveryStatus = strings.Join(status, "\n")

	return db.db.Save(n).Error
}

// ListRealmNotifications selects all non-deleted notifications for a realm,
// so that an admin can see what was sent.
func (db *Database) ListRealmNotifications(realmId uint) ([]*Notification, error) {
	var notifications []*Notification
	if err := db.db.
		Model(&Notification{}).
		Where("realm_id = ?", realmId).
		Order("created_at DESC").
		Find(&notifications).
		Error; err != nil {
		return nil, err
	}

	return notifications, nil
}

// DeleteNotifications marks for deletion old notifications, thus preventing
// their display. This is a soft delete, and PurgeNotifications must be
// called to remove from the database.
func (db *Database) DeleteNotifications(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)
	rtn := db.db.
		Where("created_at < ? AND deleted_at IS NULL", deleteBefore).
		Delete(&Notification{})
	return rtn.RowsAffected, rtn.Error
}

// PurgeNotifications removed notifications that have been soft deleted
// for the maxAge.
func (db *Database) PurgeNotifications(maxAge time.Duration) (int64, error) {
	if maxAge > 0 {
		maxAge = -1 * maxAge
	}
	deleteBefore := time.Now().UTC().Add(maxAge)
	rtn := db.db.Unscoped().
		Where("deleted_at < ?", deleteBefore).
		Delete(&Notification{})
	return rtn.RowsAffected, rtn.Error
}

// SelectNotifications returns up to limit notifications
// that still need to be dispatched. The caller should obtain a
// separate lock using the locking mechanism before actually
// delivering the message(s).
func (db *Database) SelectNotifications(limit int) ([]*Notification, error) {
	if limit < 1 {
		return nil, fmt.Errorf("limit must be a positive integer")
	}

	// Select codes that haven't been delivered yet.
	// This is not realm specific.
	var notifications []*Notification
	if err := db.db.
		Model(&Notification{}).
		Where("delivered = ?", false).
		Order("created_at ASC").
		Limit(limit).
		Find(&notifications).
		Error; err != nil {
		return nil, err
	}

	return notifications, nil
}

// SendNotification schedules a notification to be sent. The actual
// sending is done by an asynchronous process.
func (db *Database) ScheduleNotification(n *Notification, actor Auditable) error {
	if n == nil {
		return fmt.Errorf("provided notification is nil")
	}
	if actor == nil {
		return fmt.Errorf("auditing actor is nil")
	}
	// Ensure that we always enqueue notifications as not delivered.
	n.Delivered = false
	// Current time for NBF calculations.
	now := time.Now().UTC()

	return db.db.Transaction(func(tx *gorm.DB) error {
		// See if there is potentially another notification preventing this one from being sent.
		var lastNotification Notification
		if err := tx.Model(&Notification{}).
			Where("realm_id = ? and category = ? and not_before IS NOT NULL", n.RealmID, n.Category).
			Order("not_before DESC").
			First(&lastNotification).
			Error; err != nil && !IsNotFound(err) {
			return fmt.Errorf("failed to lookup previous notifications in this category: %w", err)
		} else if err == nil && lastNotification.NotBefore.After(now) {
			return fmt.Errorf("notification of category %d cannot be scheduled for this realm until: %v", n.Category, lastNotification.NotBefore.Format(time.RFC3339))
		}

		// Create / schedule the notification.
		if err := tx.Unscoped().Create(n).Error; err != nil {
			return err
		}

		// Write audit record.
		auditEntry := BuildAuditEntry(actor, "scheduled notification", n, n.RealmID)
		if err := tx.Save(auditEntry).Error; err != nil {
			return fmt.Errorf("failed to save audit entry: %w", err)
		}

		return nil
	})
}
