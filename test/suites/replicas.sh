test_replicas_setup() {
  local LXD_DIR

  echo "Creating clusters LXD_ONE and LXD_TWO..."

  LXD_ONE_DIR=$(mktemp -d -p "${TEST_DIR}" XXX)
  chmod +x "${LXD_ONE_DIR}"
  spawn_lxd "${LXD_ONE_DIR}" false

  LXD_TWO_DIR=$(mktemp -d -p "${TEST_DIR}" XXX)
  chmod +x "${LXD_TWO_DIR}"
  spawn_lxd "${LXD_TWO_DIR}" false

  # shellcheck disable=SC2034,SC2030
  LXD_DIR=${LXD_ONE_DIR}

  # Get the address of LXD_ONE.
  LXD_ONE_ADDR="$(lxc config get core.https_address)"

  # Enable clustering on LXD_ONE.
  lxc cluster enable node1
  [ "$(lxc cluster list | grep -cwF 'node1')" = 1 ]

  echo "Creating cluster link..."

  # Get a cluster link trust token from LXD_ONE.
  echo "Create pending cluster link on LXD_ONE"
  LXD_ONE_TRUST_TOKEN="$(lxc cluster link create lxd_two --quiet)"

  LXD_DIR=${LXD_TWO_DIR}

  # Get the address of LXD_TWO.
  LXD_TWO_ADDR="$(lxc config get core.https_address)"

  # Enable clustering on LXD_TWO.
  lxc cluster enable node2
  [ "$(lxc cluster list | grep -cwF 'node2')" = 1 ]

  echo "Create cluster link on LXD_TWO using the token from LXD_ONE"
  lxc cluster link create lxd_one --token "${LXD_ONE_TRUST_TOKEN}"

  # Create target project on LXD_TWO.
  lxc project create replica-project -c replica.mode=standby -c replica.source_cluster=lxd_one

  # Create source project on LXD_ONE.
  LXD_DIR=${LXD_ONE_DIR} lxc project create replica-project -c replica.mode=leader

  # Create replica on LXD_ONE.
  LXD_DIR=${LXD_ONE_DIR} lxc replica create my-replica --project replica-project target_cluster=lxd_two

  # Setting restricted.containers.nesting to 'allow' makes it possible to create nested containers.
  LXD_DIR=${LXD_ONE_DIR} lxc project set c1 restricted.containers.nesting=allow
  lxc init testimage c1 -c security.nesting=true --project replica-project

  # Run replica.
  LXD_DIR=${LXD_ONE_DIR} lxc replica run my-replica

  # Cleanup.
  kill_lxd "${LXD_ONE_DIR}"
  kill_lxd "${LXD_TWO_DIR}"
}
