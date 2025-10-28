const fs = require('fs');

/**
 * Update component manifest references in get_all_manifests.sh
 * Reads environment variables exported by get-release-branches.js
 */
module.exports = ({ core }) => {
    const manifestFile = 'get_all_manifests.sh';

    console.log('Updating component branches/tags...');

    // Read environment variables
    const specPrefix = 'component_spec_';
    const orgPrefix = 'component_org_';
    const shaPrefix = 'component_sha_';

    const updates = new Map();
    const orgUpdates = new Map();

    // Collect all spec updates
    for (const [key, value] of Object.entries(process.env)) {
        if (key.startsWith(specPrefix)) {
            const component = key.substring(specPrefix.length).replace(/_/g, '-');
            const shaKey = `component_sha_${key.substring(specPrefix.length)}`;
            const shaValue = process.env[shaKey] || '';

            if (shaValue) {
                const newRef = `${value}@${shaValue}`;
                console.log(`  Updating ${component} to: ${value}@${shaValue.substring(0, 8)}`);
                updates.set(component, newRef);
            } else {
                console.log(`  Updating ${component} to: ${value} (no SHA available)`);
                updates.set(component, value);
            }
        } else if (key.startsWith(orgPrefix)) {
            const component = key.substring(orgPrefix.length).replace(/_/g, '-');
            orgUpdates.set(component, value);
        }
    }

    if (updates.size === 0 && orgUpdates.size === 0) {
        console.log('No updates to apply');
        return;
    }

    // Read the manifest file
    let content = fs.readFileSync(manifestFile, 'utf8');

    // Update refs
    for (const [component, newRef] of updates) {
        // Escape special regex characters in component name and ref
        const escapedComponent = component.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        const escapedRef = newRef.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

        // Pattern to match: ["component"]="org:repo:old-ref:path"
        // Also handles paths with component name like: ["workbenches/notebooks"]="..."
        let pattern = new RegExp(
            `(\\["${escapedComponent}"\\]="[^:]+:[^:]+:)([^:]+)(:+[^"]+")`,
            'g'
        );

        let originalContent = content;
        content = content.replace(pattern, `$1${newRef}$3`);

        if (content === originalContent) {
            // Try with path prefix (e.g., workbenches/notebooks)
            pattern = new RegExp(
                `(\\["[^"]*/${escapedComponent}"\\]="[^:]+:[^:]+:)([^:]+)(:+[^"]+")`,
                'g'
            );
            content = content.replace(pattern, `$1${newRef}$3`);
        }

        if (content !== originalContent) {
            console.log(`  ✅ Updated ${component}`);
        } else {
            console.log(`  ⚠️  Warning: Could not find ${component} in manifest file`);
        }
    }

    // Update orgs
    for (const [component, newOrg] of orgUpdates) {
        const escapedComponent = component.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

        // Pattern to match: ["component"]="org:repo:ref:path"
        let pattern = new RegExp(
            `(\\["${escapedComponent}"\\]=")([^:]+)(:+[^"]+")`,
            'g'
        );

        let originalContent = content;
        content = content.replace(pattern, `$1${newOrg}$3`);

        if (content === originalContent) {
            pattern = new RegExp(
                `(\\["[^"]*/${escapedComponent}"\\]=")([^:]+)(:+[^"]+")`,
                'g'
            );
            content = content.replace(pattern, `$1${newOrg}$3`);
        }

        if (content !== originalContent) {
            console.log(`  ✅ Updated org for ${component}`);
        }
    }

    // Write back
    fs.writeFileSync(manifestFile, content);
    console.log(`\n✅ Updated ${manifestFile}`);
};
