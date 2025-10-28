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
        // Component name might have dashes (from env var) but manifest file might have slashes
        // workbenches-kf-notebook-controller -> try workbenches/kf-notebook-controller
        // Only replace first dash with slash (path separator)
        const componentVariants = [
            component,
            component.replace('-', '/')  // Replace only FIRST dash with slash
        ];

        let updated = false;
        let originalContent = content;

        for (const variant of componentVariants) {
            // Escape special regex characters in component name and ref
            const escapedComponent = variant.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
            const escapedRef = newRef.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

            // Pattern to match: ["component"]="org:repo:old-ref:path"
            const pattern = new RegExp(
                `(\\["${escapedComponent}"\\]="[^:]+:[^:]+:)([^:]+)(:+[^"]+")`,
                'g'
            );

            content = content.replace(pattern, `$1${newRef}$3`);

            if (content !== originalContent) {
                updated = true;
                break;
            }
        }

        if (updated) {
            console.log(`  ✅ Updated ${component}`);
        } else {
            console.log(`  ⚠️  Warning: Could not find ${component} in manifest file`);
        }
    }

    // Update orgs
    for (const [component, newOrg] of orgUpdates) {
        // Component name might have dashes (from env var) but manifest file might have slashes
        const componentVariants = [
            component,
            component.replace('-', '/')  // Replace only FIRST dash with slash
        ];

        let updated = false;
        let originalContent = content;

        for (const variant of componentVariants) {
            const escapedComponent = variant.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

            // Pattern to match: ["component"]="org:repo:ref:path"
            const pattern = new RegExp(
                `(\\["${escapedComponent}"\\]=")([^:]+)(:+[^"]+")`,
                'g'
            );

            content = content.replace(pattern, `$1${newOrg}$3`);

            if (content !== originalContent) {
                updated = true;
                break;
            }
        }

        if (updated) {
            console.log(`  ✅ Updated org for ${component}`);
        }
    }

    // Write back
    fs.writeFileSync(manifestFile, content);
    console.log(`\n✅ Updated ${manifestFile}`);
};
