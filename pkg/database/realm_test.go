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

import "testing"

func TestSMS(t *testing.T) {
	realm := NewRealmWithDefaults("test")
	realm.RegionCode = "US-WA"

	{
		got := realm.BuildSMSText("12345678", "abcdefgh12345678")
		want := "This is your Exposure Notifications Verification code: ens://v?r=US-WA&c=abcdefgh12345678 Expires in 24 hours"
		if got != want {
			t.Errorf("SMS text wrong, want: %q got %q", want, got)
		}
	}

	{
		realm.SMSTextTemplate = "State of Wonder, Covid-19 Exposure Verification code [code]. Expires in [expires] minutes. Act now!"
		got := realm.BuildSMSText("654321", "asdflkjasdlkfjl")
		want := "State of Wonder, Covid-19 Exposure Verification code 654321. Expires in 15 minutes. Act now!"
		if got != want {
			t.Errorf("SMS text wrong, want: %q got %q", want, got)
		}
	}
}
