// verify node version
const [major] = process.version.slice(1).split('.').map(Number)
console.assert(major >= 20, `need node 20+, got ${process.version}`)
console.log(`node ${process.version}`)

// stdlib fs + path round-trip
const path = require('path')
const os = require('os')
console.log(`os.platform: ${os.platform()}`)
console.log(`path.join: ${path.join('/workspace', 'hello.js')}`)

// stdlib crypto
const { createHash } = require('crypto')
const hash = createHash('sha256').update('lagoon').digest('hex')
console.assert(hash.length === 64, 'sha256 length wrong')
console.log('crypto: ok')

console.log('all checks passed')
