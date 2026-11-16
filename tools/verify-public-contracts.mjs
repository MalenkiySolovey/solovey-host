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
const owners=provenance.files.filter(file=>file.originalCommit==='9ad094b07f65ac23ddc10b496e95658c82dab24e')
const expectedOwners=[
 'componenthost/fallbacktargets/canonical.go',
 'componenthost/fallbacktargets/contracts_v2.go',
 'componenthost/fallbacktargets/provider_contracts_test.go',
 'componenthost/fallbacktargets/provider_v2.go',
 'componenthost/fallbacktargets/reservations_v1.go',
 'componenthost/fallbacktargets/target_identity.go',
 'coreruntime/inboundcontrol/patch_types.go',
 'coreruntime/inboundcontrol/ports.go',
 'coreruntime/inboundcontrol/types.go',
]
assert.deepEqual(owners.map(file=>file.destination).sort(),expectedOwners,'owner contract provenance must be complete')
for(const file of owners){
 const source=fs.readFileSync(file.destination,'utf8')
 assert.equal(createHash('sha256').update(source).digest('hex'),file.destinationSHA256,'owner contract provenance drift: '+file.destination)
 assert.match(file.originalBlob,/^[a-f0-9]{40}$/)
 assert.ok(file.history.length>0)
 assert.ok(!/github.com\/MalenkiySolovey\/solovey-ui\/|gorm.io|gin-gonic|database\/sql|os\/exec|go-sqlite3/.test(source),'private storage or executor dependency: '+file.destination)
 assert.ok(!/type (?:Registry|Planner|Workflow|MutationDependencies|MutationHooks|CandidateHydrator)|var Default|func (?:Register|LoadInstalled|RecheckInstalled)|net\.(?:Listen|Dial)|tls\.Certificate/.test(source),'concrete authority in owner contract: '+file.destination)
 assert.ok(!/^\s*Pinned\w*\s*=|CapabilityResolverRevisionV1\s*=|BuildProfile\w*\s*=/m.test(source),'engine or capability implementation pin moved into Host: '+file.destination)
}
const networkWire=provenance.files.filter(file=>file.destination==='network/nftcontract/types.go')
assert.equal(networkWire.length,1,'network wire provenance must be complete')
for(const file of networkWire){
 const source=fs.readFileSync(file.destination,'utf8')
 assert.equal(createHash('sha256').update(source).digest('hex'),file.destinationSHA256,'network wire provenance drift')
 assert.match(file.originalCommit,/^[a-f0-9]{40}$/)
 assert.match(file.originalBlob,/^[a-f0-9]{40}$/)
 assert.ok(file.history.length>0)
 assert.equal(file.additionalSources.length,1,'platform support declaration provenance is required')
 assert.match(file.additionalSources[0].originalBlob,/^[a-f0-9]{40}$/)
 assert.ok(file.additionalSources[0].history.length>0)
 assert.ok(!/^\s*(?:import|func|var)\s/m.test(source),'network wire must contain values only')
 assert.deepEqual([...source.matchAll(/^type (\w+) struct\s*\{/gm)].map(match=>match[1]).sort(),
  ['Correlation','NFTApplyRequest','NFTObserveRequest','NFTResult','NFTRollbackRequest','NFTSupport','NFTValidateRequest'].sort(),'network wire vocabulary is closed')
 assert.ok(!/Planner|Executor|ManagedRoot|type\s+\w+\s+interface\s*\{/.test(source.replace(/^\/\/.*$/gm,'')),'execution authority in network wire')
}
const providers=provenance.files.filter(file=>file.group==='resource-and-probe-contracts')
assert.deepEqual(providers.map(file=>file.destination).sort(),[
 'componenthost/resources/fronting_contracts.go',
 'componenthost/resources/local_proxy_contracts.go',
 'componenthost/resources/inbound_transport_v2.go',
 'componenthost/resources/interception_contracts.go',
 'componenthost/resources/provider_contracts_test.go',
 'componenthost/resources/provider_validation.go',
 'componenthost/resources/transport_boundary_test.go',
 'componenthost/health/protocol_probe_contracts.go',
 'componenthost/health/local_proxy_probe_contracts.go',
 'componenthost/health/interception_probe_contracts.go',
 'componenthost/health/bounds.go',
 'componenthost/health/probe_validation.go',
 'componenthost/health/probe_value_fixtures_test.go',
 'componenthost/health/probe_validation_test.go',
].sort(),'provider/probe source provenance must be complete')
for(const file of providers){
 const source=fs.readFileSync(file.destination,'utf8')
 assert.equal(createHash('sha256').update(source).digest('hex'),file.destinationSHA256,'provider/probe source drift: '+file.destination)
 assert.match(file.originalCommit,/^[a-f0-9]{40}$/)
 assert.match(file.originalBlob,/^[a-f0-9]{40}$/)
 assert.ok(file.history.length>0)
 for(const extra of file.additionalSources){assert.match(extra.originalBlob,/^[a-f0-9]{40}$/);assert.ok(extra.history.length>0)}
 assert.ok(!/github.com\/MalenkiySolovey\/solovey-ui\/|gorm.io|gin-gonic|database\/sql|os\/exec|syscall|golang.org\/x\/sys|"sync(?:\/atomic)?"/.test(source),'private, storage, process or registry dependency: '+file.destination)
 if(!file.destination.endsWith('_test.go')){
  assert.ok(!/type\s+\w*Registry|var\s+Default|New\w*Registry|\bgo\s+func|time\.NewTicker|net\.(?:Listen|Dial)/.test(source),'mutable owner or execution in provider/probe contracts: '+file.destination)
 }
}
const planners=provenance.files.filter(file=>file.group==='planner-contracts')
assert.deepEqual(planners.map(file=>file.destination).sort(),[
 'network/planning/types.go','network/planning/ports.go','network/intent/endpoint.go','network/intent/endpoint_test.go',
].sort(),'coordinator contract provenance must be complete')
for(const file of planners){
 const source=fs.readFileSync(file.destination,'utf8')
 assert.equal(createHash('sha256').update(source).digest('hex'),file.destinationSHA256,'coordinator contract source drift: '+file.destination)
 assert.match(file.originalCommit,/^[a-f0-9]{40}$/);assert.match(file.originalBlob,/^[a-f0-9]{40}$/);assert.ok(file.history.length>0)
 for(const extra of file.additionalSources){assert.match(extra.originalBlob,/^[a-f0-9]{40}$/);assert.ok(extra.history.length>0)}
 assert.ok(!/github.com\/MalenkiySolovey\/solovey-ui\/|gorm.io|gin-gonic|database\/sql|os\/exec|syscall|golang.org\/x\/sys|"sync(?:\/atomic)?"|"reflect"|\bvar\s/.test(source),'concrete coordinator/platform/registry authority in public contracts')
 if(file.destination.startsWith('network/planning/')) {
  assert.ok(!/^func\s+[A-Za-z_]/m.test(source),'coordinator algorithms must remain with their owner')
  assert.ok(!/\bany\b|interface\s*\{\s*\}/.test(source),'untyped coordinator context is forbidden')
 }
 if(file.destination==='network/intent/endpoint.go') assert.deepEqual([...source.matchAll(/^func\s+(\w+)\(/gm)].map(m=>m[1]).sort(),['ValidProtectionEndpoint','validEndpoint'],'only bounded endpoint predicates may move')
}
const views=provenance.files.filter(file=>file.group==='operation-read-views')
assert.deepEqual(views.map(file=>file.destination).sort(),['operationref/views.go','operationref/views_test.go'],'operation read view provenance must be complete')
for(const file of views){
 const source=fs.readFileSync(file.destination,'utf8')
 assert.equal(createHash('sha256').update(source).digest('hex'),file.destinationSHA256,'operation view source drift: '+file.destination)
 assert.match(file.originalCommit,/^[a-f0-9]{40}$/);assert.match(file.originalBlob,/^[a-f0-9]{40}$/);assert.ok(file.history.length>0)
 for(const extra of file.additionalSources){assert.match(extra.originalBlob,/^[a-f0-9]{40}$/);assert.ok(extra.history.length>0)}
 assert.ok(!/github.com\/MalenkiySolovey\/solovey-ui\/|gorm.io|database\/sql|os\/exec|"sync(?:\/atomic)?"|\bvar\s|\bany\b|interface\s*\{\s*\}/.test(source),'operation journal, execution or untyped owner context moved into read contracts')
 if(file.destination==='operationref/views.go') assert.deepEqual([...source.matchAll(/^func\s+(?:\([^)]*\)\s+)?(\w+)\(/gm)].map(m=>m[1]).sort(),['ProjectJob','WithProblem'],'only existing bounded read projections may move')
}
const projections=provenance.files.filter(file=>file.group==='subscription-projections')
assert.equal(projections.length,24,'shared subscription source provenance must be complete')
for(const file of projections){
 const source=fs.readFileSync(file.destination,'utf8')
 assert.equal(createHash('sha256').update(source).digest('hex'),file.destinationSHA256,'shared subscription source drift: '+file.destination)
 assert.match(file.originalCommit,/^[a-f0-9]{40}$/);assert.match(file.originalBlob,/^[a-f0-9]{40}$/);assert.ok(file.history.length>0)
 assert.ok(!/github.com\/MalenkiySolovey\/solovey-ui\/|gorm.io|database\/sql|os\/exec|net\.(?:Listen|Dial)|time\.NewTicker/.test(source),'private storage or execution in shared subscription source: '+file.destination)
}
console.log(JSON.stringify({status:'PASS',contracts:[...files,...neutral,...owners,...networkWire,...providers,...planners,...views,...projections].map(file=>file.destination),license:'GPL-3.0',authority:'retained by owner adapters'}))
