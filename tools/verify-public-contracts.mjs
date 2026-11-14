import assert from 'node:assert/strict'
import fs from 'node:fs'
import {createHash} from 'node:crypto'
const provenance=JSON.parse(fs.readFileSync('SOURCE_PROVENANCE.json','utf8'))
const files=provenance.files.filter(file=>file.originalCommit==='7adbb999d7fe70ba169bccae06afafaf4fc2e413')
assert.equal(files.length,6)
for(const file of files){
 assert.equal(createHash('sha256').update(fs.readFileSync(file.destination)).digest('hex'),file.destinationSHA256,'source provenance drift: '+file.destination)
 assert.match(file.originalBlob,/^[a-f0-9]{40}$/)
 assert.ok(file.history.length>0)
 const text=fs.readFileSync(file.destination,'utf8')
 assert.ok(!/github.com\/MalenkiySolovey\/solovey-ui\/|gorm.io|gin-gonic|database\/sql|os\/exec/.test(text),'private source or execution dependency: '+file.destination)
 assert.ok(!/type Registry|var Default|func Register|net.Listen|tls.Certificate/.test(text),'concrete authority in neutral contract: '+file.destination)
}
assert.equal(createHash('sha256').update(fs.readFileSync('LICENSE')).digest('hex'),provenance.licenseSHA256)
const neutral=provenance.files.filter(file=>file.originalCommit==='ac8f629f4668ee18bdcdae29c8ec6eb0b54c07b5')
assert.equal(neutral.length,16,'neutral contract provenance must be complete')
for(const file of neutral){
 const source=fs.readFileSync(file.destination,'utf8')
 assert.equal(createHash('sha256').update(source).digest('hex'),file.destinationSHA256,'neutral source provenance drift: '+file.destination)
 assert.match(file.originalBlob,/^[a-f0-9]{40}$/)
 assert.ok(file.history.length>0)
 assert.ok(!/github.com\/MalenkiySolovey\/solovey-ui\/|gorm.io|gin-gonic|database\/sql|os\/exec/.test(source),'private or privileged dependency: '+file.destination)
 assert.ok(!/type (?:Registry|Planner|Workflow)|var Default|func (?:Register|LoadInstalled|RecheckInstalled)|net\.Listen|tls\.Certificate/.test(source),'concrete authority in neutral source: '+file.destination)
 if(file.destination==='network/intent/protection.go')assert.ok(!/^\s*(?:func|var)\s/m.test(source),'network intent must contain values only')
}
console.log(JSON.stringify({status:'PASS',contracts:files.map(file=>file.destination),license:'GPL-3.0',authority:'retained by owner adapters'}))
