# Internationalization

## New language

To localize into a new language:

1.  Create a folder in `locales/` with the ISO 639-1 (preferred) or ISO 639-2
    code.

1.  Copy the canonical example from `locales/en/default.po`:

    ```text
    cp locales/en/default.po locales/<lang>/default.po
    ```

1.  Translate the `msgstr` values. Do **not** change other values in the file.

1.  Save the file and submit a pull request.


## Update an existing language

To update an existing language:

1.  Find the language in the `locales/` folder.

1.  Update the language on line 3.

1.  Update the `msgstr` fields to the new values.

1.  Save the file.

1.  Submit a pull request.


## Terms

-   **realm** - public health authority, state, or hospital system. The system
    is multi-tenant, and a user can be a member of multiple realms. For example,
    the same doctor might operate clinics across state lines. In the US, each
    state is a "realm". In other areas, a realm could be a country or region.

-   **case worker** - someone at a health clinic, testing facility, doctor's
    office, etc that is managing cases related to COVID-19.

-   **patient** - the person who is being given a test result for COVID-19.

-   **short code** - 6-8 digit (numeric-only) code that is
    dictated to a patient by a case worker over the phone.

-   **long code** - 16+ alpha-numeric code that is sent to the patient via an
    SMS text message. This requires the patient to provide a phone number. If no
    phone number is provided, no long code is generated.

-   **backup code** - 6-8 digit (numeric-only) code that is dictated to a
    patient by a case worker over the phone **if and only if** the long code
    fails to dispatch (e.g. due to network issues). This requires the patient to
    provide a phone number. If no phone number is provided, no backup code is
    generated.

-   **signing key** - this refers to an asymmetric signing key (cryptography).
