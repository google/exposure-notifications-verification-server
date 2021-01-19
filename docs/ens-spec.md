# Resource Identifier (RI) Scheme name: ens

## Scheme syntax

```text
ens://v?r=[region]&c=[verification code]
```

## Scheme semantics

Send COVID-19 exposure notifications verification codes into a compatible
mobile application or operating system for verification of a diagnosis
to share within the exposure notifications system.

-   `ens` : describes that the exposure notifications application to be opened.

-   `onboarding` : activate onboarding for exposure notifications. 

-   `v` : verify, currently this is the only available action.

-   `r` : region, the region that this verification code is for.

    -   Regions must be [ISO_3166-2](https://en.wikipedia.org/wiki/ISO_3166-2)
        codes.
    -   For country level, the 2 character code is used.
    -   If sub-regions are being used, this should be 2 character country, '-'
        (dash) followed by the 2 or 3 character subdivision code.
    -   Region codes are case insensitive.

-   `c` : verification code to validate the diagnosis.

### Modes

-  `onboarding` - Activate / onboarding mode. No additional parameters are supported.

-  `v` - Supports region (`r`) and code (`c`) as required parameters.

## Encoding considerations

Use URL encoding if applicable. This URI is intended to be sent over SMS. While
there is no strict limit on length, it is recommended that the greeting text
combined with the URI not exceed 160 characters in total.

## Examples

### Country level, 16 digit verification codes.

For Austraila:

```text
ens://v?r=AU&c=1234abcd5678efgh
```

### Country + Subdivision, 16 digit verification codes.

For the State of Washington, in the United States:

```text
ens://v?r=US-WA&c=abcdefgh12345678
```

And for State of New South Wales in Austraila:

```text
ens://v?r=AU-NSW&c=abcdefgh12345678
```

### Activation, region chosen on device.

The `onboarding` command does not support any other parameters.

```text
ens://onboarding
```

