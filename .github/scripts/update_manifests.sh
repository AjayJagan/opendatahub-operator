#!/bin/bash

update_tags(){
MANIFEST_STR=$(cat get_all_manifests.sh | grep $1 | sed 's/ //g')
    readarray -d ":" -t STR_ARR <<< "$MANIFEST_STR"
    RES=""
    for i in "${!STR_ARR[@]}"; do
        if [ $i == 2 ]; then
            RES+=$2":"
        else
            RES+=${STR_ARR[$i]}":"
        fi
    done
    sed -i -r "s|.*$1.*|    ${RES::-2}|" get_all_manifests.sh
}
declare -A COMPONENT_VERSION_MAP=(
    ["\"codeflare\""]=${{ env.CODEFLARE_BRANCH }}
    ["\"ray\""]=${{ env.KUBERAY_BRANCH }}
    ["\"kueue\""]=${{ env.KUEUE_BRANCH }}
    ["\"data-science-pipelines-operator\""]=${{ env.DSPO_BRANCH }}
    ["\"odh-dashboard\""]=${{ env.DASHBOARD_BRANCH }}
    ["\"kf-notebook-controller\""]=${{ env.KF_NB_CONTROLLER_BRANCH }}
    ["\"odh-notebook-controller\""]=${{ env.NB_CONTROLLER_BRANCH }}
    ["\"notebooks\""]=${{ env.NB_BRANCH }}
    ["\"trustyai\""]=${{ env.TRUSTYAI_BRANCH }}
    ["\"model-mesh\""]=${{ env.MODEL_MESH_BRANCH }}
    ["\"odh-model-controller\""]=${{ env.ODH_MODEL_CONTROLLER_BRANCH }}
    ["\"kserve\""]=${{ env.KSERVE_BRANCH }}
    ["\"modelregistry\""]=${{ env.MODEL_REGISTRY_BRANCH }}
)
for key in ${!COMPONENT_VERSION_MAP[@]}; do
    update_tags ${key} ${COMPONENT_VERSION_MAP[${key}]}
done

cat get_all_manifests.sh