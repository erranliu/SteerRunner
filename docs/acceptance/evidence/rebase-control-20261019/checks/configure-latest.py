import json, pathlib
r = pathlib.Path(__file__).parent
old = json.loads((r/'live-config-r2.json').read_text(encoding='utf-8-sig'))
c = json.loads(pathlib.Path('tests/test-config.example.json').read_text())
local = c['configurations']['local-codex']['execution']['settings']
local.update(executable=old['codex_executable'], model=old['model'], reasoning_effort=old['reasoning_effort'])
c['configurations']['orca-remote-codex'] = {
    'workflow': {'id':'development','version':2},
    'execution': {
        'adapter':'orca-terminal',
        'target':{'kind':'orca_environment','environment':old['orca_environment']},
        'settings': {
            'access_mode':'sandbox',
            'transport': {'executable':old['orca_executable']},
            'agent_launch': {
                'executable':old['remote_agent'], 'model':old['remote_model'],
                'reasoning_effort':old['remote_reasoning_effort'], 'args':old['remote_args'],
                'runtime_executable':old['remote_runtime_executable'],
                'runtime_sha256':old['remote_runtime_sha256']
            }
        }
    }
}
c['fixture']['repository']['local_path'] = old['orca_local_repository_path']
(r/'test-config-r3.json').write_text(json.dumps(c,indent=2)+'\n',encoding='utf-8')
(r/'interrupted-before-latest-main.json').write_text(json.dumps({
    'source':'5861ded54e0233633a521cc620441d46291e8cf9',
    'reason':'Main advanced to 2aa17cc while Controlled prerequisites were running; deliberately interrupted owned batch before any native cases or probes. Original running summary remains unmodified; not a pass. Public stop/status of owned service captured separately.',
    'final_source_qualification':False
},indent=2)+'\n',encoding='utf-8')
