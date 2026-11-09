import assert from 'node:assert/strict'
import {execFileSync} from 'node:child_process'

assert.equal(process.env.GOWORK,'off','dependency qualification requires GOWORK=off')
const metadata=JSON.parse(execFileSync('go',['mod','edit','-json'],{encoding:'utf8'}))
assert.equal(metadata.Module.Path,'github.com/MalenkiySolovey/solovey-host')
assert.deepEqual((metadata.Require??[]).map(item=>({path:item.Path,version:item.Version})),
  [{path:'github.com/robfig/cron/v3',version:'v3.0.1'}],'only the accepted cron dependency is permitted')
assert.equal((metadata.Replace??[]).length,0,'local or substituted source is forbidden')
const output=execFileSync('go',['list','-deps','-test','-f','{{if not .Standard}}{{.ImportPath}}{{end}}','./...'],{encoding:'utf8'})
const packages=output.trim().split(/\r?\n/).filter(Boolean)
for(const name of packages)assert.ok(name.startsWith('github.com/MalenkiySolovey/solovey-host/')||name==='github.com/robfig/cron/v3',`forbidden dependency: ${name}`)
const direct=execFileSync('go',['list','-f','{{.ImportPath}}|{{join .Imports ","}}','./...'],{encoding:'utf8'})
for(const line of direct.trim().split(/\r?\n/)) {
  const [owner,imports]=line.split('|')
  for(const name of (imports??'').split(','))assert.ok(!['database/sql','os/exec','plugin'].includes(name),`execution/storage dependency in ${owner}: ${name}`)
}
console.log(JSON.stringify({status:'PASS',packages,externalModules:1,acceptedDependency:'github.com/robfig/cron/v3@v3.0.1'}))
