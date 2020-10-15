#!/usr/bin/env bash

set -o errexit
set -o nounset

function eexit() {
    echo "ERROR: ${@}" 1>&2
    exit 1
}

[[ -x $(command -v gcloud) ]] || eexit "gcloud is not installed"

[[ $(gcloud components list --filter='id=alpha state.name=Installed' --format=json) != "[]" ]] || eexit "gcloud alpha component not installed. Install using: gcloud components install alpha"

cat <<EOF

WARNING: this is a temporary workaround to create alert policies using MQL.

See https://github.com/hashicorp/terraform-provider-google/issues/7464 for
upstream feature request.
---------------------------------------------------------------------------

EOF

# Sanity check: ensure required environment variables are set.
[[ ${CLOUDSDK_CORE_PROJECT:?} ]]
[[ ${POLICY:?} ]]
[[ ${DISPLAY_NAME:?} ]]

# Sanity check: ensure DISPLAY_NAME matches the POLICY
DISPLAY_NAME_IN_POLICY=$(echo "${POLICY}"|sed -n 's/^displayName: \(.*\)/\1/p')
[[ ${DISPLAY_NAME} == ${DISPLAY_NAME_IN_POLICY} ]] || eexit "Policy contains displayName=${DISPLAY_NAME_IN_POLICY}, expect ${DISPLAY_NAME}"

EXISTING_POLICIES=($(gcloud alpha monitoring policies list --filter="display_name='${DISPLAY_NAME}'" --uri| sed -n 's|https://monitoring.googleapis.com/v3/\(.*\)|\1|p'))

case ${#EXISTING_POLICIES[@]} in
    0)
        echo "Creating policy..."
        gcloud alpha monitoring policies create --policy="${POLICY}"
        ;;
    1)
        TO_UPDATE=${EXISTING_POLICIES[0]}
        echo "Updating policy [${TO_UPDATE}]..."
        gcloud alpha monitoring policies update ${TO_UPDATE} --policy="${POLICY}"
        ;;
    *)
        cat <<HERE
ERROR: multiple policies matching display_name='${DISPLAY_NAME}'.
Please make sure there's only one alert policy with display_name='${DISPLAY_NAME}'

${EXISTING_POLICIES[@]/#/https:\/\/monitoring.googleapis.com\/v3\/}
HERE
        exit 1
        ;;
esac
