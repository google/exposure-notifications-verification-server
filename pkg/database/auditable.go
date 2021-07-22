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

// Auditable represents a resource that can be audited as an actor or actee.
type Auditable interface {
	// AuditID returns the id for this resource as it will be stored in audit
	// logs. This ID is usually of the format `table:id`.
	AuditID() string

	// AuditDisplay returns how this resource should appear in audit logs.
	AuditDisplay() string
}

// BuildAuditEntry builds an AuditEntry from the given parameters. For actions
// that don't take place on a realm, use a realmID of 0.
func BuildAuditEntry(actor Auditable, action string, target Auditable, realmID uint) *AuditEntry {
	var e AuditEntry
	e.RealmID = realmID
	e.ActorID = actor.AuditID()
	e.ActorDisplay = actor.AuditDisplay()
	e.Action = action
	e.TargetID = target.AuditID()
	e.TargetDisplay = target.AuditDisplay()
	return &e
}
