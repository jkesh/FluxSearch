"""Upload and run apply-external-access.sh on remote server."""
import os
import sys

import paramiko

HOST = os.environ.get("FLUXSEARCH_DEPLOY_HOST", "113.128.132.69")
USER = os.environ.get("FLUXSEARCH_DEPLOY_USER", "deploy")
PASSWORD = os.environ.get("FLUXSEARCH_DEPLOY_PASSWORD", "")
PUBLIC_IP = os.environ.get("FLUXSEARCH_PUBLIC_IP", HOST)

SCRIPT = os.path.join(os.path.dirname(__file__), "..", "deploy", "scripts", "apply-external-access.sh")


def main() -> int:
    if not PASSWORD:
        print("ERROR: set FLUXSEARCH_DEPLOY_PASSWORD")
        return 1
    if not os.path.isfile(SCRIPT):
        print(f"ERROR: missing {SCRIPT}")
        return 1

    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=15)

    sftp = ssh.open_sftp()
    with open(SCRIPT, "r", encoding="utf-8") as f:
        content = f.read()
    with sftp.file("/tmp/apply-external-access.sh", "w") as rf:
        rf.write(content)
    sftp.close()

    cmd = (
        f"chmod +x /tmp/apply-external-access.sh && "
        f"FLUXSEARCH_PUBLIC_IP={PUBLIC_IP} "
        f"echo \"$FLUXSEARCH_DEPLOY_PASSWORD\" | sudo -S -E bash /tmp/apply-external-access.sh"
    )
    _, out, err = ssh.exec_command(cmd, timeout=120)
    print(out.read().decode())
    print(err.read().decode())
    ssh.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
