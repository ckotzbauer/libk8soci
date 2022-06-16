#!/bin/bash
set -e

# Creates a dockerconfigjson formatted auth-file with the given credentials. Output is redirected to stdout.
# Usage: create_auth_file "<USERNAME>" "<PASSWORD>" "<SERVER>" "<EMAIL>"
create_auth_file() {
    kubectl create secret docker-registry pull-secret \
        --docker-username="${1}" \
        --docker-password="${2}" \
        --docker-server="${3}" \
        --docker-email="${4}" \
        -o json --dry-run=client | jq -r '.data.".dockerconfigjson"'
}

# Creates a legacy dockercfg formatted auth-file with the given credentials. Output is redirected to stdout.
# Usage: create_legacy_auth_file "<USERNAME>" "<PASSWORD>" "<SERVER>"
create_legacy_auth_file() {
    cat << EOF > .dockercfg
    {
        "${3}": { "username": "${1}", "${2}" }
    }
EOF

    kubectl create secret generic pull-secret \
        --from-file=.dockercfg \
        --type=kubernetes.io/dockercfg \
        -o json --dry-run=client | jq -r '.data.".dockercfg"'

    rm .dockercfg
}


