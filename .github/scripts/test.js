function arrayToMarkdownTable(array) {
    if (array.length === 0) return '';

    // Create header row and separator
    const header = array[0];
    const separator = header.map(() => '---').join('|');
    
    // Create the Markdown table
    const rows = array.map(row => row.join('|')).join('\n');

    // Combine header, separator, and rows
    return [header.join('|'), separator, rows].join('\n');
}

const data = [
    ["Technology", "Version"],
    ["Opendatahub", "v2.18.0"],
    ["dashboard", "v2.11.0"],
    ["Model reg", "2.11.11"]
]

module.exports = ({ github, core }) => {
    const markdownTable = arrayToMarkdownTable(data);
    core.setOutput("MD", `\n ### Open Data Hub version\n${markdownTable}\n`);
}