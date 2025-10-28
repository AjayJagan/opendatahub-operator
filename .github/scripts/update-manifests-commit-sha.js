const fs = require('fs');

// Parse the get_all_manifests.sh file to extract component definitions
function parseManifestFile(filePath) {
  const content = fs.readFileSync(filePath, 'utf8');
  const componentsToUpdate = new Map();

  // Regex to match component manifest definitions (line by line, no global flag)
  // Pattern: ["component"]="org:repo:ref@sha:path"
  const manifestRegex = /^\s*\["([^"]+)"\]="([^:]+):([^:]+):([^:]+):([^"]+)"$/;

  // Process each line individually to prevent multiline matches
  const lines = content.split('\n');
  for (const line of lines) {
    const match = line.match(manifestRegex);
    if (!match) {
      continue;
    }

    const [fullMatch, componentName, org, repo, ref, sourcePath] = match;

    if (!ref.includes('@')) {
      continue;
    }

    const refParts = ref.split('@');
    if (refParts.length !== 2) {
      console.log(`Skipping ${componentName}: invalid ref format "${ref}" (expected "branch@sha")`);
      continue;
    }

    const [branchRef, commitSha] = refParts;
    if (!branchRef || !commitSha) {
      console.log(`Skipping ${componentName}: empty branch or SHA in ref "${ref}"`);
      continue;
    }

    componentsToUpdate.set(componentName, {
      org,
      repo,
      ref: branchRef,
      commitSha,
      sourcePath,
      originalLine: fullMatch.trim()
    });
  }

  console.log(`Parsed ${componentsToUpdate.size} components from ${filePath}`);
  return componentsToUpdate;
}

// Get the latest commit SHA for a repository reference
async function getLatestCommitSha(octokit, org, repo, ref) {
  console.log(`Fetching latest commit for ${org}/${repo}:${ref}`);
  const { data } = await octokit.rest.repos.getCommit({
    owner: org,
    repo: repo,
    ref: ref
  });

  return data.sha;
}

// Update the manifest file with new SHAs
function updateManifestFile(filePath, componentsToUpdate) {
  if (componentsToUpdate.size === 0) {
    return false;
  }

  let content = fs.readFileSync(filePath, 'utf8');
  let hasChanges = false;

  for (const [componentName, updateInfo] of componentsToUpdate) {
    const oldLine = updateInfo.originalLine;
    const newLine = `["${componentName}"]="${updateInfo.org}:${updateInfo.repo}:${updateInfo.ref}@${updateInfo.newCommitSha}:${updateInfo.sourcePath}"`;

    if (content.includes(oldLine)) {
      content = content.replace(oldLine, newLine);
      hasChanges = true;
      console.log(`Updated ${componentName}: ${updateInfo.commitSha} → ${updateInfo.newCommitSha}`);
    }
  }

  if (hasChanges) {
    fs.writeFileSync(filePath, content);
  }

  return hasChanges;
}

module.exports = async function ({ github, core }) {
  console.log('Starting manifest SHA update process...');

  const manifestFile = 'get_all_manifests.sh';
  const componentsToUpdate = parseManifestFile(manifestFile);

  for (const [componentName, manifest] of componentsToUpdate) {
    console.log(`Checking ${componentName} (${manifest.org}/${manifest.repo}:${manifest.ref})...`);

    const latestSha = await getLatestCommitSha(github, manifest.org, manifest.repo, manifest.ref);

    if (latestSha !== manifest.commitSha && manifest.commitSha) {
      console.log(`Update needed for ${componentName}: ${manifest.commitSha} → ${latestSha}`);

      componentsToUpdate.set(componentName, {
        ...manifest,
        newCommitSha: latestSha
      });
    } else {
      console.log(`No update needed for ${componentName}`);
      componentsToUpdate.delete(componentName);
    }
  }

  // Set outputs
  const hasUpdates = componentsToUpdate.size > 0;
  core.setOutput('updates-needed', hasUpdates);

  if (!hasUpdates) {
    console.log('All manifest references are up to date');
    return;
  }

  // Update manifest file
  console.log('Updating manifest file...');
  updateManifestFile(manifestFile, componentsToUpdate);

  console.log(`Successfully processed ${componentsToUpdate.size} manifest updates`);
}
