#!/usr/bin/env python3
"""Upload and run a script on the remote server via SSH with optional SOCKS tunnel."""
import sys
import socket
import select
import threading
import paramiko


def setup_remote_forward(transport, remote_port, local_host, local_port):
    """SSH -R: expose local proxy on remote 127.0.0.1:remote_port"""
    transport.request_port_forward("127.0.0.1", remote_port)

    def handler():
        while transport.is_active():
            try:
                chan = transport.accept(1000)
            except Exception:
                continue
            if chan is None:
                continue
            thr = threading.Thread(target=_forward, args=(chan, local_host, local_port), daemon=True)
            thr.start()

    t = threading.Thread(target=handler, daemon=True)
    t.start()
    return t


def _forward(chan, local_host, local_port):
  try:
    sock = socket.create_connection((local_host, local_port), timeout=10)
  except Exception:
    chan.close()
    return
  try:
    while True:
      r, _, _ = select.select([sock, chan], [], [], 60)
      if not r:
        break
      if sock in r:
        data = sock.recv(65536)
        if not data:
          break
        chan.send(data)
      if chan in r:
        data = chan.recv(65536)
        if not data:
          break
        sock.send(data)
  finally:
    chan.close()
    sock.close()


def main():
    host = sys.argv[1] if len(sys.argv) > 1 else "your-k8s-node.example.com"
    user = sys.argv[2] if len(sys.argv) > 2 else "deploy"
    password = sys.argv[3] if len(sys.argv) > 3 else ""
    local_script = sys.argv[4] if len(sys.argv) > 4 else "setup-infra.sh"
    use_tunnel = "--tunnel" in sys.argv
    remote_path = "/tmp/fluxsearch-setup.sh"

    if not password:
        print("Usage: remote-run.py <host> <user> <password> <script.sh> [--tunnel]")
        sys.exit(1)

    with open(local_script, "r", encoding="utf-8") as f:
        content = f.read()

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    client.connect(host, username=user, password=password, timeout=15,
                   allow_agent=False, look_for_keys=False)

    if use_tunnel:
        transport = client.get_transport()
        setup_remote_forward(transport, 11080, "127.0.0.1", 10808)
        sys.stderr.write("SSH tunnel: remote 127.0.0.1:11080 -> local 127.0.0.1:10808\n")

    sftp = client.open_sftp()
    with sftp.file(remote_path, "w") as remote_file:
        remote_file.write(content)
    sftp.close()

    cmd = f"echo '{password}' | sudo -S bash {remote_path}"
    stdin, stdout, stderr = client.exec_command(cmd, timeout=900)
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    exit_code = stdout.channel.recv_exit_status()

    sys.stdout.buffer.write(out.encode("utf-8", errors="replace"))
    sys.stdout.buffer.write(b"\n")
    if err.strip():
        sys.stdout.buffer.write(b"STDERR: ")
        sys.stdout.buffer.write(err.encode("utf-8", errors="replace"))
        sys.stdout.buffer.write(b"\n")
    print(f"EXIT: {exit_code}")
    client.close()
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
