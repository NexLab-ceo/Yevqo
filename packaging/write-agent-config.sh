#!/data/data/com.termux/files/usr/bin/bash
set -euo pipefail
umask 077
: "${SERVER_URL:?}" "${TOKEN:?}" "${DEVICE_ID:?}" "${DATA_ROOT:?}" "${HOSTNAME_VALUE:?}" "${CONFIG_FILE:?}"
mkdir -p "$(dirname "$CONFIG_FILE")"
python - "$CONFIG_FILE" <<'PY'
import json, os, sys
path=sys.argv[1]
data={
 "server_url":os.environ["SERVER_URL"],
 "token":os.environ["TOKEN"],
 "device_id":os.environ["DEVICE_ID"],
 "hostname":os.environ["HOSTNAME_VALUE"],
 "data_root":os.environ["DATA_ROOT"]
}
tmp=path+".tmp"
with open(tmp,"w",encoding="utf-8") as f: json.dump(data,f,indent=2)
os.chmod(tmp,0o600)
os.replace(tmp,path)
PY
