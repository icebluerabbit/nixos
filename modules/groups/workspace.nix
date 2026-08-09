# workspace group — shared group for the /var/lib/workspace working tree.
# Because the code-server (and later opencode) containers run --private-users=no,
# this numeric gid means the same thing on the host and inside those containers, so
# the host user + the container service users can all co-write the tree once each
# carries it as a supplementary group.
_: {
  den.aspects.groups._.workspace = {
    aspect-name = "groups.workspace";
    nixos.users.groups.workspace.gid = 6000;
  };
}
