export const en = {
  brand: 'ipfs-panel',
  tagline: 'Kubo panel',
  languageLabel: 'Language',
  languageEn: 'EN',
  languageZh: '中文',
  navLabel: 'Sections',
  navNode: 'Node',
  nodeStopped: 'The node is stopped.',
  nodeStopForced:
    'The node is stopped. It did not exit on its own, so the panel forced it to stop.',
  nodeStarting: 'The node is starting.',
  nodeRunning: 'The node is running.',
  nodeStopping: 'The node is stopping.',
  nodeUnreachable: 'The panel cannot reach the manager on this machine.',
  nodeStart: 'Start',
  nodeStop: 'Stop',
  nodeRestart: 'Restart',
  nodeRetry: 'Retry',
  nodeVersion: 'Version',
  nodePeerId: 'Peer ID',
  nodeFolder: 'Node folder',
  nodeDetails: 'Details',
  nodeErrorBinary: 'Kubo is not installed, or the panel cannot find the `ipfs` command.',
  nodeErrorRepoWrite: 'The panel cannot write its node folder.',
  nodeErrorInit: 'The node folder could not be created.',
  nodeErrorApiPublic:
    'The node control port is not limited to this machine. The panel will not start it.',
  nodeErrorGatewayPublic:
    'The gateway is not limited to this machine. The panel will not start it.',
  nodeErrorPanelPort: "The node is set to use the panel's port. The panel will not start it.",
  nodeErrorPortInUse: 'A port the node needs is already in use.',
  nodeErrorRepoBusy:
    'Another Kubo is already using this folder. The panel will not stop that one.',
  nodeErrorDaemonExited: 'Kubo stopped on its own.',
  nodeErrorHealthTimeout:
    'The node process started, but it never became ready. The panel stopped it.',
  nodeErrorUnknown: 'Something went wrong starting or stopping the node.',
}

export type MessageSchema = typeof en
