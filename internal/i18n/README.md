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

1.  Update the `msgstr` fields to the new values.

1.  Save the file.

1.  Submit a pull request.
