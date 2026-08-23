#!/usr/bin/env python3
"""Upload and run server-port-forward.sh on remote host (credentials via env)."""
import os
import sys
import paramiko

HOST = os.environ.get("FLUXSEARCH_DEPLOY_HOST", "your-k8s-node.example.com")
USER = os.environ.get("FLUXSEARCH_DEPLOY_USER", "deploy")
PW = os.environ.get("FLUXSEARCH_DEPLOY_PASSWORD", "")
SCRIPT = os.path.join(os.path.dirname(__file__), "..", "deploy", "scripts", "server-port-forward.sh")

if not PW:
    print("ERROR: set FLUXSEARCH_DEPLOY_PASSWORD")
    sys.exit(1)

with open(SCRIPT, "r", encoding="utf-8") as f:
    script = f.read()

ssh = paramiko.SSHClient()
ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
ssh.connect(HOST, username=USER, password=PW, timeout=15)
sftp = ssh.open_sftp()
with sftp.file("/tmp/server-port-forward.sh", "w") as rf:
    rf.write(script)
sftp.close()

_, out, err = ssh.exec_command("echo \"$FLUXSEARCH_DEPLOY_PASSWORD\" | sudo -S bash /tmp/server-port-forward.sh", timeout=60)
print(out.read().decode("utf-8", errors="replace"))
e = err.read().decode("utf-8", errors="replace")
if e.strip():
    print(e)
ssh.close()
