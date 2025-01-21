#!/bin/bash

set -euo pipefail

update_tags(){
    component=$(basename "$1")
    echo "$component"
    sed -i -r "/\"(.*\/)*$component\"/s|([^:]*):([^:]*):[^:]*:(.*)|\1:\2:$2:\3|" get_all_manifests.sh
}

prefix=component_spec_

echo
env | while IFS="=" read varname value; do
    [[ $varname =~ "${prefix}" ]] || continue
    component=${varname#${prefix}}
    component=${component//_/-}
    update_tags $component $value
done