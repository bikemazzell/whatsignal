const fs = require('node:fs');

const utilsPath = '/app/node_modules/whatsapp-web.js/src/util/Injected/Utils.js';
const marker = "        // Bot's won't reply if canonicalUrl is set (linking)";
const patch = '        delete message.__x_id;\n';

const source = fs.readFileSync(utilsPath, 'utf8');

if (source.includes(patch.trim())) {
    process.exit(0);
}

const markerCount = source.split(marker).length - 1;
if (markerCount !== 1) {
    throw new Error(`Expected one WAHA Utils.js patch marker, found ${markerCount}`);
}

fs.writeFileSync(utilsPath, source.replace(marker, `${patch}${marker}`));
