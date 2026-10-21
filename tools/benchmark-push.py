#!/usr/bin/env python3
"""Measure successful same-repository, same-branch pushes on an isolated server."""
import argparse
import base64
from concurrent.futures import ThreadPoolExecutor
import hashlib
import http.client
import json
import os
from pathlib import Path
import secrets
import shutil
import struct
import subprocess
import sys
import tempfile
import threading
import time
import urllib.error
import urllib.request
import uuid
import zlib


def pkt(data):
    return ("%04x" % (len(data) + 4)).encode() + data


def object_id(kind, data):
    return hashlib.sha1(kind.encode() + b" " + str(len(data)).encode() + b"\0" + data).hexdigest()


def fixture_commit(parent, number):
    blob = ("generated text increment %d\n" % number).encode().ljust(1024, b"x")
    blob_id = object_id("blob", blob)
    tree = b"100644 sample.txt\0" + bytes.fromhex(blob_id)
    tree_id = object_id("tree", tree)
    commit = ("tree " + tree_id + "\n" + ("parent " + parent + "\n" if parent else "") +
              "author Fixture <fixture@example.invalid> 981173106 +0000\n" +
              "committer Fixture <fixture@example.invalid> 981173106 +0000\n\n" +
              "increment %d\n" % number).encode()
    return object_id("commit", commit), [(1, commit), (2, tree), (3, blob)]


def pack(objects):
    output = bytearray(b"PACK" + struct.pack(">II", 2, len(objects)))
    for kind, data in objects:
        size = len(data)
        header = bytearray([(kind << 4) | (size & 15)])
        size >>= 4
        while size:
            header[-1] |= 128
            header.append(size & 127)
            size >>= 7
        output.extend(header + zlib.compress(data))
    output.extend(hashlib.sha1(output).digest())
    return bytes(output)


def summary(latencies, elapsed, attempts, uploaded):
    ordered = sorted(latencies)
    def percentile(q):
        return round(ordered[min(len(ordered) - 1, int((len(ordered) - 1) * q))] * 1000, 3)
    return dict(successful_pushes=len(latencies), attempts=attempts, rejected_attempts=attempts-len(latencies),
                elapsed_seconds=round(elapsed, 6), successful_pushes_per_second=round(len(latencies)/elapsed, 3),
                p50_ms=percentile(.50), p95_ms=percentile(.95), p99_ms=percentile(.99), uploaded_bytes=uploaded)


def protocol_load(host, port, path, authorization, current, pushes, workers, worker_counts=None, scheduling="continuous"):
    def post(old, new, objects, connection=None):
        payload = pkt((old+" "+new+" refs/heads/main\0report-status\n").encode()) + b"0000" + pack(objects)
        conn = connection or http.client.HTTPConnection(host, port, timeout=30)
        then = time.monotonic()
        conn.request('POST', path+'/git-receive-pack', payload,
                     {'Content-Type':'application/x-git-receive-pack-request','Authorization':authorization})
        response = conn.getresponse()
        data = response.read()
        if connection is None:
            conn.close()
        accepted = response.status==200 and b'unpack ok\n' in data and b'ok refs/heads/main\n' in data and b'ng refs/heads/main' not in data
        reason = next((item for item in ('failed to update ref','non-fast-forward','missing necessary objects',
                       'pre-receive hook declined','hook declined','unpacker error') if item.encode() in data), 'other-protocol-rejection')
        return accepted, time.monotonic()-then, len(payload), response.status, reason

    latencies=[]; uploaded=0; started=time.monotonic()
    for index in range(1, pushes+1):
        new, objects=fixture_commit(current, index)
        accepted, elapsed, size, status, reason=post(current, new, objects)
        if not accepted:
            raise RuntimeError('Sequential push failed: '+reason)
        current=new; latencies.append(elapsed); uploaded+=size
    sequential=dict(case='sequential-smart-http', **summary(latencies,time.monotonic()-started,len(latencies),uploaded))
    results=[sequential]
    for count in worker_counts or [workers]:
        current, concurrent=concurrent_protocol_load(post,host,port,current,pushes,count,scheduling)
        results.append(concurrent)
    return dict(current=current,results=results)


def concurrent_protocol_load(post, host, port, current, pushes, workers, scheduling):
    latencies=[]; attempts=0; uploaded=0; mutex=threading.Lock(); chain=[]; parent=current; rejections={}
    for index in range(pushes):
        new, objects=fixture_commit(parent,pushes+index+1)
        chain.append((parent,new,objects)); parent=new
    started=time.monotonic(); deadline=started+180
    def transaction(index):
        nonlocal attempts, uploaded
        old,new,_=chain[index]
        window_start=max(0,index-workers+1) if scheduling=='continuous' else index-index%workers
        objects=[item for entry in chain[window_start:index+1] for item in entry[2]]
        conn=http.client.HTTPConnection(host,port,timeout=30)
        first=time.monotonic()
        try:
            while time.monotonic()<deadline:
                accepted,elapsed,size,status,reason=post(old,new,objects,conn)
                with mutex:
                    attempts+=1; uploaded+=size
                    if accepted:
                        latencies.append(time.monotonic()-first)
                    else:
                        rejections[reason]=rejections.get(reason,0)+1
                if accepted:
                    return
                if status!=200:
                    raise RuntimeError('Unexpected authenticated protocol status')
                time.sleep(.001)
            raise RuntimeError('Same-branch CAS retries exceeded deadline')
        finally:
            conn.close()
    with ThreadPoolExecutor(workers) as pool:
        if scheduling=='continuous':
            list(pool.map(transaction,range(len(chain))))
        else:
            for first in range(0,len(chain),workers):
                list(pool.map(transaction,range(first,min(first+workers,len(chain)))))
    concurrent=dict(case='concurrent-smart-http-linear-chain',workers=workers,
                    maximum_commits_in_one_transport_pack=workers,rejection_reasons=rejections,load_scheduling=scheduling,
                    **summary(latencies,time.monotonic()-started,attempts,uploaded))
    return chain[-1][1], concurrent


def main():
    parser = argparse.ArgumentParser(__doc__)
    parser.add_argument("--server", type=Path, required=True)
    parser.add_argument("--git", type=Path, required=True)
    parser.add_argument("--image", default="golang:1.27-bookworm")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--pushes", type=int, default=128)
    parser.add_argument("--workers", type=int, default=8)
    parser.add_argument("--worker-counts", help="Comma-separated concurrency levels for one acceptance batch")
    parser.add_argument("--concurrent-scheduling", choices=("continuous", "windowed"), default="continuous")
    parser.add_argument("--cli-pushes", type=int, default=12)
    parser.add_argument("--deep-compatibility", action="store_true", help="Also run the installed Git/LFS client, PR merge and permission suite")
    parser.add_argument("--authentication", choices=("personal-access-token", "password"), default="personal-access-token")
    parser.add_argument("--load-location", choices=("windows-host", "container-loopback"), default="windows-host")
    parser.add_argument("--sqlite-journal-mode", choices=("default", "WAL", "DELETE"), default="default")
    parser.add_argument("--sqlite-synchronous", choices=("default", "FULL", "NORMAL", "EXTRA"), default="default")
    parser.add_argument("--diagnostics-dir", type=Path, help="Save private Git Trace2 timings before owned-container cleanup")
    parser.add_argument("--hook-relay", type=Path, help="Matching packaged gitsss-hook executable for the isolated server")
    args = parser.parse_args()
    batch_started=time.monotonic()
    if not 8 <= args.pushes <= 10000 or not 2 <= args.workers <= 32:
        parser.error("Use 8..10000 transactions and 2..32 workers")
    try:
        worker_counts=[int(value) for value in args.worker_counts.split(',')] if args.worker_counts else None
        if worker_counts and any(not 2<=value<=32 for value in worker_counts):
            raise ValueError()
    except ValueError:
        parser.error('Use comma-separated worker counts from 2 to 32')
    server = args.server.resolve(strict=True)
    git = args.git.resolve(strict=True)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    identity = uuid.uuid4().hex
    name = "gitsss-push-acceptance-" + identity
    volume = name + "-data"
    init = name + "-init"
    owner = "gitsss.owner=push-throughput-" + identity
    report = dict(status="fail", same_repository=True, same_branch="refs/heads/main", target_pushes_per_second=100,
                  server_sha256=hashlib.file_digest(server.open("rb"), "sha256").hexdigest(),
                  personal_account_accessed=False, git_objects_relocated=False, credentials_logged=False,
                  storage="Docker persistent volume; no repository tmpfs", repository_fsync_policy="unchanged Git defaults",
                  authentication=args.authentication, protocol_client_location=args.load_location,
                  sqlite_journal_mode=args.sqlite_journal_mode, sqlite_synchronous_policy=args.sqlite_synchronous,
                  results=[])
    go_binary=shutil.which('go')
    if go_binary:
        metadata=subprocess.run([go_binary,'version','-m',str(server)],capture_output=True,text=True,timeout=10)
        settings={}
        for line in metadata.stdout.splitlines():
            fields=line.strip().split('\t')
            if len(fields)==2 and fields[0]=='build' and '=' in fields[1]:
                key,value=fields[1].split('=',1)
                if key in ('CGO_ENABLED','GOOS','GOARCH','-tags'):
                    settings[key]=value
        report['server_build']=settings
        tags=settings.get('-tags','').split(',')
        cgo_driver=settings.get('CGO_ENABLED')=='1' and 'sqlite_unlock_notify' in tags and ('sqlite' in tags or 'sqlite_mattn' in tags)
        report['sqlite_driver']='mattn-cgo' if cgo_driver else 'modernc-pure-go'
        report['sqlite_synchronous_configured_or_driver_default']=args.sqlite_synchronous if args.sqlite_synchronous!='default' else ('NORMAL' if cgo_driver else 'FULL')
    stage = "prepare"
    profile_pool = None
    profiles = []
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    username = "pushbench"
    password = secrets.token_urlsafe(32)
    authorization = "Basic " + base64.b64encode((username+":"+password).encode()).decode()
    fixture_password_authorization = authorization

    def docker(arguments, data=None, check=True, timeout=120):
        p = subprocess.run(["docker", *arguments], input=data, capture_output=True, timeout=timeout)
        if check and p.returncode:
            raise RuntimeError("Docker operation failed at " + stage)
        return p

    def api(path, method="GET", body=None, authenticated=True, auth_header=None):
        request = urllib.request.Request(base+"/api/v1"+path, None if body is None else json.dumps(body).encode(), method=method)
        if authenticated:
            request.add_header("Authorization", authorization if auth_header is None else auth_header)
        request.add_header("Content-Type", "application/json")
        with opener.open(request, timeout=30) as response:
            data = response.read()
            return json.loads(data) if data else None

    def wait_ready():
        deadline = time.monotonic()+90
        while time.monotonic() < deadline:
            try:
                with opener.open(base+"/api/healthz", timeout=2) as response:
                    if response.status == 200:
                        return
            except (OSError, urllib.error.URLError):
                pass
            time.sleep(.2)
        raise RuntimeError("Isolated server did not become healthy")

    def start_server():
        docker(["exec", "-d", name, "/accept/bin/gitsss", "--work-path", "/accept", "--config", "/accept/conf/app.ini", "web"])
        wait_ready()

    def post(old, new, objects, connection=None, authenticated=True, auth_header=None):
        payload = pkt((old+" "+new+" refs/heads/main\0report-status\n").encode()) + b"0000" + pack(objects)
        conn = connection or http.client.HTTPConnection("127.0.0.1", port, timeout=30)
        headers = {"Content-Type": "application/x-git-receive-pack-request", "Accept": "application/x-git-receive-pack-result"}
        if authenticated:
            headers["Authorization"] = authorization if auth_header is None else auth_header
        then = time.monotonic()
        conn.request("POST", repo_url+"/git-receive-pack", payload, headers)
        response = conn.getresponse()
        data = response.read()
        elapsed = time.monotonic()-then
        if connection is None:
            conn.close()
        if response.status != 200:
            return False, elapsed, len(payload), response.status
        accepted = b"unpack ok\n" in data and b"ok refs/heads/main\n" in data and b"ng refs/heads/main" not in data
        return accepted, elapsed, len(payload), response.status

    try:
        image = json.loads(docker(["image", "inspect", args.image]).stdout)[0]["Id"]
        report["image_id"] = image
        docker(["volume", "create", "--label", owner, volume])
        docker(["run", "--rm", "--name", init, "--label", owner, "--network", "none", "--mount", "type=volume,source="+volume+",target=/accept",
                "--mount", "type=bind,source="+str(server)+",target=/input/gitsss,readonly", image,
                "sh", "-c", "mkdir -p /accept/conf /accept/home /accept/data /accept/repos /accept/bin; cp /input/gitsss /accept/bin/gitsss; chmod 755 /accept/bin/gitsss; chown -R 65534:65534 /accept"])
        if args.hook_relay:
            relay=args.hook_relay.resolve(strict=True)
            docker(["run","--rm","--name",init,"--label",owner,"--network","none","--mount","type=volume,source="+volume+",target=/accept",
                    "--mount","type=bind,source="+str(relay)+",target=/input/gitsss-hook,readonly",image,
                    "sh","-c","cp /input/gitsss-hook /accept/bin/gitsss-hook; chmod 755 /accept/bin/gitsss-hook; chown 65534:65534 /accept/bin/gitsss-hook"])
            report['hook_relay_sha256']=hashlib.file_digest(relay.open('rb'),'sha256').hexdigest()
        docker(["run", "-d", "--name", name, "--label", owner, "--user", "65534:65534", "--read-only", "--tmpfs", "/tmp:rw,exec,size=512m",
                "--mount", "type=volume,source="+volume+",target=/accept",
                "--mount", "type=bind,source="+str(Path(__file__).resolve().parents[1])+",target=/source,readonly",
                "-e", "HOME=/accept/home", "-e", "USERPROFILE=/accept/home", "-e", "SSH_ROOT_PATH=/accept/ssh", "-e", "GIT_CONFIG_GLOBAL=/accept/home/.gitconfig",
                *(["-e", "GIT_TRACE2_EVENT=/accept/git-trace.json"] if args.diagnostics_dir else []),
                "-p", "127.0.0.1::3000", image, "sh", "-c", "sleep infinity"])
        inspected = json.loads(docker(["inspect", name]).stdout)[0]
        report['server_git_version']=docker(['exec',name,'git','--version']).stdout.decode().strip()
        port = int(inspected["NetworkSettings"]["Ports"]["3000/tcp"][0]["HostPort"])
        base = "http://127.0.0.1:"+str(port)
        config = """RUN_USER = nobody
RUN_MODE = prod
WORK_PATH = /accept
[server]
APP_DATA_PATH = /accept/data
STATIC_ROOT_PATH = /source
HTTP_ADDR = 0.0.0.0
HTTP_PORT = 3000
ROOT_URL = ROOT_PLACEHOLDER
LOCAL_ROOT_URL = http://127.0.0.1:3000/
ENABLE_PPROF = PPROF_PLACEHOLDER
DISABLE_SSH = true
START_SSH_SERVER = false
SSH_CREATE_AUTHORIZED_KEYS_FILE = false
SSH_CREATE_AUTHORIZED_PRINCIPALS_FILE = false
SSH_ROOT_PATH = /accept/ssh
LFS_START_SERVER = true
[database]
DB_TYPE = sqlite3
PATH = /accept/data/gitsss.db
SQLITE_JOURNAL_MODE = JOURNAL_PLACEHOLDER
SQLITE_SYNCHRONOUS = SYNCHRONOUS_PLACEHOLDER
[repository]
ROOT = /accept/repos
DEFAULT_BRANCH = main
[security]
INSTALL_LOCK = true
INTERNAL_TOKEN = INTERNAL_PLACEHOLDER
SECRET_KEY = SECRET_PLACEHOLDER
[service]
DISABLE_REGISTRATION = true
ENABLE_NOTIFY_MAIL = false
[picture]
DISABLE_GRAVATAR = true
ENABLE_FEDERATED_AVATAR = false
[actions]
ENABLED = false
[session]
PROVIDER = memory
[log]
MODE = console
LEVEL = Warn
ENABLE_ACCESS_LOG = false
""".replace("INTERNAL_PLACEHOLDER", secrets.token_urlsafe(48)).replace("SECRET_PLACEHOLDER", secrets.token_urlsafe(48)).replace("PPROF_PLACEHOLDER",str(bool(args.diagnostics_dir)).lower()).replace("JOURNAL_PLACEHOLDER",'' if args.sqlite_journal_mode=='default' else args.sqlite_journal_mode).replace('SYNCHRONOUS_PLACEHOLDER','' if args.sqlite_synchronous=='default' else args.sqlite_synchronous).replace('ROOT_PLACEHOLDER',base+'/')
        docker(["exec", "-i", name, "sh", "-c", "umask 077; cat > /accept/conf/app.ini"], config.encode())
        stage = "database migration"
        docker(["exec", name, "/accept/bin/gitsss", "--work-path", "/accept", "--config", "/accept/conf/app.ini", "migrate"], timeout=180)
        stage = "generated test administrator"
        docker(["exec", name, "/accept/bin/gitsss", "--work-path", "/accept", "--config", "/accept/conf/app.ini", "admin", "user", "create",
                "--username", username, "--password", password, "--email", "fixture@example.invalid", "--admin", "--must-change-password=false"])
        stage = "server startup"
        start_server()
        if args.authentication == 'personal-access-token':
            stage = "generated personal access token"
            token=api('/users/'+username+'/tokens','POST',dict(name='push-throughput-fixture',scopes=['all']))['sha1']
            authorization="Basic "+base64.b64encode((username+':'+token).encode()).decode()
            token=None
        stage = "create repository"
        created_repo=api("/user/repos", "POST", dict(name="throughput", private=True, default_branch="main", auto_init=False))
        repo_url = "/"+username+"/throughput.git"
        current, objects = fixture_commit(None, 0)
        assert post("0"*40, current, objects)[0], "Initial Git protocol push failed"
        for _ in range(100):
            if not api("/repos/"+username+"/throughput")["empty"]:
                break
            time.sleep(.1)
        else:
            raise RuntimeError("Initial push metadata did not drain")
        repositories=docker(["exec",name,"find","/accept","-type","d","-name","throughput.git"]).stdout.decode().splitlines()
        assert len(repositories)==1 and repositories[0].startswith('/accept/'), "Generated repository path is ambiguous"
        bare=repositories[0]
        hooks=bare+'/hooks'
        def write_fixture(path, data):
            docker(['exec','-i',name,'sh','-c','umask 077; cat > "$1"; chmod 700 "$1"','fixture',path],data)
        def hooks_identity():
            return docker(['exec',name,'sh','-c','find "$1" -type f -print0 | sort -z | xargs -0 sha256sum','fixture',hooks]).stdout
        original_hooks=hooks_identity()
        original_config=docker(['exec',name,'cat',bare+'/config']).stdout
        stage='custom hook dispatch, stdin, ordering and rejection contracts'
        custom_paths=[]
        for hook in ('pre-receive','update','post-receive'):
            for order in ('001-before','zzz-after'):
                path=hooks+'/'+hook+'.d/'+order
                custom_paths.append(path)
                script=('#!/bin/sh\n[ -z "${GITSSS_HOOK_TOKEN:-}" ] || exit 91\nprintf "%s\\n" "'+hook+':'+order+'" >> /accept/hook-order\n')
                if hook=='update':script+='printf "%s %s %s\\n" "$1" "$2" "$3" > /accept/'+hook+'-'+order+'-input\n'
                else:script+='cat > /accept/'+hook+'-'+order+'-input\n'
                write_fixture(path,script.encode())
        next_oid,objects=fixture_commit(current,400001)
        assert post(current,next_oid,objects)[0], 'Custom hook dispatch rejected a valid push'
        order=docker(['exec',name,'cat','/accept/hook-order']).stdout.decode().splitlines()
        assert order==[hook+':'+suffix for hook in ('pre-receive','update','post-receive') for suffix in ('001-before','zzz-after')], 'Custom hook order changed'
        for hook in ('pre-receive','update','post-receive'):
            expected=((('refs/heads/main '+current+' '+next_oid) if hook=='update' else (current+' '+next_oid+' refs/heads/main'))+'\n').encode()
            for suffix in ('001-before','zzz-after'):
                assert docker(['exec',name,'cat','/accept/'+hook+'-'+suffix+'-input']).stdout==expected, 'Custom hook stdin/arguments changed'
        current=next_oid
        write_fixture(hooks+'/pre-receive.d/001-before',b'#!/bin/sh\n[ -z "${GITSSS_HOOK_TOKEN:-}" ] || exit 91\nexit 23\n')
        rejected_oid,objects=fixture_commit(current,400002)
        assert not post(current,rejected_oid,objects)[0], 'Rejecting custom hook was bypassed'
        assert docker(['exec',name,'git','--git-dir='+bare,'rev-parse','refs/heads/main']).stdout.decode().strip()==current
        docker(['exec',name,'rm',*custom_paths])
        assert hooks_identity()==original_hooks, 'Managed hook bytes changed during relay execution'
        report['results'].append(dict(case='custom-hook-order-input-arguments-rejection-and-credential-isolation',status='pass'))
        stage='explicit original hooksPath and custom dispatcher fallback'
        docker(['exec',name,'git','--git-dir='+bare,'config','core.hooksPath',hooks])
        next_oid,objects=fixture_commit(current,400003)
        assert post(current,next_oid,objects)[0], 'Explicit hooksPath stopped working'
        current=next_oid
        docker(['exec',name,'git','--git-dir='+bare,'config','--unset','core.hooksPath'])
        original_pre=docker(['exec',name,'cat',hooks+'/pre-receive']).stdout
        write_fixture(hooks+'/pre-receive',original_pre.replace(b'\n',b'\n# independent dispatcher fixture\n',1))
        next_oid,objects=fixture_commit(current,400004)
        assert post(current,next_oid,objects)[0], 'Custom top-level dispatcher stopped working'
        current=next_oid
        write_fixture(hooks+'/pre-receive',original_pre)
        assert hooks_identity()==original_hooks and docker(['exec',name,'cat',bare+'/config']).stdout==original_config
        report['results'].append(dict(case='explicit-hooks-path-and-custom-dispatcher-fallback-without-persistent-rewrites',status='pass'))
        stage='protected branch policy'
        api('/repos/'+username+'/throughput/branch_protections','POST',dict(branch_name='main',rule_name='main',enable_push=False))
        next_oid,objects=fixture_commit(current,400005)
        assert not post(current,next_oid,objects)[0], 'Protected branch rejection was bypassed'
        assert docker(['exec',name,'git','--git-dir='+bare,'rev-parse','refs/heads/main']).stdout.decode().strip()==current
        api('/repos/'+username+'/throughput/branch_protections/main','DELETE')
        report['results'].append(dict(case='protected-branch-rejection-preserves-native-reference',status='pass'))
        stage='read-only and revoked token rejection'
        for scope in ('read:repository','all'):
            generated=api('/users/'+username+'/tokens','POST',dict(name='fixture-'+scope.replace(':','-'),scopes=[scope]),auth_header=fixture_password_authorization)
            temporary_auth='Basic '+base64.b64encode((username+':'+generated['sha1']).encode()).decode()
            if scope=='all':api('/users/'+username+'/tokens/'+str(generated['id']),'DELETE',auth_header=fixture_password_authorization)
            next_oid,objects=fixture_commit(current,400006)
            outcome=post(current,next_oid,objects,auth_header=temporary_auth)
            assert not outcome[0] and outcome[3] in (401,403,404), 'Read-only/revoked token was accepted'
            if scope!='all':api('/users/'+username+'/tokens/'+str(generated['id']),'DELETE',auth_header=fixture_password_authorization)
            generated=None;temporary_auth=None
        report['results'].append(dict(case='read-only-and-revoked-authentication-rejected',status='pass'))
        stage = "sequential same-branch protocol pushes"
        if args.diagnostics_dir:
            profile_pool = ThreadPoolExecutor(2)
            for filename, endpoint in [('cpu.pprof','/debug/pprof/profile'),('wall.pprof','/debug/fgprof')]:
                profiles.append(profile_pool.submit(docker,['exec',name,'curl','-fsS','-o','/accept/'+filename,
                    'http://localhost:6060'+endpoint+'?seconds=20'],check=False,timeout=30))
        parameters=dict(host='127.0.0.1',port=port,path=repo_url,authorization=authorization,current=current,pushes=args.pushes,workers=args.workers,worker_counts=worker_counts,scheduling=args.concurrent_scheduling)
        if args.load_location=='container-loopback':
            parameters['port']=3000
            driver='import json,runpy,sys; m=runpy.run_path("/source/tools/benchmark-push.py"); print(json.dumps(m["protocol_load"](**json.load(sys.stdin))))'
            load=json.loads(docker(['exec','-i',name,'python3','-c',driver],json.dumps(parameters).encode(),timeout=240).stdout)
        else:
            load=protocol_load(**parameters)
        current=load['current']; report['results'].extend(load['results'])
        actual_database=docker(['exec',name,'python3','-c',
            'import sqlite3,sys; c=sqlite3.connect("file:/accept/data/gitsss.db?mode=ro",uri=True,timeout=30); print(c.execute("select commit_id from branch where repo_id=? and name=? and is_deleted=0",(int(sys.argv[1]),"main")).fetchone()[0])',str(created_repo['id'])]).stdout.decode().strip()
        report['results'].append(dict(case='concurrent-native-reference-and-actual-database-row-match',status='pass' if actual_database==current else 'fail'))
        stage = "same-old-head conflict correctness"
        variants=[fixture_commit(current, 100000+i) for i in range(2)]
        with ThreadPoolExecutor(2) as pool:
            outcomes=list(pool.map(lambda entry:post(current, entry[0], entry[1]), variants))
        assert sum(item[0] for item in outcomes)==1, "Competing same-old pushes did not have exactly one winner"
        current=variants[next(i for i,item in enumerate(outcomes) if item[0])][0]
        denied=fixture_commit(current, 200000)
        assert post(current, denied[0], denied[1], authenticated=False)[3] in (401,403,404), "Unauthenticated private push was accepted"
        report["results"].append(dict(case="same-old-head-single-winner-and-private-authentication", status="pass"))
        stage = "parallel installed-client functional verification"
        def run_installed_cli():
            with tempfile.TemporaryDirectory(prefix="gitsss-push-client-") as temporary:
                home=Path(temporary)/"home";home.mkdir();work=Path(temporary)/"work"
                env={key:value for key,value in os.environ.items() if not key.startswith(("GIT_", "GCM_", "SSH_", "GNUPG"))}
                env.update(HOME=str(home),USERPROFILE=str(home),APPDATA=str(home/"appdata"),LOCALAPPDATA=str(home/"local"),
                           XDG_CONFIG_HOME=str(home/".config"),SSH_ROOT_PATH=str(home/".ssh"),GIT_CONFIG_GLOBAL=str(home/".gitconfig"),GIT_CONFIG_NOSYSTEM="1",
                           GIT_TERMINAL_PROMPT="0",GCM_INTERACTIVE="never",GIT_CONFIG_COUNT="2",GIT_CONFIG_KEY_0="http.extraHeader",GIT_CONFIG_VALUE_0="Authorization: "+authorization,
                           GIT_CONFIG_KEY_1="credential.helper",GIT_CONFIG_VALUE_1="",GIT_AUTHOR_NAME="Fixture",GIT_COMMITTER_NAME="Fixture",
                           GIT_AUTHOR_EMAIL="fixture@example.invalid",GIT_COMMITTER_EMAIL="fixture@example.invalid")
                def client(arguments, data=None):
                    p=subprocess.run([str(git),*arguments],cwd=work if work.exists() else home,env=env,input=data,capture_output=True,timeout=60)
                    if p.returncode:raise RuntimeError("Ordinary Git client failed at "+stage)
                    return p.stdout
                client(["clone", "--quiet", base+repo_url, str(work)])
                latencies=[];started=time.monotonic()
                for i in range(args.cli_pushes):
                    (work/"sample.txt").write_bytes(("CLI increment %d\n"%i).encode().ljust(1024,b"x"))
                    client(["add","sample.txt"]);client(["commit","--quiet","-m","generated CLI increment"])
                    then=time.monotonic();client(["push","--porcelain","origin","HEAD:refs/heads/main"]);latencies.append(time.monotonic()-then)
                # Exclude authoring from push throughput while keeping every actual process/network operation.
                record=dict(case="ordinary-windows-git-cli", parallel_functional_suite=args.deep_compatibility, client_sha256=hashlib.file_digest(git.open("rb"),"sha256").hexdigest(),
                                              authoring_and_push_elapsed_seconds=round(time.monotonic()-started,6),
                                              **summary(latencies,sum(latencies),len(latencies),0))
                head=client(["rev-parse","HEAD"]).decode().strip()
                client(["fsck","--strict"])
            return head,record
        def run_deep_compatibility():
            assert args.authentication=='personal-access-token', 'Deep compatibility needs the generated PAT'
            with tempfile.TemporaryDirectory(prefix='gitsss-deep-compatibility-') as directory:
                isolated=Path(directory); token_file=isolated/'fixture-token'
                fixture_token=base64.b64decode(authorization.removeprefix('Basic ')).decode().split(':',1)[1]
                token_file.write_text(fixture_token,encoding='utf-8'); token_file.chmod(0o600); fixture_token=None
                environment=os.environ.copy()
                environment.update(HOME=directory,USERPROFILE=directory,SSH_ROOT_PATH=str(isolated/'ssh'),
                    GIT_CONFIG_GLOBAL=str(isolated/'empty.gitconfig'),GIT_CONFIG_NOSYSTEM='1',
                    PATH=str(git.parent)+os.pathsep+environment.get('PATH',''))
                verification=subprocess.run([sys.executable,str(Path(__file__).with_name('verify-git.py')),
                    '--url',base,'--token-file',str(token_file)],capture_output=True,text=True,
                    encoding='utf-8',errors='replace',env=environment,timeout=180)
                result=json.loads(verification.stdout)
                if result.get('error'):
                    result['error']=result['error'].replace(token_file.read_text(encoding='utf-8'),'[fixture-secret]')
                assert verification.returncode==0 and result['status']=='pass', 'Installed Git/LFS/PR compatibility failed'
            return result
        with ThreadPoolExecutor(2) as functional_pool:
            cli_future=functional_pool.submit(run_installed_cli)
            deep_future=functional_pool.submit(run_deep_compatibility) if args.deep_compatibility else None
            current,cli_result=cli_future.result()
            report['results'].append(cli_result)
            if deep_future:
                report['deep_compatibility']=deep_future.result()
                report['results'].append(dict(case='installed-git-lfs-pr-merge-clone-tags-notes-and-permissions',status='pass'))
        stage="persistent storage and database readback"
        for profile in profiles:
            profile.result(timeout=30)
        repositories=docker(["exec",name,"find","/accept","-type","d","-name","throughput.git"]).stdout.decode().splitlines()
        assert len(repositories)==1 and repositories[0].startswith('/accept/'), "Generated repository path is ambiguous"
        bare=repositories[0]
        assert docker(["exec", name, "git", "--git-dir="+bare, "rev-parse", "refs/heads/main"]).stdout.decode().strip()==current
        stage="server-side object integrity"
        docker(["exec", name, "git", "--git-dir="+bare, "fsck", "--strict"])
        assert docker(["exec", name, "sh", "-c", "test ! -e /accept/ssh/authorized_keys && test ! -e /accept/ssh/authorized_principals"]).returncode==0
        stage="restart and durable reference readback"
        docker(["restart", name])
        inspected=json.loads(docker(['inspect',name]).stdout)[0]
        port=int(inspected['NetworkSettings']['Ports']['3000/tcp'][0]['HostPort'])
        base='http://127.0.0.1:'+str(port)
        start_server()
        assert docker(["exec", name, "git", "--git-dir="+bare, "rev-parse", "refs/heads/main"]).stdout.decode().strip()==current
        stage="branch database readback"
        branch=api("/repos/"+username+"/throughput/branches/main")
        assert branch["commit"]["id"]==current, "Branch database did not match durable ref"
        report["results"].append(dict(case="restart-object-integrity-branch-database-and-ssh-isolation",status="pass"))
        assert hooks_identity()==original_hooks and docker(['exec',name,'cat',bare+'/config']).stdout==original_config, 'Persistent hooks/configuration changed'
        report["status"]="fail" if any(item.get('status')=='fail' for item in report['results']) else "pass"
        report["target_met"]=report['status']=='pass' and max(item.get("successful_pushes_per_second",0) for item in report["results"] if item["case"].endswith(("smart-http","linear-chain")))>=100
    except Exception as error:
        report.update(failed_stage=stage, failure_type=type(error).__name__, target_met=False)
        if isinstance(error,(AssertionError,RuntimeError)): report['failure_reason']=str(error)
    finally:
        if profile_pool:
            profile_pool.shutdown(wait=True)
        if args.diagnostics_dir:
            diagnostics=args.diagnostics_dir.resolve()
            diagnostics.mkdir(parents=True,exist_ok=True)
            trace=diagnostics/'git-trace.json'
            copied=docker(["cp",name+":/accept/git-trace.json",str(trace)],check=False)
            report['private_diagnostics']=dict(trace_captured=copied.returncode==0,
                trace_sha256=hashlib.file_digest(trace.open('rb'),'sha256').hexdigest() if trace.exists() else None,
                published=False)
            for filename in ('cpu.pprof','wall.pprof'):
                docker(['cp',name+':/accept/'+filename,str(diagnostics/filename)],check=False)
        cleanup=[]
        for resource,kind in [(init,"container"),(name,"container"),(volume,"volume")]:
            probe=docker((["volume","inspect",resource] if kind=="volume" else ["inspect",resource]),check=False)
            if probe.returncode:continue
            value=json.loads(probe.stdout)[0]
            labels=value.get("Labels",{}) if kind=="volume" else value["Config"].get("Labels",{})
            if labels.get("gitsss.owner")!="push-throughput-"+identity:
                cleanup.append(dict(resource=kind,status="ownership-refused"));continue
            p=docker((["volume","rm",resource] if kind=="volume" else ["rm","-f",resource]),check=False)
            cleanup.append(dict(resource=kind,status="removed" if p.returncode==0 else "fail"))
        report["cleanup"]=cleanup
        report['batch_elapsed_seconds']=round(time.monotonic()-batch_started,3)
        if any(item["status"]!="removed" for item in cleanup):report["status"]="fail"
        args.output.write_text(json.dumps(report,indent=2)+"\n",encoding="utf-8",newline="\n")
        print(json.dumps(report))
    return report["status"]!="pass"


if __name__=="__main__":
    raise SystemExit(main())
