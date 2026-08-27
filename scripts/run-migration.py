#!/usr/bin/env python3
"""Upload and apply SQL migrations on remote PostgreSQL (K3s fluxsearch namespace)."""
import os
import sys
import paramiko

HOST = os.environ.get("FLUXSEARCH_DEPLOY_HOST", "113.128.132.69")
USER = os.environ.get("FLUXSEARCH_DEPLOY_USER", "deploy")
PASSWORD = os.environ.get("FLUXSEARCH_DEPLOY_PASSWORD", "")
NAMESPACE = "fluxsearch"
PG_USER = "fluxsearch"
PG_DB = "fluxsearch"
PG_PASSWORD = os.environ.get("FLUXSEARCH_POSTGRES_PASSWORD", "changeme")

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MIGRATIONS_DIR = os.path.join(ROOT, "migrations")

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")


def run(ssh, cmd, timeout=120):
    stdin, stdout, stderr = ssh.exec_command(cmd, timeout=timeout)
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    code = stdout.channel.recv_exit_status()
    return code, out, err


def main():
    migration = sys.argv[1] if len(sys.argv) > 1 else "001_init.sql"
    sql_path = os.path.join(MIGRATIONS_DIR, migration)
    if not os.path.isfile(sql_path):
        print(f"ERROR: migration not found: {sql_path}")
        sys.exit(1)

    with open(sql_path, "r", encoding="utf-8") as f:
        sql = f.read()

    if not PASSWORD:
        print("ERROR: set FLUXSEARCH_DEPLOY_PASSWORD")
        sys.exit(1)

    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(HOST, username=USER, password=PASSWORD, timeout=15, allow_agent=False, look_for_keys=False)

    remote_sql = f"/tmp/{migration}"
    sftp = ssh.open_sftp()
    with sftp.file(remote_sql, "w") as rf:
        rf.write(sql)
    sftp.close()

    print(f"=== Apply {migration} ===")
    cmd = (
        f"echo {PASSWORD} | sudo -S bash -c '"
        f"export KUBECONFIG=/etc/rancher/k3s/k3s.yaml; "
        f"POD=$(kubectl get pod -n {NAMESPACE} -l app=postgres -o jsonpath=\"{{.items[0].metadata.name}}\"); "
        f"echo Using pod: $POD; "
        f"kubectl cp {remote_sql} {NAMESPACE}/$POD:/tmp/{migration}; "
        f"kubectl exec -n {NAMESPACE} $POD -- env PGPASSWORD={PG_PASSWORD} "
        f"psql -U {PG_USER} -d {PG_DB} -v ON_ERROR_STOP=1 -f /tmp/{migration}; "
        f"kubectl exec -n {NAMESPACE} $POD -- env PGPASSWORD={PG_PASSWORD} "
        f"psql -U {PG_USER} -d {PG_DB} -c \"\\dt\"; "
        f"kubectl exec -n {NAMESPACE} $POD -- env PGPASSWORD={PG_PASSWORD} "
        f"psql -U {PG_USER} -d {PG_DB} -c \"SELECT version, applied_at FROM schema_migrations ORDER BY applied_at;\""
        f"'"
    )
    code, out, err = run(ssh, cmd, timeout=180)
    print(out)
    if err.strip():
        print(err)
    ssh.close()

    if code != 0:
        print(f"Migration failed (exit {code})")
        sys.exit(code)

    print(f"\nDONE: {migration} applied on {HOST}")


if __name__ == "__main__":
    main()
