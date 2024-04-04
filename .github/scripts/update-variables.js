module.exports = ({ github, context, core }) => {
    const { VERSION, TRACKER_URL } = process.env
    try {
        console.log(`Variables to update are: VERSION: ${VERSION} and TRACKER_URL:${TRACKER_URL}`)
        async function updateVariables() {
            await github.request('PATCH /repos/{owner}/{repo}/actions/variables/{name}', {
                owner: context.repo.owner,
                repo: context.repo.repo,
                name: 'VERSION',
                value: VERSION,
                headers: {
                    'X-GitHub-Api-Version': '2022-11-28'
                }
            })

            await github.request('PATCH /repos/{owner}/{repo}/actions/variables/{name}', {
                owner: context.repo.owner,
                repo: context.repo.repo,
                name: 'TRACKER_URL',
                value: TRACKER_URL,
                headers: {
                    'X-GitHub-Api-Version': '2022-11-28'
                }
            })
        }
        updateVariables()
        console.log("Updated variables successfully...")
    } catch (e) {
        core.setFailed(`Action failed with error ${e}`);
    }
}