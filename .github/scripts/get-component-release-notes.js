function getModifiedComponentName(name) {
    let modifiedWord = name.split("-").join(" ").replace(/[^a-zA-Z ]/g, "").trim()
    modifiedWord = modifiedWord[0].toUpperCase() + modifiedWord.slice(1).toLowerCase()
    return modifiedWord.replace("Odh", "ODH")
}

function getComponentVersion(url) {
    const splitArr = url.trim().split("/")
    let idx = null
    if (splitArr.includes("tag")) {
        idx = splitArr.indexOf("tag")
    } else if (splitArr.includes("tree")) {
        idx = splitArr.indexOf("tree")
    }
    const releaseName = splitArr.slice(idx + 1).join("/")
    const semverRegex = /\bv?\d+\.\d+\.\d+\b/;
    const match = releaseName.match(semverRegex);
    return match ? match[0] : null;
}

function createMarkdownTable(array){
    if (!array.length) return '';

    const colWidths = array[0].map((_, colIndex) => 
        Math.max(...array.map(row => String(row[colIndex]).length))
    );

    const header = array[0].map((header, index) => 
        header.toString().padEnd(colWidths[index])
    ).join(' | ');

    const separator = colWidths.map(width => '-'.repeat(width)).join(' | ');

    const rows = array.slice(1).map(row => 
        row.map((cell, index) => 
            cell.toString().padEnd(colWidths[index])
        ).join(' | ')
    );

    return [header, separator, ...rows].join('\n');
}

module.exports = async ({ github, core, context }) => {
    const { TRACKER_URL: trackerUrl, VERSION: currentTag } = process.env
    console.log(`The TRACKER_URL is ${trackerUrl}`)
    const arr = trackerUrl.split("/")
    const owner = arr[3]
    const repo = arr[4]
    const issue_number = arr[6]

    try {
        const latestReleaseResult = await github.rest.repos.getLatestRelease({
            owner: context.repo.owner,
            repo: context.repo.repo,
            headers: {
                'X-GitHub-Api-Version': '2022-11-28',
                'Accept': 'application/vnd.github+json',
            }
        })
        const previousTag = latestReleaseResult.data["tag_name"]
        console.log(`The current tag is: ${previousTag}`)

        const releaseNotesResult = await github.rest.repos.generateReleaseNotes({
            owner: context.repo.owner,
            repo: context.repo.repo,
            tag_name: currentTag,
            previous_tag_name: previousTag,
            headers: {
                'X-GitHub-Api-Version': '2022-11-28',
                'Accept': 'application/vnd.github+json'
            }
        })
        const releaseNotesString = releaseNotesResult.data["body"]

        const commentsResult = await github.rest.issues.listComments({
            owner,
            repo,
            issue_number,
            headers: {
                'X-GitHub-Api-Version': '2022-11-28',
                'Accept': 'application/vnd.github.text+json'
            }
        })
        let outputStr = "## Component Release Notes\n"
        const releaseNotesArray = [["Technology", "Version"]]
        commentsResult.data.forEach((issue) => {
            let issueCommentBody = issue.body_text
            if (issueCommentBody.includes("#Release#")) {
                let components = issueCommentBody.split("\n")
                const releaseIdx = components.indexOf("#Release#")
                components = components.splice(releaseIdx + 1, components.length)
                const regex = /\s*[A-Za-z-_0-9]+\s*\|\s*(https:\/\/github\.com\/.*(tree|releases).*){1}\s*\|?\s*(https:\/\/github\.com\/.*releases.*)?\s*/;
                components.forEach(component => {
                    if (regex.test(component)) {
                        let [componentName, branchUrl, tagUrl] = component.split("|")
                        componentName = getModifiedComponentName(componentName.trim())
                        const releaseNotesUrl = (tagUrl || branchUrl).trim();
                        if (!outputStr.includes(componentName)) {
                            outputStr += `- **${componentName}**: ${releaseNotesUrl}\n`
                            releaseNotesArray.push([componentName, getComponentVersion(releaseNotesUrl)])
                        }

                    }
                })
            }
        })

        outputStr += "\n" + releaseNotesString
        console.log("Created component release notes successfully...")
        core.setOutput('release-notes-body', outputStr);
        core.setOutput('release-notes-markdown', `\n ### Open Data Hub version: ${currentTag}\n${createMarkdownTable(releaseNotesArray)}\n`)
    } catch (error) {
        core.setFailed(`Action failed with error ${error}`);
    }
}