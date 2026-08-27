#!/usr/bin/env python3
"""Deploy and fix fluxsearch-monitor on remote server."""
import os
import sys
import time
import paramiko

# Windows 控制台默认 GBK，避免打印远程日志时 UnicodeEncodeError
if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
if hasattr(sys.stderr, "reconfigure"):
    sys.stderr.reconfigure(encoding="utf-8", errors="replace")

HOST = os.environ.get("FLUXSEARCH_DEPLOY_HOST", "113.128.132.69")
USER = os.environ.get("FLUXSEARCH_DEPLOY_USER", "deploy")
PASSWORD = os.environ.get("FLUXSEARCH_DEPLOY_PASSWORD", "")
INFRA_PASSWORD = os.environ.get("FLUXSEARCH_POSTGRES_PASSWORD", "changeme-in-production")
REMOTE_DIR = f"/home/{USER}/fluxsearch"
LOCAL_BINARY = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "fluxsearch-monitor")
REMOTE_SCRIPT = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "deploy", "scripts", "server-run-monitor.sh")


def run(ssh, cmd, timeout=60):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    code = stdout.channel.recv_exit_status()
    return code, out, err


def main():
    if not PASSWORD:
        print("ERROR: set FLUXSEARCH_DEPLOY_PASSWORD")
        sys.exit(1)
    if not os.path.exists(LOCAL_BINARY):
        print(f"ERROR: binary not found: {LOCAL_BINARY}")
        sys.exit(1)

    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=15, allow_agent=False, look_for_keys=False)

    print("=== 1. Upload binary ===")
    code, out, err = run(ssh, f"mkdir -p {REMOTE_DIR} && ls -la {REMOTE_DIR}")
    print(out or err)
    sftp = ssh.open_sftp()
    remote_bin = f"{REMOTE_DIR}/fluxsearch-monitor"
    try:
        sftp.remove(remote_bin)
    except OSError:
        pass
    sftp.put(LOCAL_BINARY, remote_bin)
    with open(REMOTE_SCRIPT, "r", encoding="utf-8") as f:
        script = f.read()
    with sftp.file(f"{REMOTE_DIR}/server-run-monitor.sh", "w") as rf:
        rf.write(script)
    sftp.close()
    run(ssh, f"chmod +x {REMOTE_DIR}/fluxsearch-monitor {REMOTE_DIR}/server-run-monitor.sh")

    print("=== 2. Diagnose cluster DNS from host ===")
    _, out, _ = run(ssh, "getent hosts postgres.fluxsearch.svc.cluster.local 2>&1 || nslookup postgres.fluxsearch.svc.cluster.local 2>&1 | head -5")
    print(out.strip() or "(no dns result)")

    print("=== 3. Get cluster service ClusterIPs ===")
    _, out, _ = run(ssh, f"echo {PASSWORD} | sudo -S kubectl get svc -n fluxsearch -o wide 2>/dev/null")
    print(out)

    print("=== 4. Resolve service IPs for monitor env ===")
    get_ip = """echo {pw} | sudo -S bash -c '
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
PG=$(kubectl get svc postgres -n fluxsearch -o jsonpath={{.spec.clusterIP}})
RD=$(kubectl get svc redis -n fluxsearch -o jsonpath={{.spec.clusterIP}})
MI=$(kubectl get svc minio -n fluxsearch -o jsonpath={{.spec.clusterIP}})
MV=$(kubectl get svc milvus -n fluxsearch -o jsonpath={{.spec.clusterIP}})
ET=$(kubectl get svc etcd -n fluxsearch -o jsonpath={{.spec.clusterIP}})
echo PG=$PG RD=$RD MI=$MI MV=$MV ET=$ET
'"""
    _, out, err = run(ssh, get_ip.format(pw=PASSWORD))
    print(out)
    ips = {}
    for part in out.strip().split():
        if "=" in part:
            k, v = part.split("=", 1)
            ips[k] = v

    if not ips.get("PG"):
        print("ERROR: cannot get cluster IPs", err)
        ssh.close()
        sys.exit(1)

    print("=== 5. Stop old monitor ===")
    run(ssh, "pkill -f fluxsearch-monitor 2>/dev/null || true")
    time.sleep(1)

    print("=== 6. Write monitor.env and start monitor ===")
    env_content = f"""FLUXSEARCH_POSTGRES_HOST={ips["PG"]}
FLUXSEARCH_POSTGRES_PORT=5432
FLUXSEARCH_POSTGRES_USER=fluxsearch
FLUXSEARCH_POSTGRES_PASSWORD={INFRA_PASSWORD}
FLUXSEARCH_POSTGRES_DB=fluxsearch
FLUXSEARCH_REDIS_HOST={ips["RD"]}
FLUXSEARCH_REDIS_PORT=6379
FLUXSEARCH_REDIS_PASSWORD={INFRA_PASSWORD}
FLUXSEARCH_MINIO_ENDPOINT={ips["MI"]}:9000
FLUXSEARCH_MINIO_ACCESS_KEY=fluxsearch
FLUXSEARCH_MINIO_SECRET_KEY={INFRA_PASSWORD}
FLUXSEARCH_MINIO_BUCKET=milvus-bucket
FLUXSEARCH_MILVUS_HOST={ips["MV"]}
FLUXSEARCH_MILVUS_PORT=19530
FLUXSEARCH_ETCD_HOST={ips["ET"]}
FLUXSEARCH_ETCD_PORT=2379
FLUXSEARCH_MONITOR_ADDR=0.0.0.0:8090
"""
    sftp = ssh.open_sftp()
    with sftp.file(f"{REMOTE_DIR}/monitor.env", "w") as ef:
        ef.write(env_content)
    sftp.close()

    start_cmd = f"""bash -c 'set -a && source {REMOTE_DIR}/monitor.env && set +a
nohup {REMOTE_DIR}/fluxsearch-monitor > /tmp/fluxsearch-monitor.log 2>&1 &
sleep 3
pgrep -af fluxsearch-monitor || true
curl -s http://127.0.0.1:8090/healthz || echo health_fail
curl -s http://127.0.0.1:8090/api/v1/status | head -c 500 || echo status_fail
'"""
    code, out, err = run(ssh, start_cmd, timeout=30)
    print(out)
    if err.strip():
        print("STDERR:", err)

    print("=== 7. Check log if needed ===")
    _, out, _ = run(ssh, "tail -30 /tmp/fluxsearch-monitor.log 2>/dev/null")
    print(out)

    print("=== 8. External access test ===")
    _, out, _ = run(ssh, f"curl -s -o /dev/null -w '%{{http_code}}' http://127.0.0.1:8090/api/v1/status")
    print(f"HTTP status from server: {out.strip()}")

    ssh.close()
    print(f"\nDONE. Monitor URL: http://{HOST}:8090/api/v1/status")

if __name__ == "__main__":
    main()
