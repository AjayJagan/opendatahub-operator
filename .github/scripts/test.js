module.exports = async ({ github, core }) => {
    try {
        console.log("here2")
        const latestReleaseResult = await github.rest.repos.getLatestRelease({
            owner: "AjayJagan",
            repo: "opendatahub-operator",
            headers: {
                'X-GitHub-Api-Version': '2022-11-28',
                'Accept': 'application/vnd.github+json',
            }
        })
        console.log("here3")
        console.log(latestReleaseResult.data["tag_name"])
        // const latestTag = latestReleaseResult.data["tag_name"]
        // console.log(`The current tag is: ${latestTag}`)
    } catch (error) {

        core.setFailed(`Action failed with error ${error}`);
    }
}