<!-- TOC -->autoauto- [Realm admin guide](#realm-admin-guide)auto  - [Access protection recommendations](#access-protection-recommendations)auto    - [Account protection](#account-protection)auto    - [API key protection](#api-key-protection)auto  - [Settings, enabling EN Express](#settings-enabling-en-express)auto  - [Settings, code settings](#settings-code-settings)auto    - [Bulk Issue Codes](#bulk-issue-codes)auto    - [Allowed Test Types](#allowed-test-types)auto    - [Date Configuration](#date-configuration)auto    - [Code Length & Expiration](#code-length--expiration)auto    - [SMS Text Template](#sms-text-template)auto  - [Settings, Twilio SMS credentials](#settings-twilio-sms-credentials)auto  - [Adding users](#adding-users)auto  - [API Keys](#api-keys)auto  - [Rotating certificate signing keys](#rotating-certificate-signing-keys)auto    - [Step 1 - Create a new signing key version](#step-1---create-a-new-signing-key-version)autoauto<!-- /TOC -->

# Realm admin guide

This guide provides high-level steps for realm administrators to follow.

If you are not a realm administrator, you will not have access to these screens.

## Access protection recommendations

### Account protection

We provide a base level of account protection measures that we urge you to share with your caseworkers that are issuing verification codes.

* All user accounts must verify ownership of their email address before using the system.
* Two-factor authentication (2FA) is available, we strongly suggest you require your users to enroll in 2FA
  using a mobile device under their sole control.
* Users should not share logins to the verification system.
* Users should only issue codes to people who have a verified COVID-19 diagnosis.

Realm administrators should monitor the number of codes issued and take corrective action if needed (suspend a user's access to issue codes)

### API key protection

* API keys should not be checked into source code.
* ADMIN level API Keys can issue codes, these should be closely guarded and their access should be monitored. Periodically, the API key should be rotated.


## Settings, enabling EN Express

Go to the realm setting by selecting the `settings` drop down menu (shown under your name).

![settings](images/admin/menu_settings.png "Click on your name and select 'settings'")

Under general settings, confirm the `Name` (display name only) and `Region code` settings.

The region code is important for `EN Express` customers and must match the
[ISO 3166-1 country codes and ISO 3166-2 subdivision codes](https://en.wikipedia.org/wiki/List_of_ISO_3166_country_codes)
for the geographic region that you cover.

![region code](images/admin/settings02.png "Confirm your region code")

Once that is confirmed and saved, click the `Enable EN Express` button.

![express](images/admin/settings03.png "Enable EN Express")

## Settings, code settings

Also under realm settings `settings` from the drop down menu, there are several settings for code issuance.

![express](images/admin/settings_code.png "Code settings")

### Bulk Issue Codes

  * Enabled

    A new tab is added to the realm that allows the issuance of many codes from a CSV file.
    This can be useful for case-workers who are given a data-set of test results rather than
    administering tests one-by-one.
    Details about how to bulk-issue codes [can be found here](/case-workeer-guide.md#bulk-issue-verification-codes).

  * Disabled

    Only the single issue-code tab will be shown. Calls to the batch issue API will fail
    for this realm.

### Allowed Test Types

  Realms may allow the following test result types from case workers.

  * Positive + Likely + Negative
  * Positive + Likely
  * Positive

  Although only `positive` and `likely` are used for matching exposure notifications on the client,
  `negative` is recommended for realms where the test result is shown to the user through the patient app.
  Showing all diagnosis - including `negative` through the app upon code issuance is a more powerful way to
  drive adoption of this system and can be more secure because the receipt of an SMS from this system does not
  reveal the diagnosis outcome.

### Date Configuration

Issuing codes have two date fields `testDate` and `symptomDate`. If this setting is marked `required`
the issuer must pass one or both of these dates. Case workers might ask for the date of symptom onset together
with the test, but when only `testDate` is given, apps are optionally recommended to prompt the user to enter
a date for first onset of symptoms - this may allow for more accurate matching of exposure.

If set to `optional`, codes may be issued successfully with no dates present.

### Code Length & Expiration

This setting adjusts the number of characters required for both long and short codes.
Realm admins may also define how long an issued code lasts before it expires. Once expired,
the patient will not longer be able to claim the diagnosis as theirs.

If EN Express is enabled, these fields are not adjustable.

Short codes are intended to be used where a case-worker may need to dictate the code to their patients
whereas long codes may be more secure for realms where they may be sent via SMS (but may be more difficult to dictate and recall).

### SMS Text Template

It is possible to customize the text of the SMS message that gets sent to patients.
See the help text on that page for guidance.

![sms text](images/admin/settings04.png "SMS Template")

The fields `[region]`, `[code]`, `[expires]`, `[longcode]`, and `[longexpires]` may be included with brackets
which will be programmatically substituted with values. It is recommended that the text of this SMS be composed
in such a way that is respectful to the patient and does not reveal details about their diagnosis to potential onlookers of the phone's notifications with further information presented in-app.

## Settings, Twilio SMS credentials

To dispatch verification codes / links over SMS, a realm must provide their credentials for [Twilio](https://www.twilio.com/). The necessary credentials (Twilio account, auth token, and phone number)
must be obtained from the Twilio console.

![smssettings](images/admin/sms01.png "SMS settings")

## Adding users

Go to realm users admin by selecting 'Users' from the drop-down menu (shown under your name).

![settings](images/admin/menu_users.png "Click on your name and select 'Users'")

Add users, by clicking on `create a new user`.

![users](images/admin/users01.png "User listing")

Enter the name of the user and the email address to add. The email address will need to be verified on the person's first login.

The admin checkbox indicates if this person should be made a realm admin (same powers that you have).
If a user only needs to be able to issue verification codes, they do not need to be a realm admin.

![users](images/admin/users02.png "User listing")

## API Keys

API Keys are used by your mobile app to access the verification server.
These API keys should be kept secret and only used by your mobile app.

![api keys](images/admin/menu_apikeys.png "Click on your name and select 'API Keys'")

Click the link to create a new API key.

![api keys](images/admin/apikeys01.png "Click on create a new API key")

Enter a name that indicates what this API key is for and select the type.
The `Device` type is the one that is needed by mobile apps.

When ready, click the `Create API key` button.

![api keys](images/admin/apikeys02.png "Create API key")

Once the API key is created, it will be displayed to you.
This is the __only__ time that this API key will be displayed.
If you fail to copy it, you will need to create another one.

![api keys](images/admin/apikeys03.png "API key created")

## Rotating certificate signing keys

Periodically, you will want to rotate the certificate signing key for your verification certificates.

This is done from the 'Signing Keys' screen.

![settings](images/admin/menu_signing.png "Click on your name and select 'Signing Keys'")

### Step 1 - Create a new signing key version

Click the "Create a new signing key version" button. This will _create_ but not make active a new key.

![api keys](images/admin/keys01.png "API key created")

If successful, you will get a message indicating the new key version that was created.

![api keys](images/admin/keys02.png "successful")

This keyID and the public key need to be communicated to your key sever operator.

![api keys](images/admin/keys03.png "successful")

When your key server operator confirms that this key is configured, you can click 'Activate.'

15 minutes after activating the new key, you can destroy the old version.
__Caution__: destroying the old key too early it may invalidate already issued, and still valid, certificate tokens.
