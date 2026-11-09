import assert from 'node:assert/strict'
import {execFileSync} from 'node:child_process'

assert.equal(process.env.GOWORK,'off','dependency qualification requires GOWORK=off')
const metadata=JSON.parse(execFileSync('go',['mod','edit','-json'],{encoding:'utf8'}))
assert.equal(metadata.Module.Path,'github.com/MalenkiySolovey/solovey-host')
assert.equal((metadata.Require??[]).length,0,'host facts must have a standard-library dependency closure')
assert.equal((metadata.Replace??[]).length,0,'local or substituted source is forbidden')
const output=execFileSync('go',['list','-deps','-test','-f','{{if not .Standard}}{{.ImportPath}}{{end}}','./...'],{encoding:'utf8'})
const packages=output.trim().split(/\r?\n/).filter(Boolean)
for(const name of packages)assert.ok(name.startsWith('github.com/MalenkiySolovey/solovey-host/'),`forbidden dependency: ${name}`)
console.log(JSON.stringify({status:'PASS',packages,externalModules:0}))
