# Resource Identifier (RI) Scheme name: ens

# Scheme syntax

```ens://v?r=[region]&c=[verificaton code]```

# Scheme semantics

Send covid-19 exposure notifications verifications code into a compatible mobile application or operating system for verificafication of a diagnosis to share within the exposure notifications system.

* `ens` : describes the exposure notifications sytem application to be opened.
* `v` : verify, currently this is the only available action.
* `r` : region, the region that this verification code is for.
* `c` : verification code to validate the diagnosis.

# Encoding considerations

Use URL encoding if applicable.