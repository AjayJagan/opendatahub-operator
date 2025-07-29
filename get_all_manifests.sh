#!/usr/bin/env bash
set -e

GITHUB_URL="https://github.com"

# COMPONENT_MANIFESTS is a list of components repositories info to fetch the manifests
# in the format of "repo-org:repo-name:ref-name:source-folder" and key is the target folder under manifests/
declare -A COMPONENT_MANIFESTS=(
    ["dashboard"]="AjayJagan:odh-dashboard:multi-arch-konflux:manifests"
    ["workbenches/kf-notebook-controller"]="AjayJagan:kubeflow:multi-arch-konflux:components/notebook-controller/config"
    ["workbenches/odh-notebook-controller"]="AjayJagan:kubeflow:multi-arch-konflux:components/odh-notebook-controller/config"
    ["workbenches/notebooks"]="grdryn:notebooks:multi-arch-konflux:manifests"
    ["modelmeshserving"]="AjayJagan:modelmesh-serving:multi-arch-konflux:config"
    ["kserve"]="AjayJagan:kserve:rmulti-arch-konflux:config"
    ["kueue"]="AjayJagan:kueue:multi-arch-konflux:config"
    ["codeflare"]="AjayJagan:codeflare-operator:multi-arch-konflux:config"
    ["ray"]="AjayJagan:kuberay:multi-arch-konflux:ray-operator/config"
    ["trustyai"]="trustyai-explainability:trustyai-service-operator:multi-arch-konflux:config"
    ["modelregistry"]="AjayJagan:model-registry-operator:multi-arch-konflux:config"
    ["trainingoperator"]="AjayJagan:training-operator:multi-arch-konflux:manifests"
    ["datasciencepipelines"]="AjayJagan:data-science-pipelines-operator:multi-arch-konflux:config"
    ["modelcontroller"]="AjayJagan:odh-model-controller:multi-arch-konflux:config"
    ["feastoperator"]="AjayJagan:feast:multi-arch-konflux:infra/feast-operator/config"
    ["llamastackoperator"]="AjayJagan:llama-stack-k8s-operator:multi-arch-konflux:config"
)

# Allow overwriting repo using flags component=repo
pattern="^[a-zA-Z0-9_.-]+:[a-zA-Z0-9_.-]+:[a-zA-Z0-9_.-]+:[a-zA-Z0-9_./-]+$"
if [ "$#" -ge 1 ]; then
    for arg in "$@"; do
        if [[ $arg == --* ]]; then
            arg="${arg:2}"  # Remove the '--' prefix
            IFS="=" read -r key value <<< "$arg"
            if [[ -n "${COMPONENT_MANIFESTS[$key]}" ]]; then
                if [[ ! $value =~ $pattern ]]; then
                    echo "ERROR: The value '$value' does not match the expected format 'repo-org:repo-name:ref-name:source-folder'."
                    continue
                fi
                COMPONENT_MANIFESTS["$key"]=$value
            else
                echo "ERROR: '$key' does not exist in COMPONENT_MANIFESTS, it will be skipped."
                echo "Available components are: [${!COMPONENT_MANIFESTS[@]}]"
                exit 1
            fi
        else
            echo "Warning: Argument '$arg' does not follow the '--key=value' format."
        fi
    done
fi

TMP_DIR=$(mktemp -d -t "odh-manifests.XXXXXXXXXX")
trap '{ rm -rf -- "$TMP_DIR"; }' EXIT

function git_fetch_ref()
{

    local repo=$1
    local ref=$2
    local dir=$3
    local git_fetch="git fetch -q --depth 1 $repo"

    mkdir -p $dir
    pushd $dir &>/dev/null
    git init -q
    # try tag first, avoid printing fatal: couldn't find remote ref
    if ! $git_fetch refs/tags/$ref 2>/dev/null ; then
        $git_fetch refs/heads/$ref
    fi
    git reset -q --hard FETCH_HEAD
    popd &>/dev/null
}

download_manifest() {
    local key=$1
    local repo_info=$2
    echo -e "\033[32mCloning repo \033[33m${key}\033[32m:\033[0m ${repo_info}"
    IFS=':' read -r -a repo_info <<< "${repo_info}"

    repo_org="${repo_info[0]}"
    repo_name="${repo_info[1]}"
    repo_ref="${repo_info[2]}"
    source_path="${repo_info[3]}"
    target_path="${key}"

    repo_url="${GITHUB_URL}/${repo_org}/${repo_name}"
    repo_dir=${TMP_DIR}/${key}

    git_fetch_ref ${repo_url} ${repo_ref} ${repo_dir}

    mkdir -p ./opt/manifests/${target_path}
    cp -rf ${repo_dir}/${source_path}/* ./opt/manifests/${target_path}
}

# Track background job PIDs +declare -a pids=()
# Use parallel processing
for key in "${!COMPONENT_MANIFESTS[@]}"; do
    download_manifest "$key" "${COMPONENT_MANIFESTS[$key]}" &
    pids+=($!)
done
# Wait and check exit codes
failed=0
for pid in "${pids[@]}"; do
    if ! wait "$pid"; then
        failed=1
    fi
done
if [ $failed -eq 1 ]; then
    echo "One or more downloads failed"
    exit 1
fi
