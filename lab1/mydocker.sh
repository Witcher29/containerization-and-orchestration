#!/bin/bash
# mydocker.sh — свой Docker: namespaces + cgroup + capabilities + seccomp
set -e

API_PATH="/home/bulterier04/Desktop/itmo/containers/rep/containerization-and-orchestration/lab1/api"
CGROUP="/sys/fs/cgroup/lab-api"

# --- 1. Готовим cgroup с лимитами ---
sudo mkdir -p "$CGROUP"
echo "+memory +cpu +pids" | sudo tee /sys/fs/cgroup/cgroup.subtree_control > /dev/null
echo 64M             | sudo tee "$CGROUP/memory.max" > /dev/null
echo "50000 100000"  | sudo tee "$CGROUP/cpu.max"    > /dev/null
echo 10              | sudo tee "$CGROUP/pids.max"   > /dev/null

# --- 2. Запускаем api в изоляции ---
systemd-run --user --remain-after-exit\
  -p SystemCallFilter="~mkdir" \
  -p SystemCallErrorNumber=EPERM \
  unshare --pid --mount --net --uts --ipc --user --map-root-user --fork \
    --mount-proc \
    setpriv --inh-caps=-all --ambient-caps=-all --bounding-set=-all \
    sh -c "echo \$\$ | sudo tee $CGROUP/cgroup.procs > /dev/null; exec $API_PATH"

PID=$(pgrep -n -f 'lab1/api')

sudo nsenter -t "$PID" -n ip link set lo up