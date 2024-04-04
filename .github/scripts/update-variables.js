module.exports = ({ github, context }) => {
    const { VERSION, TRACKER_URL } = process.env
    try {
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
        console.log("Updated variables successfully")
    } catch (e) {
        console.log("Failed to update the variables:", e)
    }
}