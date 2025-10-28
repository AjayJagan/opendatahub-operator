// Helper function to get the latest commit SHA for a repository reference
async function getLatestCommitSha(github, org, repo, ref) {
    try {
        console.log(`  Fetching latest commit SHA for ${org}/${repo}:${ref}`)
        const { data } = await github.rest.repos.getCommit({
            owner: org,
            repo: repo,
            ref: ref
        });
        return data.sha;
    } catch (error) {
        console.error(`  ⚠️  Failed to fetch commit SHA for ${org}/${repo}:${ref}: ${error.message}`);
        return null;
    }
}

module.exports = async ({ github, core }) => {
    const { TRACKER_URL } = process.env
    console.log(`The tracker url is: ${TRACKER_URL}`)

    const arr = TRACKER_URL.split("/")
    const owner = arr[3]
    const repo = arr[4]
    const issue_number = arr[6]

    try {
        const result = await github.request('GET /repos/{owner}/{repo}/issues/{issue_number}/comments', {
            owner,
            repo,
            issue_number,
            headers: {
                'X-GitHub-Api-Version': '2022-11-28',
                'Accept': 'application/vnd.github.text+json'
            }
        });

        // Collect all components to process
        const componentsToProcess = [];

        result.data.forEach((issue) => {
            let issueCommentBody = issue.body_text
            if (issueCommentBody.includes("#Release#")) {
                let components = issueCommentBody.split("\n")
                const releaseIdx = components.indexOf("#Release#")
                components = components.splice(releaseIdx + 1, components.length)
                const regex = /\s*[A-Za-z-_0-9/]+\s*\|\s*(https:\/\/github\.com\/.*(tree|releases).*){1}\s*\|?\s*(https:\/\/github\.com\/.*releases.*)?\s*/;

                components.forEach(component => {
                    if (regex.test(component)) {
                        const [componentName, branchOrTagUrl] = component.split("|")
                        const splitArr = branchOrTagUrl.trim().split("/")
                        let idx = null
                        if (splitArr.includes("tag")) {
                            idx = splitArr.indexOf("tag")
                        } else if (splitArr.includes("tree")) {
                            idx = splitArr.indexOf("tree")
                        }
                        const branchName = splitArr.slice(idx + 1).join("/")
                        const repoOrg = splitArr[3]
                        const repoName = splitArr[4]

                        componentsToProcess.push({
                            componentName: componentName.trim(),
                            branchName,
                            repoOrg,
                            repoName
                        });
                    }
                })
            }
        })

        console.log(`Found ${componentsToProcess.length} components in tracker issue`);

        // Process each component and fetch commit SHAs
        for (const comp of componentsToProcess) {
            console.log(`Processing component: ${comp.componentName}`);

            // Fetch the commit SHA for this branch/tag
            const commitSha = await getLatestCommitSha(github, comp.repoOrg, comp.repoName, comp.branchName);

            // Handle special case for notebook-controller
            if (comp.componentName === "workbenches/notebook-controller") {
                core.exportVariable("component_spec_odh-notebook-controller".toLowerCase(), comp.branchName);
                core.exportVariable("component_spec_kf-notebook-controller".toLowerCase(), comp.branchName);
                core.exportVariable("component_org_odh-notebook-controller".toLowerCase(), comp.repoOrg);
                core.exportVariable("component_org_kf-notebook-controller".toLowerCase(), comp.repoOrg);

                if (commitSha) {
                    core.exportVariable("component_sha_odh-notebook-controller".toLowerCase(), commitSha);
                    core.exportVariable("component_sha_kf-notebook-controller".toLowerCase(), commitSha);
                }
            } else {
                const normalizedName = comp.componentName.toLowerCase().replace(/\//g, '-');
                core.exportVariable("component_spec_" + normalizedName, comp.branchName);
                core.exportVariable("component_org_" + normalizedName, comp.repoOrg);

                if (commitSha) {
                    core.exportVariable("component_sha_" + normalizedName, commitSha);
                    console.log(`  ✅ Set SHA for ${comp.componentName}: ${commitSha.substring(0, 8)}`);
                }
            }
        }

        console.log("Read release/tag from tracker issue successfully...");
    } catch (e) {
        core.setFailed(`Action failed with error ${e}`);
    }
}
