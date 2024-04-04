module.exports = ({ github, context, core }) => {
    try {
        async function getAndSetVariables() {
            const { data: versionData } = await github.request('GET /repos/{owner}/{repo}/actions/variables/{name}', {
                owner: context.repo.owner,
                repo: context.repo.repo,
                name: 'VERSION',
                headers: {
                    'X-GitHub-Api-Version': '2022-11-28'
                }
            })
            const { data: trackerUrlData } = await github.request('GET /repos/{owner}/{repo}/actions/variables/{name}', {
                owner: context.repo.owner,
                repo: context.repo.repo,
                name: 'TRACKER_URL',
                headers: {
                    'X-GitHub-Api-Version': '2022-11-28'
                }
            })
            console.log(`The VERSION is ${versionData.value} and the TRACKER_URL is ${trackerUrlData.value}`)
            core.exportVariable('VERSION', versionData.value);
            core.exportVariable('TRACKER_URL', trackerUrlData.value);
        }
        getAndSetVariables()
    } catch (e) {
        core.setFailed(`Action failed with error ${e}`);
    }
}