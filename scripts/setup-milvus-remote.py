#!/usr/bin/env python3
"""Build and run ensure-milvus on remote server with ClusterIP env."""
import os
import subprocess
import sys
import paramiko

HOST = os.environ.get("FLUXSEARCH_DEPLOY_HOST", "113.128.132.69")
USER = os.environ.get("FLUXSEARCH_DEPLOY_USER", "deploy")
PASSWORD = os.environ.get("FLUXSEARCH_DEPLOY_PASSWORD", "")
REMOTE_DIR = f"/home/{USER}/fluxsearch"
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BINARY = os.path.join(ROOT, "fluxsearch-ensure-milvus")

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")


def run(ssh, cmd, timeout=120):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    code = stdout.channel.recv_exit_status()
    return code, out, err


def main():
    if not PASSWORD:
        print("ERROR: set FLUXSEARCH_DEPLOY_PASSWORD")
        sys.exit(1)
    print("=== Build linux binary ===")
    env = os.environ.copy()
    env["GOOS"] = "linux"
    env["GOARCH"] = "amd64"
    env["CGO_ENABLED"] = "0"
    env.setdefault("GOPROXY", "https://goproxy.cn,direct")
    subprocess.check_call(
        ["go", "build", "-ldflags=-s -w", "-o", BINARY, "./cmd/ensure-milvus"],
        cwd=ROOT,
        env=env,
    )

    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=15, allow_agent=False, look_for_keys=False)

    sftp = ssh.open_sftp()
    sftp.put(BINARY, f"{REMOTE_DIR}/fluxsearch-ensure-milvus")
    sftp.close()

    print("=== Resolve Milvus ClusterIP ===")
    get_ip = f"""echo {PASSWORD} | sudo -S bash -c '
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
kubectl get svc milvus -n fluxsearch -o jsonpath="{{{{.spec.clusterIP}}}}"
'"""
    _, out, _ = run(ssh, get_ip)
    milvus_ip = out.strip().splitlines()[-1].strip()
    print(f"Milvus IP: {milvus_ip}")

    print("=== Ensure collection ===")
    cmd = (
        f"chmod +x {REMOTE_DIR}/fluxsearch-ensure-milvus && "
        f"FLUXSEARCH_MILVUS_HOST={milvus_ip} FLUXSEARCH_MILVUS_PORT=19530 "
        f"{REMOTE_DIR}/fluxsearch-ensure-milvus"
    )
    code, out, err = run(ssh, cmd, timeout=90)
    print(out)
    if err.strip():
        print(err)
    ssh.close()

    if code != 0:
        sys.exit(code)
    print("DONE")


if __name__ == "__main__":
    main()
